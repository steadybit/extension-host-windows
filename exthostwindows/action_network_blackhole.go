// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

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

func NewNetworkBlackholeContainerAction() action_kit_sdk.Action[NetworkActionState] {
	return &networkAction{
		optsProvider: blackhole(),
		optsDecoder:  blackholeDecode,
		description:  getNetworkBlackholeDescription(),
	}
}

func getNetworkBlackholeDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.network_blackhole", BaseActionID),
		Label:       "Block Traffic",
		Description: "Block network traffic (incoming and outgoing).",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(blackHoleIcon),
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
			networkInterfaceParameter,
		),
	}
}

func blackhole() networkOptsProvider {
	return func(ctx context.Context, request action_kit_api.PrepareActionRequestBody) (network.WinOpts, action_kit_api.Messages, error) {
		_, err := CheckTargetHostname(request.Target.Attributes)
		if err != nil {
			return nil, nil, err
		}

		duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond
		if duration < time.Second {
			return nil, nil, errors.New("duration must be greater / equal than 1s")
		}

		filter, messages, err := mapToNetworkFilter(ctx, request.Config, getRestrictedEndpoints(request))
		if err != nil {
			return nil, nil, err
		}

		// Block incoming and outgoing traffic
		filter.Direction = network.DirectionAll

		return &network.BlackholeOpts{
			Filter:   filter,
			Duration: duration,
		}, messages, nil
	}
}

func blackholeDecode(data json.RawMessage) (network.WinOpts, error) {
	var opts network.BlackholeOpts
	err := json.Unmarshal(data, &opts)
	return &opts, err
}
