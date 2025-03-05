// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthost

import (
	"context"
	"encoding/json"
	"fmt"
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
		Parameters: append(
			commonNetworkParameters,
			action_kit_api.ActionParameter{
				Name:         "bandwidth",
				Label:        "Network Bandwidth",
				Description:  extutil.Ptr("How much traffic should be allowed per second?"),
				Type:         action_kit_api.Bitrate,
				DefaultValue: extutil.Ptr("1024kbit"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			action_kit_api.ActionParameter{
				Name:        "networkInterface",
				Label:       "Network Interface",
				Description: extutil.Ptr("Target Network Interface which should be affected. All if none specified."),
				Type:        action_kit_api.StringArray,
				Required:    extutil.Ptr(false),
				Order:       extutil.Ptr(104),
			},
		),
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

		filter, messages, err := mapToNetworkFilter(ctx, request.Config, getRestrictedEndpoints(request))
		if err != nil {
			return nil, nil, err
		}

		return &network.LimitBandwidthOpts{
			Filter:    filter,
			Bandwidth: bandwidth,
		}, messages, nil
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
