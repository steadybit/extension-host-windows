package exthostwindows

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type Mode string
type Unit string

const (
	ModeUsage    Mode = "usage"
	ModeAbsolute Mode = "absolute"
	UnitPercent  Unit = "%"
	UnitMegabyte Unit = "MiB"
)

func stringToMode(modeString string) (Mode, error) {
	if modeString == string(ModeAbsolute) {
		return ModeAbsolute, nil
	}

	if modeString == string(ModeUsage) {
		return ModeUsage, nil
	}

	return "", fmt.Errorf("mode must be one of the following: %s, %s", ModeAbsolute, ModeUsage)
}

func stringToUnit(stringUnit string) (Unit, error) {
	if stringUnit == string(UnitMegabyte) {
		return UnitMegabyte, nil
	}

	if stringUnit == string(UnitPercent) {
		return UnitPercent, nil
	}

	return "", fmt.Errorf("mode must be one of the following: %s, %s", UnitMegabyte, UnitPercent)
}

type fillMemAction struct {
	description  action_kit_api.ActionDescription
	optsProvider fillMemOptsProvider
}

type FillMemOpts struct {
	Duration time.Duration
	Mode     Mode
	Unit     Unit
	Size     uint
}

func (o *FillMemOpts) Args() []string {
	args := []string{fmt.Sprintf("%d%s", o.Size, o.Unit)}
	args = append(args, string(o.Mode))
	args = append(args, fmt.Sprintf("%ds", int(o.Duration.Seconds())))

	return args
}

type FillMemActionState struct {
	StressOpts  FillMemOpts
	ExecutionId uuid.UUID
}

var (
	_ action_kit_sdk.Action[FillMemActionState]           = (*fillMemAction)(nil)
	_ action_kit_sdk.ActionWithStatus[FillMemActionState] = (*fillMemAction)(nil)
	_ action_kit_sdk.ActionWithStop[FillMemActionState]   = (*fillMemAction)(nil) // Optional, needed when the action needs a stop method
)

type fillMemOptsProvider func(request action_kit_api.PrepareActionRequestBody) (*FillMemOpts, error)

func NewFillMemAction() action_kit_sdk.Action[FillMemActionState] {
	return &fillMemAction{
		description:  getFillMemDescription(),
		optsProvider: fillMem(),
	}
}

func (a *fillMemAction) NewEmptyState() FillMemActionState {
	return FillMemActionState{}
}

func fillMem() fillMemOptsProvider {
	return func(request action_kit_api.PrepareActionRequestBody) (*FillMemOpts, error) {
		duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond

		if duration < 1*time.Second {
			return nil, errors.New("duration must be greater / equal than 1s")
		}

		modeString := extutil.ToString(request.Config["mode"])

		mode, err := stringToMode(modeString)

		if err != nil {
			return nil, err
		}

		unitString := extutil.ToString(request.Config["unit"])

		unit, err := stringToUnit(unitString)

		if err != nil {
			return nil, err
		}

		size := extutil.ToUInt(request.Config["size"])

		return &FillMemOpts{
			Duration: duration,
			Size:     size,
			Mode:     mode,
			Unit:     unit,
		}, nil
	}
}

func getFillMemDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.fill_mem", BaseActionID),
		Label:       "Fill Memory",
		Description: "Fills the memory of the host for the given duration.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(fillMemoryIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			// The target type this action is for
			TargetType: targetID,
			// You can provide a list of target templates to help the user select targets.
			// A template can be used to pre-fill a selection
			SelectionTemplates: &targetSelectionTemplates,
		}),
		Technology: extutil.Ptr(WindowsHostTechnology),
		// Category for the targets to appear in
		Category: extutil.Ptr("Resource"),

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
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long should the memory be filled?"),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("30s"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			{
				Name:         "mode",
				Label:        "Mode",
				Description:  extutil.Ptr("*Fill and meet specified usage:* Fill up the memory until the desired usage is met. Memory allocation will be adjusted constantly to meet the target.\n\n*Fill the specified amount:* Allocate and hold the specified amount of Memory."),
				Type:         action_kit_api.String,
				DefaultValue: extutil.Ptr(string(ModeUsage)),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "Fill and meet specified usage",
						Value: string(ModeUsage),
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Fill the specified amount",
						Value: string(ModeAbsolute),
					},
				}),
			},
			{
				Name:         "size",
				Label:        "Size",
				Description:  extutil.Ptr("Percentage of total memory or Megabytes."),
				Type:         action_kit_api.Integer,
				DefaultValue: extutil.Ptr("80"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(3),
			},
			{
				Name:         "unit",
				Label:        "Unit",
				Description:  extutil.Ptr("Unit for the size parameter."),
				Type:         action_kit_api.String,
				DefaultValue: extutil.Ptr(string(UnitPercent)),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(4),
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "Megabytes",
						Value: string(UnitMegabyte),
					},
					action_kit_api.ExplicitParameterOption{
						Label: "% of total memory",
						Value: string(UnitPercent),
					},
				}),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

// Describe returns the action description for the platform with all required information.
func (a *fillMemAction) Describe() action_kit_api.ActionDescription {
	return a.description
}

// Prepare is called before the action is started.
// It can be used to validate the parameters and prepare the action.
// It must not cause any harmful effects.
// The passed in state is included in the subsequent calls to start/status/stop.
// So the state should contain all information needed to execute the action and even more important: to be able to stop it.
func (a *fillMemAction) Prepare(ctx context.Context, state *FillMemActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	if _, err := CheckTargetHostname(request.Target.Attributes); err != nil {
		return nil, err
	}

	if !isMemFillInstalled() {
		return nil, errors.New("memfill is not installed or cannot be found in %PATH%")
	}

	opts, err := a.optsProvider(request)
	if err != nil {
		return nil, err
	}

	state.StressOpts = *opts
	state.ExecutionId = request.ExecutionId
	return nil, nil
}

func (a *fillMemAction) Start(ctx context.Context, state *FillMemActionState) (*action_kit_api.StartResult, error) {
	command := exec.CommandContext(context.Background(), "memfill", state.StressOpts.Args()...)

	log.Info().Msgf("Running command: %s, %s.", command.Path, command.Args)

	go func() {
		output, err := command.CombinedOutput()

		if err != nil {
			log.Error().Msgf("Failed to start memfill attack: %s.", err)
		}

		log.Info().Msgf("%s", output)
	}()

	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{
			{
				Level:   extutil.Ptr(action_kit_api.Info),
				Message: fmt.Sprintf("Starting memfill with args: %s.", fmt.Sprintf("\"%s\"", strings.Join(state.StressOpts.Args(), " "))),
			},
		}),
	}, nil
}

func (a *fillMemAction) Status(_ context.Context, state *FillMemActionState) (*action_kit_api.StatusResult, error) {
	isRunning, err := isMemFillRunning()

	if err != nil {
		return &action_kit_api.StatusResult{
			Completed: true,
			Error: &action_kit_api.ActionKitError{
				Status: extutil.Ptr(action_kit_api.Failed),
				Title:  fmt.Sprintf("unable to retrieve 'memfill' process status: %s", err),
			},
		}, nil
	}

	if isRunning {
		return &action_kit_api.StatusResult{Completed: false}, nil
	}

	return &action_kit_api.StatusResult{
		Completed: true,
		Messages: &[]action_kit_api.Message{
			{
				Level:   extutil.Ptr(action_kit_api.Info),
				Message: "Memfill stopped",
			},
		},
	}, nil

}

func (a *fillMemAction) Stop(_ context.Context, state *FillMemActionState) (*action_kit_api.StopResult, error) {
	messages := make([]action_kit_api.Message, 0)
	isRunning, err := isMemFillRunning()

	if err != nil {
		return nil, err
	}

	if isRunning {
		cmd := exec.Command("powershell", "-Command", "Stop-Process", "-Name", "memfill", "-Force")
		out, err := cmd.CombinedOutput()

		if err != nil {
			if !strings.Contains(string(out), "Cannot find a process with the name") {
				return nil, err
			}
		}

		log.Info().Msgf("%s", out)
	}

	messages = append(messages, action_kit_api.Message{
		Level:   extutil.Ptr(action_kit_api.Info),
		Message: "Canceled memfill",
	})

	return &action_kit_api.StopResult{
		Messages: &messages,
	}, nil
}

func isMemFillInstalled() bool {
	cmd := exec.Command("memfill", "--help")
	cmd.Dir = os.TempDir()
	var outputBuffer bytes.Buffer
	cmd.Stdout = &outputBuffer
	cmd.Stderr = &outputBuffer
	err := cmd.Run()
	if err != nil {
		log.Error().Err(err).Msg("failed to start memfill")
		return false
	}
	success := cmd.ProcessState.Success()
	if !success {
		log.Error().Err(err).Msgf("memfill is not installed: 'memfill' in %v returned: %v", os.TempDir(), outputBuffer.Bytes())
	}
	return success
}

func isMemFillRunning() (bool, error) {
	cmd := exec.Command("powershell", "-Command", "Get-Process", "-Name", "memfill")
	output, err := cmd.Output()
	if err != nil {
		if !strings.Contains(string(output), "Cannot find a process with the name") {
			return false, nil
		}
		return false, err
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}
