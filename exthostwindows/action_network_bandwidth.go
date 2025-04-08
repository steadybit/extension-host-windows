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

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_commons/network"
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
		Description: "Limit available egress network bandwidth using QsS rules.",
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
				Type:         action_kit_api.Bitrate,
				DefaultValue: extutil.Ptr("1024kbit"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			{
				Name:         "hostname",
				Label:        "Hostname",
				Description:  extutil.Ptr("Restrict to which host the traffic is affected (Only host or IP allowed)."),
				Type:         action_kit_api.String,
				DefaultValue: extutil.Ptr(""),
				Advanced:     extutil.Ptr(true),
				Order:        extutil.Ptr(101),
			},
			{
				Name:         "ip",
				Label:        "IP Address/CIDR",
				Description:  extutil.Ptr("Restrict to which IP address or blocks the traffic is affected (Only host or IP allowed)."),
				Type:         action_kit_api.String,
				DefaultValue: extutil.Ptr(""),
				Advanced:     extutil.Ptr(true),
				Order:        extutil.Ptr(102),
			},
			{
				Name:         "port",
				Label:        "Port",
				Description:  extutil.Ptr("Restrict to which port the traffic is affected."),
				Type:         action_kit_api.String,
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

		bandwidth := extutil.ToString(request.Config["bandwidth"])
		bandwidth, err = sanitizeBandwidthAttribute(bandwidth)
		if err != nil {
			return nil, nil, err
		}

		includeCidrs, err := mapToNetworks(ctx, extutil.ToString(request.Config["host"]), extutil.ToString(request.Config["ip"]))
		if err != nil {
			return nil, nil, err
		}
		var includeCidr *net.IPNet
		if len(includeCidrs) > 0 {
			includeCidr = &includeCidrs[0]
		}

		port := extutil.ToInt(request.Config["port"])

		return &network.LimitBandwidthOpts{
			Bandwidth:   bandwidth,
			IncludeCidr: includeCidr,
			Port:        port,
		}, nil, nil
	}
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
