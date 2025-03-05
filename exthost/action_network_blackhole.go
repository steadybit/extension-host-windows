// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthost

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_commons/network"
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
		Description: "Blocks network traffic (incoming and outgoing).",
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
		Parameters:  commonNetworkParameters,
	}
}

func blackhole() networkOptsProvider {
	return func(ctx context.Context, request action_kit_api.PrepareActionRequestBody) (network.WinOpts, action_kit_api.Messages, error) {
		_, err := CheckTargetHostname(request.Target.Attributes)
		if err != nil {
			return nil, nil, err
		}

		var messages action_kit_api.Messages
		filter, netMessages, err := mapToNetworkFilter(ctx, request.Config, getRestrictedEndpoints(request))
		if err != nil {
			return nil, nil, err
		}
		messages = append(messages, netMessages...)

		return &network.BlackholeOpts{Filter: filter}, messages, nil
	}
}

func blackholeDecode(data json.RawMessage) (network.WinOpts, error) {
	var opts network.BlackholeOpts
	err := json.Unmarshal(data, &opts)
	return &opts, err
}
