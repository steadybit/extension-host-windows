// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/steadybit/extension-host-windows/exthostwindows/network"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"net"
	"strings"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-host-windows/config"
	extensionKit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
)

type networkOptsProvider func(ctx context.Context, request action_kit_api.PrepareActionRequestBody) (network.WinOpts, action_kit_api.Messages, error)

type networkOptsDecoder func(data json.RawMessage) (network.WinOpts, error)

type networkAction struct {
	description  action_kit_api.ActionDescription
	optsProvider networkOptsProvider
	optsDecoder  networkOptsDecoder
}

type NetworkActionState struct {
	NetworkOpts json.RawMessage
}

// Make sure networkAction implements all required interfaces
var _ action_kit_sdk.Action[NetworkActionState] = (*networkAction)(nil)
var _ action_kit_sdk.ActionWithStop[NetworkActionState] = (*networkAction)(nil)

var durationParamter = action_kit_api.ActionParameter{
	Name:         "duration",
	Label:        "Duration",
	Description:  extutil.Ptr("How long should the network be affected?"),
	Type:         action_kit_api.ActionParameterTypeDuration,
	DefaultValue: extutil.Ptr("30s"),
	Required:     extutil.Ptr(true),
	Order:        extutil.Ptr(0),
}

var networkInterfaceParameter = action_kit_api.ActionParameter{
	Name:        "networkInterface",
	Label:       "Network Interface",
	Description: extutil.Ptr("Target Network Interface which should be affected. All if none specified."),
	Type:        action_kit_api.ActionParameterTypeStringArray,
	Required:    extutil.Ptr(false),
	Order:       extutil.Ptr(104),
}

var commonNetworkParameters = []action_kit_api.ActionParameter{
	durationParamter,
	{
		Name:         "hostname",
		Label:        "Hostname",
		Description:  extutil.Ptr("Restrict to/from which hosts the traffic is affected."),
		Type:         action_kit_api.ActionParameterTypeStringArray,
		DefaultValue: extutil.Ptr(""),
		Advanced:     extutil.Ptr(true),
		Order:        extutil.Ptr(101),
	},
	{
		Name:         "ip",
		Label:        "IP Address/CIDR",
		Description:  extutil.Ptr("Restrict to/from which IP addresses or blocks the traffic is affected."),
		Type:         action_kit_api.ActionParameterTypeStringArray,
		DefaultValue: extutil.Ptr(""),
		Advanced:     extutil.Ptr(true),
		Order:        extutil.Ptr(102),
	},
	{
		Name:         "port",
		Label:        "Ports",
		Description:  extutil.Ptr("Restrict to/from which ports the traffic is affected."),
		Type:         action_kit_api.ActionParameterTypeStringArray,
		DefaultValue: extutil.Ptr(""),
		Advanced:     extutil.Ptr(true),
		Order:        extutil.Ptr(103),
	},
}

func (a *networkAction) NewEmptyState() NetworkActionState {
	return NetworkActionState{}
}

func (a *networkAction) Describe() action_kit_api.ActionDescription {
	return a.description
}

func (a *networkAction) Prepare(ctx context.Context, state *NetworkActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	_, err := CheckTargetHostname(request.Target.Attributes)
	if err != nil {
		return nil, err
	}

	opts, messages, err := a.optsProvider(ctx, request)
	if err != nil {
		return nil, extensionKit.WrapError(err)
	}

	if messages == nil {
		messages = []action_kit_api.Message{} // prevent empty messages response
	}

	rawOpts, err := json.Marshal(opts)
	if err != nil {
		return nil, extensionKit.ToError("Failed to serialize network settings.", err)
	}

	state.NetworkOpts = rawOpts

	return &action_kit_api.PrepareResult{Messages: &messages}, nil
}

func (a *networkAction) Start(ctx context.Context, state *NetworkActionState) (*action_kit_api.StartResult, error) {
	opts, err := a.optsDecoder(state.NetworkOpts)
	if err != nil {
		return nil, extensionKit.ToError("Failed to deserialize network settings.", err)
	}

	result := action_kit_api.StartResult{Messages: &action_kit_api.Messages{
		{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: opts.String(),
		},
	}}

	err = network.Apply(ctx, opts)
	if err != nil {
		return &result, extensionKit.ToError("Failed to apply network settings.", err)
	}

	return &result, nil
}

func (a *networkAction) Stop(ctx context.Context, state *NetworkActionState) (*action_kit_api.StopResult, error) {
	opts, err := a.optsDecoder(state.NetworkOpts)
	if err != nil {
		return nil, extensionKit.ToError("Failed to deserialize network settings.", err)
	}

	if err := network.Revert(ctx, opts); err != nil {
		return nil, extensionKit.ToError("Failed to revert network settings.", err)
	}

	return nil, nil
}

func mapToNetworkFilter(ctx context.Context, actionConfig map[string]interface{}, restrictedEndpoints []action_kit_api.RestrictedEndpoint) (network.Filter, action_kit_api.Messages, error) {
	ipsAndHosts := append(
		extutil.ToStringArray(actionConfig["ip"]),
		extutil.ToStringArray(actionConfig["hostname"])...,
	)
	includeCidrs, err := utils.MapToNetworks(ctx, ipsAndHosts...)
	if err != nil {
		return network.Filter{}, nil, err
	}

	//if no hostname/ip specified we affect all ips
	if len(includeCidrs) == 0 {
		includeCidrs = akn.NetAny
	}

	portRanges, err := utils.ParsePortRanges(extutil.ToStringArray(actionConfig["port"]))
	if err != nil {
		return network.Filter{}, nil, err
	}
	if len(portRanges) == 0 {
		//if no hostname/ip specified we affect all ports
		portRanges = []akn.PortRange{akn.PortRangeAny}
	}

	includes := akn.NewNetWithPortRanges(includeCidrs, portRanges...)
	for _, i := range includes {
		i.Comment = "parameters"
	}

	excludes, err := toExcludes(restrictedEndpoints)
	if err != nil {
		return network.Filter{}, nil, err
	}

	excludes = append(excludes, akn.ComputeExcludesForOwnIpAndPorts(config.Config.Port, config.Config.HealthPort)...)

	messages := []action_kit_api.Message{} // make sure messages is not nil
	excludes, condensed := condenseExcludes(excludes)
	if condensed {
		messages = append(messages, action_kit_api.Message{
			Level: extutil.Ptr(action_kit_api.Warn),
			Message: "Some excludes (to protect agent and extensions) were aggregated to reduce the number of commands necessary." +
				"This may lead to less specific exclude rules, some traffic might not be affected, as expected. " +
				"You can avoid this by configuring a more specific attack (e.g. by specifying ports or CIDRs).",
		})
	}

	interfaces := extutil.ToStringArray(actionConfig["networkInterface"])
	var interfaceIndexes []int
	if len(interfaces) != 0 {
		interfaceIndexes = akn.GetNetworkInterfaceIndexesByName(interfaces)
	}

	return network.Filter{
		Filter: akn.Filter{
			Include: includes,
			Exclude: excludes,
		},
		InterfaceIndexes: interfaceIndexes,
	}, messages, nil
}

func condenseExcludes(excludes []akn.NetWithPortRange) ([]akn.NetWithPortRange, bool) {
	l := len(excludes)
	excludes = utils.CondenseNetWithPortRange(excludes, 500)
	return excludes, l != len(excludes)
}

func toExcludes(restrictedEndpoints []action_kit_api.RestrictedEndpoint) ([]akn.NetWithPortRange, error) {
	var excludes []akn.NetWithPortRange

	for _, restrictedEndpoint := range restrictedEndpoints {
		_, cidr, err := net.ParseCIDR(restrictedEndpoint.Cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid cidr %s: %w", restrictedEndpoint.Cidr, err)
		}

		portRange := akn.PortRangeAny
		if restrictedEndpoint.PortMin != 0 || restrictedEndpoint.PortMax != 0 {
			portRange = akn.PortRange{From: uint16(restrictedEndpoint.PortMin), To: uint16(restrictedEndpoint.PortMax)}
		}

		nwps := akn.NewNetWithPortRanges([]net.IPNet{*cidr}, portRange)
		for i := range nwps {
			var sb strings.Builder
			if restrictedEndpoint.Name != "" {
				sb.WriteString(restrictedEndpoint.Name)
				sb.WriteString(" ")
			}
			if restrictedEndpoint.Url != "" {
				sb.WriteString(restrictedEndpoint.Url)
			}
			nwps[i].Comment = strings.TrimSpace(sb.String())
		}

		excludes = append(excludes, nwps...)
	}
	return excludes, nil
}
