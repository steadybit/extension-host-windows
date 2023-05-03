// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package exthost

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-container/pkg/networkutils"
	"github.com/steadybit/extension-host/exthost/common"
	"github.com/steadybit/extension-host/exthost/network"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
	"net/url"
	"strconv"
)

type networkOptsProvider func(ctx context.Context, request action_kit_api.PrepareActionRequestBody) (networkutils.Opts, error)

type networkOptsDecoder func(data json.RawMessage) (networkutils.Opts, error)

type networkAction struct {
	description  action_kit_api.ActionDescription
	optsProvider networkOptsProvider
	optsDecoder  networkOptsDecoder
}

type NetworkActionState struct {
	ExecutionId uuid.UUID
	NetworkOpts json.RawMessage
}

// Make sure networkAction implements all required interfaces
var _ action_kit_sdk.Action[NetworkActionState] = (*networkAction)(nil)
var _ action_kit_sdk.ActionWithStop[NetworkActionState] = (*networkAction)(nil)

var commonNetworkParameters = []action_kit_api.ActionParameter{
	{
		Name:         "duration",
		Label:        "Duration",
		Description:  extutil.Ptr("How long should the network be affected?"),
		Type:         action_kit_api.Duration,
		DefaultValue: extutil.Ptr("30s"),
		Required:     extutil.Ptr(true),
		Order:        extutil.Ptr(0),
	},
	{
		Name:         "hostname",
		Label:        "Hostname",
		Description:  extutil.Ptr("Restrict to/from which hosts the traffic is affected."),
		Type:         action_kit_api.StringArray,
		DefaultValue: extutil.Ptr(""),
		Advanced:     extutil.Ptr(true),
		Order:        extutil.Ptr(101),
	},
	{
		Name:         "ip",
		Label:        "IP Address",
		Description:  extutil.Ptr("Restrict to/from which IP addresses the traffic is affected."),
		Type:         action_kit_api.StringArray,
		DefaultValue: extutil.Ptr(""),
		Advanced:     extutil.Ptr(true),
		Order:        extutil.Ptr(102),
	},
	{
		Name:         "port",
		Label:        "Ports",
		Description:  extutil.Ptr("Restrict to/from which ports the traffic is affected."),
		Type:         action_kit_api.StringArray,
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

	opts, err := a.optsProvider(ctx, request)
	if err != nil {
		return nil, extension_kit.ToError("Failed to prepare network settings.", err)
	}

	rawOpts, err := json.Marshal(opts)
	if err != nil {
		return nil, extension_kit.ToError("Failed to serialize network settings.", err)
	}

	state.NetworkOpts = rawOpts
	return nil, nil
}

func (a *networkAction) Start(ctx context.Context, state *NetworkActionState) (*action_kit_api.StartResult, error) {
	opts, err := a.optsDecoder(state.NetworkOpts)
	if err != nil {
		return nil, extension_kit.ToError("Failed to deserialize network settings.", err)
	}

	hostname, err := osHostname()
	if err != nil {
		return nil, extension_kit.ToError("Failed to get hostname.", err)
	}
	err = network.Apply(ctx, hostname, opts)
	if err != nil {
		return nil, extension_kit.ToError("Failed to apply network settings.", err)
	}

	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{
			{
				Level:   extutil.Ptr(action_kit_api.Info),
				Message: opts.String(),
			},
		}),
	}, nil

}

func (a *networkAction) Stop(ctx context.Context, state *NetworkActionState) (*action_kit_api.StopResult, error) {
	opts, err := a.optsDecoder(state.NetworkOpts)
	if err != nil {
		return nil, extension_kit.ToError("Failed to deserialize network settings.", err)
	}

	hostname, err := osHostname()
	if err != nil {
		return nil, extension_kit.ToError("Failed to get hostname.", err)
	}
	err = network.Revert(ctx, hostname, opts)
	if err != nil {
		return nil, extension_kit.ToError("Failed to revert network settings.", err)
	}

	return nil, nil
}

func parsePortRanges(raw []string) ([]networkutils.PortRange, error) {
	if raw == nil {
		return nil, nil
	}

	var ranges []networkutils.PortRange

	for _, r := range raw {
		parsed, err := networkutils.ParsePortRange(r)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, parsed)
	}

	return ranges, nil
}

func mapToNetworkFilter(ctx context.Context, config map[string]interface{}, restrictedEndpoints []action_kit_api.RestrictedEndpoint) (networkutils.Filter, error) {
	toResolve := append(
		extutil.ToStringArray(config["ip"]),
		extutil.ToStringArray(config["hostname"])...,
	)
	includeCidrs, err := network.ResolveHostnames(ctx, toResolve...)
	if err != nil {
		return networkutils.Filter{}, err
	}
	if len(includeCidrs) == 0 {
		//if no hostname/ip specified we affect all ips
		includeCidrs = []string{"::/0", "0.0.0.0/0"}
	}

	portRanges, err := parsePortRanges(extutil.ToStringArray(config["port"]))
	if err != nil {
		return networkutils.Filter{}, err
	}
	if len(portRanges) == 0 {
		//if no hostname/ip specified we affect all ports
		portRanges = []networkutils.PortRange{networkutils.PortRangeAny}
	}

	includes := networkutils.NewCidrWithPortRanges(includeCidrs, portRanges...)
	var excludes []networkutils.CidrWithPortRange

	for _, restrictedEndpoint := range restrictedEndpoints {
		log.Debug().Msgf("Adding restricted endpoint %s (%S) => %s:%d-%d", restrictedEndpoint.Name, restrictedEndpoint.Url, restrictedEndpoint.Cidr, restrictedEndpoint.PortMin, restrictedEndpoint.PortMax)
		excludes = append(excludes, networkutils.NewCidrWithPortRanges([]string{restrictedEndpoint.Cidr}, networkutils.PortRange{From: uint16(restrictedEndpoint.PortMin), To: uint16(restrictedEndpoint.PortMax)})...)
	}

	// add own ip to exclude list
	ip4s, _ := common.GetIP4sAndNICs()
	ownPort := common.GetOwnPort()
	ownHealthPort := common.GetOwnHealthPort()
	for _, ip4 := range ip4s {
		cidrs, err := networkutils.IpRangeToCIDR(ip4, ip4)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to convert ip %s to cidr", ip4)
		}
		excludes = append(excludes, networkutils.NewCidrWithPortRanges(cidrs, networkutils.PortRange{From: ownPort, To: ownPort})...)
		excludes = append(excludes, networkutils.NewCidrWithPortRanges(cidrs, networkutils.PortRange{From: ownHealthPort, To: ownHealthPort})...)
	}

	return networkutils.Filter{
		Include: includes,
		Exclude: excludes,
	}, nil
}

func readNetworkInterfaces(ctx context.Context) ([]string, error) {
	ifcs, err := network.ListInterfaces(ctx)
	if err != nil {
		return nil, err
	}

	var ifcNames []string
	for _, ifc := range ifcs {
		if ifc.HasFlag("UP") && !ifc.HasFlag("LOOPBACK") {
			ifcNames = append(ifcNames, ifc.Name)
		}
	}
	return ifcNames, nil
}

func resolveUrl(ctx context.Context, raw string) ([]string, uint16, error) {
	port := uint16(0)
	u, err := url.Parse(raw)
	if err != nil {
		return nil, port, err
	}
	resolvedIps := []string{}
	if u.Hostname() == "localhost" {
		resolvedIps = append(resolvedIps, "127.0.0.1")
	} else {
		resolvedIps, err = network.ResolveHostnames(ctx, u.Hostname())
		if err != nil {
			return nil, port, err
		}
	}

	ips := make([]string, 0)
	for _, ip := range resolvedIps {
		if ip == "" {
			continue
		}
		cidr, err := networkutils.IpRangeToCIDR(ip, ip)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to convert ip range to cidr: %s", ip)
			continue
		}
		log.Debug().Msgf("Converted ip %s to cidr: %s", ip, cidr[0])
		ips = append(ips, cidr[0])
	}

	portStr := u.Port()
	if portStr != "" {
		parsed, _ := strconv.ParseUint(portStr, 10, 16)
		port = uint16(parsed)
	} else {
		switch u.Scheme {
		case "https":
			port = 443
		default:
			port = 80
		}
	}

	return ips, port, nil
}
