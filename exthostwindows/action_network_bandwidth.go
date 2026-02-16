// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"context"
	"encoding/json"
	"fmt"
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
		Icon:        extutil.Ptr(bandwidthIcon),
		TargetSelection: &action_kit_api.TargetSelection{
			TargetType:         targetID,
			SelectionTemplates: &targetSelectionTemplates,
		},
		Technology:  extutil.Ptr(WindowsHostTechnology),
		Category:    extutil.Ptr("Network"),
		Kind:        action_kit_api.Attack,
		TimeControl: action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			durationParamter,
			{
				Name:         "bandwidth",
				Label:        "Network Bandwidth",
				Description:  extutil.Ptr("How much traffic should be allowed per second?"),
				Type:         action_kit_api.ActionParameterTypeBitrate,
				DefaultValue: extutil.Ptr("1024kbit"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			{
				Name:  "filter",
				Label: "Traffic Filter",
				Type:  action_kit_api.ActionParameterTypeHeader,
				Order: extutil.Ptr(100),
				Hint: &action_kit_api.ActionHint{
					Content: "Either the hostname or IP parameter is required.",
					Type:    action_kit_api.HintInfo,
				},
			},
			{
				Name:         "hostname",
				Label:        "Hostnames",
				Description:  extutil.Ptr("Restrict to/from which hosts the traffic is affected."),
				Type:         action_kit_api.ActionParameterTypeStringArray,
				DefaultValue: extutil.Ptr(""),
				Order:        extutil.Ptr(101),
			},
			{
				Name:         "ip",
				Label:        "IPs/CIDRs",
				Description:  extutil.Ptr("Restrict to/from which IP addresses or blocks the traffic is affected."),
				Type:         action_kit_api.ActionParameterTypeStringArray,
				DefaultValue: extutil.Ptr(""),
				Order:        extutil.Ptr(102),
			},
			{
				Name:         "port",
				Label:        "Ports",
				Description:  extutil.Ptr("Restrict to/from which port the traffic is affected."),
				Type:         action_kit_api.ActionParameterTypeString,
				DefaultValue: extutil.Ptr(""),
				Advanced:     extutil.Ptr(true),
				Order:        extutil.Ptr(103),
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

		bandwidth := extutil.ToString(request.Config["bandwidth"])
		bandwidth, err = sanitizeBandwidthAttribute(bandwidth)
		if err != nil {
			return nil, nil, err
		}

		ipsAndHosts := append(
			extutil.ToStringArray(request.Config["ip"]),
			extutil.ToStringArray(request.Config["hostname"])...,
		)
		if len(ipsAndHosts) == 0 {
			return nil, nil, fmt.Errorf("hostname or IP required")
		}

		includeCidrs, err := utils.MapToNetworks(ctx, ipsAndHosts...)
		if err != nil {
			return nil, nil, err
		}

		portRange := akn.PortRangeAny
		if portRangeParameter := extutil.ToString(request.Config["port"]); portRangeParameter != "" {
			portRange, err = akn.ParsePortRange(portRangeParameter)
			if err != nil {
				return nil, nil, err
			}
		}

		err = validateRestrictedEndpoints(request, includeCidrs, portRange)
		if err != nil {
			return nil, nil, err
		}

		return &network.LimitBandwidthOpts{
			Bandwidth:    bandwidth,
			IncludeCidrs: includeCidrs,
			PortRange:    portRange,
		}, nil, nil
	}
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

func sanitizeBandwidthAttribute(bandwidth string) (string, error) {
	suffixArray := map[string]string{"tbps": "TB", "gbps": "GB", "mbps": "MB", "kbps": "KB", "bps": "", "tbit": "TB", "gbit": "GB", "mbit": "MB", "kbit": "KB", "bit": ""}
	orderedKeys := []string{"tbps", "gbps", "mbps", "kbps", "bps", "tbit", "gbit", "mbit", "kbit", "bit"}

	for _, key := range orderedKeys {
		if strings.Contains(bandwidth, key) {
			numericStr := strings.Replace(bandwidth, key, "", 1)
			numeric, err := strconv.ParseUint(numericStr, 10, 64)
			if err != nil {
				return "", err
			}

			if strings.Contains(key, "bit") {
				return fmt.Sprintf("%d%s", numeric, suffixArray[key]), nil

			} else if strings.Contains(key, "bps") {
				numeric = 8 * numeric
				return fmt.Sprintf("%d%s", numeric, suffixArray[key]), nil
			} else {
				return "", fmt.Errorf("invalid network bandwidth")
			}
		}
	}

	return "", fmt.Errorf("invalid network bandwidth")
}

func limitBandwidthDecode(data json.RawMessage) (network.WinOpts, error) {
	var opts network.LimitBandwidthOpts
	err := json.Unmarshal(data, &opts)
	return &opts, err
}
