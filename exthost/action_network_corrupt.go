// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

func NewNetworkCorruptPackagesContainerAction() action_kit_sdk.Action[NetworkActionState] {
	return &networkAction{
		optsProvider: corruptPackages(),
		optsDecoder:  corruptPackagesDecode,
		description:  getNetworkCorruptPackagesDescription(),
	}
}

func getNetworkCorruptPackagesDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.network_package_corruption", BaseActionID),
		Label:       "Corrupt Outgoing Packages",
		Description: "Inject corrupt packets by introducing single bit error at a random offset into egress network traffic.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(corruptIcon),
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
				Name:         "networkCorruption",
				Label:        "Package Corruption",
				Description:  extutil.Ptr("How much of the traffic should be corrupted?"),
				Type:         action_kit_api.Percentage,
				DefaultValue: extutil.Ptr("15"),
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

func corruptPackages() networkOptsProvider {
	return func(ctx context.Context, request action_kit_api.PrepareActionRequestBody) (network.WinOpts, action_kit_api.Messages, error) {
		_, err := CheckTargetHostname(request.Target.Attributes)
		if err != nil {
			return nil, nil, err
		}
		corruption := extutil.ToUInt(request.Config["networkCorruption"])
		duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond

		if duration < time.Second {
			return nil, nil, errors.New("duration must be greater / equal than 1s")
		}

		filter, messages, err := mapToNetworkFilter(ctx, request.Config, getRestrictedEndpoints(request))
		if err != nil {
			return nil, nil, err
		}

		return &network.CorruptPackagesOpts{
			Filter:     filter,
			Corruption: corruption,
			Duration:   duration,
		}, messages, nil
	}
}

func corruptPackagesDecode(data json.RawMessage) (network.WinOpts, error) {
	var opts network.CorruptPackagesOpts
	err := json.Unmarshal(data, &opts)
	return &opts, err
}
