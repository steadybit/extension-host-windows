package exthostwindows

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type timeTravelAction struct {
}

type TimeTravelActionState struct {
	DisableNtp    bool
	Offset        time.Duration
	OffsetApplied bool
}

var (
	_ action_kit_sdk.Action[TimeTravelActionState]         = (*timeTravelAction)(nil)
	_ action_kit_sdk.ActionWithStop[TimeTravelActionState] = (*timeTravelAction)(nil) // Optional, needed when the action needs a stop method
)

func NewTimetravelAction() action_kit_sdk.Action[TimeTravelActionState] {
	return &timeTravelAction{}
}

func (a *timeTravelAction) NewEmptyState() TimeTravelActionState {
	return TimeTravelActionState{}
}

// Describe returns the action description for the platform with all required information.
func (a *timeTravelAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.timetravel", BaseActionID),
		Label:       "Time Travel",
		Description: "Change the system time by the given offset.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        new(timeTravelIcon),
		TargetSelection: new(action_kit_api.TargetSelection{
			// The target type this action is for
			TargetType: targetID,
			// You can provide a list of target templates to help the user select targets.
			// A template can be used to pre-fill a selection
			SelectionTemplates: &targetSelectionTemplates,
		}),
		Technology: new(WindowsHostTechnology),
		// Category for the targets to appear in
		Category: new("State"),

		// To clarify the purpose of the action, you can set a kind.
		//   Attack: Will cause harm to targets
		//   Check: Will perform checks on the targets
		//   LoadTest: Will perform load tests on the targets
		//   Other
		Kind: action_kit_api.Attack,

		// How the action is controlled over time.
		//   External: The agent takes care and calls stop then the time has passed. Requires a duration parameter. Use this when the duration is known in advance.
		//   Internal: The action has to implement the status endpoint to signal when the action is done. Use this when the duration is not known in advance.
		//   Instantaneous: The action is done immediately. Use this for actions that happen immediately, e.g. a reboot.
		TimeControl: action_kit_api.TimeControlExternal,

		// The parameters for the action
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:          "offset",
				Label:         "Offset",
				Description:   new("The offset to the current time."),
				Type:          action_kit_api.Duration,
				DurationUnits: new([]action_kit_api.DurationUnit{action_kit_api.DurationUnitMilliseconds, action_kit_api.DurationUnitSeconds, action_kit_api.DurationUnitMinutes, action_kit_api.DurationUnitHours, action_kit_api.DurationUnitDays}),
				DefaultValue:  new("60m"),
				Required:      new(true),
				Order:         new(1),
			},
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  new("How long should time travel take?"),
				Type:         action_kit_api.Duration,
				DefaultValue: new("30s"),
				Required:     new(true),
				Order:        new(2),
			}, {
				Name:         "disableNtp",
				Label:        "Disable NTP",
				Description:  new("Prevent NTP from correcting time during attack."),
				Type:         action_kit_api.Boolean,
				DefaultValue: new("true"),
				Required:     new(false),
				Advanced:     new(true),
				Order:        new(1),
			},
		},
		Stop:            new(action_kit_api.MutatingEndpointReference{}),
		AdditionalFlags: new([]action_kit_api.ActionDescriptionAdditionalFlags{action_kit_api.DISABLEHEARTBEAT}),
	}
}

// Prepare is called before the action is started.
// It can be used to validate the parameters and prepare the action.
// It must not cause any harmful effects.
// The passed in state is included in the subsequent calls to start/status/stop.
// So the state should contain all information needed to execute the action and even more important: to be able to stop it.
func (a *timeTravelAction) Prepare(_ context.Context, state *TimeTravelActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	_, err := CheckTargetHostname(request.Target.Attributes)
	if err != nil {
		return nil, err
	}

	state.Offset = time.Duration(extutil.ToUInt64(request.Config["offset"])) * time.Millisecond
	if state.Offset < 1*time.Second {
		return &action_kit_api.PrepareResult{
			Error: new(action_kit_api.ActionKitError{
				Title:  "Duration must be greater / equal than 1s",
				Status: extutil.Ptr(action_kit_api.Errored),
			}),
		}, nil
	}
	state.DisableNtp = extutil.ToBool(request.Config["disableNtp"])

	return nil, nil
}

// Start is called to start the action
// You can mutate the state here.
// You can use the result to return messages/errors/metrics or artifacts
func (a *timeTravelAction) Start(ctx context.Context, state *TimeTravelActionState) (*action_kit_api.StartResult, error) {
	if state.DisableNtp {
		log.Info().Msg("Blocking NTP traffic")
		command := utils.PowershellCommand("Stop-Service", "w32time")
		output, err := command.CombinedOutput()

		if err != nil {
			log.Error().Msg("Failed to block NTP traffic.")
			return nil, err
		}

		log.Info().Msgf("%s", output)
	}

	log.Info().Dur("offset", state.Offset).Msg("Adjusting time")

	command := utils.PowershellCommand(fmt.Sprintf("Set-Date -Date (Get-Date).AddMinutes(%f)", state.Offset.Minutes()))
	output, err := command.CombinedOutput()
	if err != nil {
		log.Error().Msg("Failed to adjust time.")
		return nil, err
	}

	log.Info().Msgf("%s", output)

	state.OffsetApplied = true
	return nil, nil
}

// Stop is called to stop the action
// It will be called even if the start method did not complete successfully.
// It should be implemented in a immutable way, as the agent might to retries if the stop method timeouts.
// You can use the result to return messages/errors/metrics or artifacts
func (a *timeTravelAction) Stop(ctx context.Context, state *TimeTravelActionState) (*action_kit_api.StopResult, error) {
	if !state.OffsetApplied {
		log.Info().Msg("No offset applied, skipping revert.")
		return nil, nil
	}

	log.Info().Msg("Adjusting time back.")
	if state.DisableNtp {
		log.Info().Msg("Unblocking NTP traffic.")
		command := utils.PowershellCommand("Start-Service", "w32time")

		output, err := command.CombinedOutput()

		if err != nil {
			log.Error().Msg("Failed to unblock NTP traffic.")
			return nil, err
		}

		log.Info().Msgf("%s", output)
	}

	command := utils.PowershellCommand(fmt.Sprintf("Set-Date -Date (Get-Date).AddMinutes(-%f)", state.Offset.Minutes()))
	output, err := command.CombinedOutput()
	if err != nil {
		log.Error().Msg("Failed to revert time adjustment.")
		return nil, err
	}

	log.Info().Msgf("%s", output)

	command = utils.PowershellCommand("w32tm", "/resync")
	output, err = command.CombinedOutput()
	if err != nil {
		log.Error().Msg("Failed to resync time using NTP.")
		return nil, err
	}

	log.Info().Msgf("%s", output)

	state.OffsetApplied = false
	return nil, nil
}
