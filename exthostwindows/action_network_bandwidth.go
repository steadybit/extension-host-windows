// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"

	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/steadybit/extension-host-windows/exthostwindows/network"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

func NewNetworkLimitBandwidthContainerAction() action_kit_sdk.Action[NetworkActionState] {
	return &networkAction{
		optsProvider: limitBandwidth(),
		optsDecoder:  limitBandwidthDecode,
		description:  getNetworkLimitBandwidthDescription(),
	}
}

func getNetworkLimitBandwidthDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.network_bandwidth", BaseActionID),
		Label:       "Limit Outgoing Bandwidth",
		Description: "Limit available egress network bandwidth.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        new(bandwidthIcon),
		TargetSelection: &action_kit_api.TargetSelection{
			TargetType:         targetID,
			SelectionTemplates: &targetSelectionTemplates,
		},
		Technology:  new(WindowsHostTechnology),
		Category:    new("Network"),
		Kind:        action_kit_api.Attack,
		TimeControl: action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			durationParamter,
			{
				Name:         "bandwidth",
				Label:        "Network Bandwidth",
				Description:  new("How much traffic should be allowed per second?"),
				Type:         action_kit_api.ActionParameterTypeBitrate,
				DefaultValue: new("1024kbit"),
				Required:     new(true),
				Order:        new(1),
			},
			{
				Name:  "filter",
				Label: "Traffic Filter",
				Type:  action_kit_api.ActionParameterTypeHeader,
				Order: new(100),
				Hint: &action_kit_api.ActionHint{
					Content: "Either the hostname or IP parameter is required.",
					Type:    action_kit_api.HintInfo,
				},
			},
			{
				Name:         "hostname",
				Label:        "Hostnames",
				Description:  new("Restrict to/from which hosts the traffic is affected."),
				Type:         action_kit_api.ActionParameterTypeStringArray,
				DefaultValue: new(""),
				Order:        new(101),
			},
			{
				Name:         "ip",
				Label:        "IPs/CIDRs",
				Description:  new("Restrict to/from which IP addresses or blocks the traffic is affected."),
				Type:         action_kit_api.ActionParameterTypeStringArray,
				DefaultValue: new(""),
				Order:        new(102),
			},
			{
				Name:         "port",
				Label:        "Ports",
				Description:  new("Restrict to/from which port the traffic is affected."),
				Type:         action_kit_api.ActionParameterTypeString,
				DefaultValue: new(""),
				Order:        new(103),
			},
		},
	}
}

func limitBandwidth() networkOptsProvider {
	return func(ctx context.Context, request action_kit_api.PrepareActionRequestBody) (network.WinOpts, action_kit_api.Messages, error) {
		_, err := CheckTargetHostname(request.Target.Attributes)
		if err != nil {
			return nil, nil, err
		}

		parsedDuration := extutil.ToUInt64(request.Config["duration"])
		if parsedDuration == 0 {
			return nil, nil, fmt.Errorf("duration is required")
		}

		bandwidth, bitsPerSecond, err := parseBandwidth(extutil.ToString(request.Config["bandwidth"]))
		if err != nil {
			return nil, nil, err
		}

		includeCidrs, portRange, err := bandwidthTargets(ctx, request.Config)
		if err != nil {
			return nil, nil, err
		}

		err = validateRestrictedEndpoints(request, includeCidrs, portRange)
		if err != nil {
			return nil, nil, err
		}

		return &network.LimitBandwidthOpts{
			Bandwidth:     bandwidth,
			BitsPerSecond: bitsPerSecond,
			IncludeCidrs:  includeCidrs,
			PortRange:     portRange,
		}, nil, nil
	}
}

func bandwidthTargets(ctx context.Context, config map[string]any) ([]net.IPNet, akn.PortRange, error) {
	ipsAndHosts := append(
		extutil.ToStringArray(config["ip"]),
		extutil.ToStringArray(config["hostname"])...,
	)
	if len(ipsAndHosts) == 0 {
		return nil, akn.PortRange{}, fmt.Errorf("hostname or IP required")
	}

	includeCidrs, err := utils.MapToNetworks(ctx, ipsAndHosts...)
	if err != nil {
		return nil, akn.PortRange{}, err
	}

	portRange := akn.PortRangeAny
	if portRangeParameter := extutil.ToString(config["port"]); portRangeParameter != "" {
		portRange, err = akn.ParsePortRange(portRangeParameter)
		if err != nil {
			return nil, akn.PortRange{}, err
		}
	}
	return includeCidrs, portRange, nil
}

func validateRestrictedEndpoints(request action_kit_api.PrepareActionRequestBody, includeCidrs []net.IPNet, portRange akn.PortRange) error {
	restrictedEndpoints, err := toExcludes(getRestrictedEndpoints(request))
	if err != nil {
		return fmt.Errorf("failed to validate restricted endpoints: %w", err)
	}

	targets := akn.NewNetWithPortRanges(includeCidrs, portRange)
	for _, target := range targets {
		for _, restricted := range restrictedEndpoints {
			if target.Overlap(restricted) {
				return fmt.Errorf("target %s overlaps with restricted endpoint %s", target.String(), restricted.String())
			}
		}
	}

	return nil
}

// bandwidthUnits are the units of a bitrate parameter, the byte based ones before the bit based ones so that "mbps"
// is not taken for "bps". nameUnit keeps the policy names of earlier versions, so that a running attack can still be
// reverted after an update.
var bandwidthUnits = []struct {
	suffix   string
	bytes    bool
	scale    uint64
	nameUnit string
}{
	{"tbps", true, 1_000_000_000_000, "TB"}, {"gbps", true, 1_000_000_000, "GB"}, {"mbps", true, 1_000_000, "MB"}, {"kbps", true, 1_000, "KB"}, {"bps", true, 1, ""},
	{"tbit", false, 1_000_000_000_000, "TB"}, {"gbit", false, 1_000_000_000, "GB"}, {"mbit", false, 1_000_000, "MB"}, {"kbit", false, 1_000, "KB"}, {"bit", false, 1, ""},
}

// parseBandwidth converts a bitrate parameter like "1024kbit" into bits per second and the name used in the QoS
// policy names ("1024KB").
func parseBandwidth(bandwidth string) (string, uint64, error) {
	for _, unit := range bandwidthUnits {
		if !strings.Contains(bandwidth, unit.suffix) {
			continue
		}
		value, err := strconv.ParseUint(strings.Replace(bandwidth, unit.suffix, "", 1), 10, 64)
		if err != nil {
			return "", 0, err
		}
		if unit.bytes {
			if value > math.MaxUint64/8 {
				return "", 0, fmt.Errorf("invalid network bandwidth %s: too large", bandwidth)
			}
			value *= 8
		}
		if value > math.MaxUint64/unit.scale {
			return "", 0, fmt.Errorf("invalid network bandwidth %s: too large", bandwidth)
		}
		if value*unit.scale < 8 {
			return "", 0, fmt.Errorf("windows qos policy does not support rate settings below 8bit/s. (%s)", bandwidth)
		}
		return fmt.Sprintf("%d%s", value, unit.nameUnit), value * unit.scale, nil
	}
	return "", 0, fmt.Errorf("invalid network bandwidth")
}

func limitBandwidthDecode(data json.RawMessage) (network.WinOpts, error) {
	var opts network.LimitBandwidthOpts
	err := json.Unmarshal(data, &opts)
	return &opts, err
}
