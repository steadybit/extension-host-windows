// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-host-windows/exthostwindows/shutdown"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type shutdownAction struct {
	command shutdown.Command
}

type ActionState struct {
	Reboot bool
}

var (
	_ action_kit_sdk.Action[ActionState] = (*shutdownAction)(nil)
)

func NewShutdownAction() action_kit_sdk.Action[ActionState] {
	return &shutdownAction{
		command: shutdown.NewCommand(),
	}
}

func (l *shutdownAction) NewEmptyState() ActionState {
	return ActionState{}
}

func (l *shutdownAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.shutdown", BaseActionID),
		Label:       "Shutdown Host",
		Description: "Reboot or shut down the host.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(shutdownIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType:         targetID,
			SelectionTemplates: &targetSelectionTemplates,
		}),
		Technology:  extutil.Ptr(WindowsHostTechnology),
		Category:    extutil.Ptr("State"),
		Kind:        action_kit_api.Attack,
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "reboot",
				Label:        "Reboot",
				Description:  extutil.Ptr("Should the host reboot after shutting down?"),
				Type:         action_kit_api.Boolean,
				DefaultValue: extutil.Ptr("true"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
		},
	}
}

func (l *shutdownAction) Prepare(_ context.Context, state *ActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	_, err := CheckTargetHostname(request.Target.Attributes)
	if err != nil {
		return nil, err
	}
	reboot := extutil.ToBool(request.Config["reboot"])
	state.Reboot = reboot

	if !l.command.IsShutdownCommandExecutable() {
		return &action_kit_api.PrepareResult{
			Error: &action_kit_api.ActionKitError{
				Title:  "Shutdown command not found",
				Status: extutil.Ptr(action_kit_api.Errored),
			},
		}, nil
	}

	return nil, nil
}

func (l *shutdownAction) Start(_ context.Context, state *ActionState) (*action_kit_api.StartResult, error) {
	if state.Reboot {
		log.Info().Msg("Rebooting host via command")
		err := l.command.Reboot()
		if err != nil {
			log.Err(err).Msg("Rebooting host via command failed")
			return &action_kit_api.StartResult{
				Error: &action_kit_api.ActionKitError{
					Title:  "Reboot failed",
					Status: extutil.Ptr(action_kit_api.Failed),
					Detail: extutil.Ptr(err.Error()),
				},
			}, nil
		}
	} else {
		log.Info().Msg("Shutting down host via command")
		err := l.command.Shutdown()
		if err != nil {
			log.Err(err).Msg("Shutting down host via command failed")
			return &action_kit_api.StartResult{
				Error: &action_kit_api.ActionKitError{
					Title:  "Shutdown failed",
					Status: extutil.Ptr(action_kit_api.Failed),
					Detail: extutil.Ptr(err.Error()),
				},
			}, nil
		}
	}
	return nil, nil
}
