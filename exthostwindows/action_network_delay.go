// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/steadybit/extension-host-windows/exthostwindows/network"
	"time"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

func NewNetworkDelayContainerAction() action_kit_sdk.Action[NetworkActionState] {
	return &networkAction{
		optsProvider: delay(),
		optsDecoder:  delayDecode,
		description:  getNetworkDelayDescription(),
	}
}

func getNetworkDelayDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.network_delay", BaseActionID),
		Label:       "Delay Outgoing Traffic",
		Description: "Inject latency into egress network traffic using WinDivert.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(delayIcon),
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
				Name:         "networkDelay",
				Label:        "Network Delay",
				Description:  extutil.Ptr("How much should the traffic be delayed?"),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("500ms"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			action_kit_api.ActionParameter{
				Name:         "networkDelayJitter",
				Label:        "Jitter",
				Description:  extutil.Ptr("Add random +/-30% jitter to network delay?"),
				Type:         action_kit_api.Boolean,
				DefaultValue: extutil.Ptr("false"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
			networkInterfaceParameter,
		),
	}
}

func delay() networkOptsProvider {
	return func(ctx context.Context, request action_kit_api.PrepareActionRequestBody) (network.WinOpts, action_kit_api.Messages, error) {
		_, err := CheckTargetHostname(request.Target.Attributes)
		if err != nil {
			return nil, nil, err
		}

		delay := time.Duration(extutil.ToInt64(request.Config["networkDelay"])) * time.Millisecond
		hasJitter := extutil.ToBool(request.Config["networkDelayJitter"])

		duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond
		if duration < time.Second {
			return nil, nil, errors.New("duration must be greater / equal than 1s")
		}

		filter, messages, err := mapToNetworkFilter(ctx, request.Config, getRestrictedEndpoints(request))
		if err != nil {
			return nil, nil, err
		}

		return &network.DelayOpts{
			Filter:   filter,
			Delay:    delay,
			Jitter:   hasJitter,
			Duration: duration,
		}, messages, nil
	}
}

func delayDecode(data json.RawMessage) (network.WinOpts, error) {
	var opts network.DelayOpts
	err := json.Unmarshal(data, &opts)
	return &opts, err
}
