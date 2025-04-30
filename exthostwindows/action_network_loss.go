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

func NewNetworkPackageLossContainerAction() action_kit_sdk.Action[NetworkActionState] {
	return &networkAction{
		optsProvider: packageLoss(),
		optsDecoder:  packageLossDecode,
		description:  getNetworkPackageLossDescription(),
	}
}

func getNetworkPackageLossDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.network_package_loss", BaseActionID),
		Label:       "Drop Outgoing Traffic",
		Description: "Cause packet loss for outgoing network traffic (egress).",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(lossIcon),
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
				Name:         "percentage",
				Label:        "Network Loss",
				Description:  extutil.Ptr("How much of the traffic should be lost?"),
				Type:         action_kit_api.ActionParameterTypePercentage,
				DefaultValue: extutil.Ptr("70"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			networkInterfaceParameter,
		),
	}
}

func packageLoss() networkOptsProvider {
	return func(ctx context.Context, request action_kit_api.PrepareActionRequestBody) (network.WinOpts, action_kit_api.Messages, error) {
		_, err := CheckTargetHostname(request.Target.Attributes)
		if err != nil {
			return nil, nil, err
		}
		loss := extutil.ToUInt(request.Config["percentage"])

		duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond
		if duration < time.Second {
			return nil, nil, errors.New("duration must be greater / equal than 1s")
		}

		filter, messages, err := mapToNetworkFilter(ctx, request.Config, getRestrictedEndpoints(request))
		if err != nil {
			return nil, nil, err
		}

		return &network.PackageLossOpts{
			Filter:   filter,
			Loss:     loss,
			Duration: duration,
		}, messages, nil
	}
}

func packageLossDecode(data json.RawMessage) (network.WinOpts, error) {
	var opts network.PackageLossOpts
	err := json.Unmarshal(data, &opts)
	return &opts, err
}
