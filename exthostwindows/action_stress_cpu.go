package exthostwindows

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

var validProcessPriorities [4]string = [4]string{"Normal", "AboveNormal", "High", "RealTime"}

const steadybitStressCpuExecutableName = "steadybit-stress-cpu"

type cpuStressAction struct {
	description  action_kit_api.ActionDescription
	optsProvider stressOptsProvider
}

type CpuStressOpts struct {
	Cores    int
	CpuLoad  int
	Priority string
	Duration time.Duration
}

func (o *CpuStressOpts) Args() []string {
	args := []string{"--duration", strconv.Itoa(int(o.Duration.Seconds()))}
	args = append(args, "--cores", strconv.Itoa(int(o.Cores)))
	args = append(args, "--priority", o.Priority)
	args = append(args, "--percentage", strconv.Itoa(o.CpuLoad))

	return args
}

type StressActionState struct {
	StressOpts  CpuStressOpts
	ExecutionId uuid.UUID
}

var (
	_ action_kit_sdk.Action[StressActionState]           = (*cpuStressAction)(nil)
	_ action_kit_sdk.ActionWithStatus[StressActionState] = (*cpuStressAction)(nil)
	_ action_kit_sdk.ActionWithStop[StressActionState]   = (*cpuStressAction)(nil) // Optional, needed when the action needs a stop method
)

type stressOptsProvider func(request action_kit_api.PrepareActionRequestBody) (*CpuStressOpts, error)

func NewStressCpuAction() action_kit_sdk.Action[StressActionState] {
	return &cpuStressAction{
		description:  getStressCpuDescription(),
		optsProvider: stressCpu(),
	}
}

func (a *cpuStressAction) NewEmptyState() StressActionState {
	return StressActionState{}
}

func stressCpu() stressOptsProvider {
	return func(request action_kit_api.PrepareActionRequestBody) (*CpuStressOpts, error) {
		duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond

		if duration < 1*time.Second {
			return nil, errors.New("duration must be greater / equal than 1s")
		}

		cores := extutil.ToInt(request.Config["cores"])
		availableCores := runtime.NumCPU()

		if cores > availableCores {
			return nil, fmt.Errorf("number of cores must not be more than maximum available number of cores (%d)", availableCores)
		}

		if cores == 0 {
			cores = availableCores
		}

		cpuLoad := extutil.ToInt(request.Config["cpuLoad"])

		if cpuLoad < 1 || cpuLoad > 100 {
			return nil, extension_kit.ToError("cpu load must be in an inclusive range from 1%% to 100%%", nil)
		}

		priority := extutil.ToString(request.Config["priority"])

		if !slices.Contains(validProcessPriorities[:], priority) {
			return nil, extension_kit.ToError("priority must be one of the following: 'Normal', 'High', 'RealTime'.", nil)
		}

		return &CpuStressOpts{
			Cores:    cores,
			CpuLoad:  cpuLoad,
			Duration: duration,
			Priority: priority,
		}, nil
	}
}

func getStressCpuDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.stress-cpu", BaseActionID),
		Label:       "Stress CPU",
		Description: "Generates CPU load for one or more cores.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(stressCPUIcon),
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
				Name:         "cpuLoad",
				Label:        "Host CPU Load",
				Description:  extutil.Ptr("How much CPU should be consumed?"),
				Type:         action_kit_api.ActionParameterTypePercentage,
				DefaultValue: extutil.Ptr("80"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
				MinValue:     extutil.Ptr(0),
				MaxValue:     extutil.Ptr(100),
			},
			{
				Name:         "cores",
				Label:        "Host Cores",
				Description:  extutil.Ptr("How many CPU cores should be targeted during the stress attack?"),
				Type:         action_kit_api.ActionParameterTypeStressngWorkers,
				DefaultValue: extutil.Ptr("1"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
			{
				Name:         "priority",
				Label:        "Process Priority",
				Description:  extutil.Ptr("What is the priority of the stress process?"),
				Type:         action_kit_api.ActionParameterTypeString,
				DefaultValue: extutil.Ptr("Normal"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(3),
				Options: &[]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "Normal",
						Value: validProcessPriorities[0],
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Above Normal",
						Value: validProcessPriorities[1],
					},
					action_kit_api.ExplicitParameterOption{
						Label: "High",
						Value: validProcessPriorities[2],
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Real Time",
						Value: validProcessPriorities[3],
					},
				},
			},
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long should CPU be stressed?"),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("30s"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(4),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

// Describe returns the action description for the platform with all required information.
func (a *cpuStressAction) Describe() action_kit_api.ActionDescription {
	return a.description
}

// Prepare is called before the action is started.
// It can be used to validate the parameters and prepare the action.
// It must not cause any harmful effects.
// The passed in state is included in the subsequent calls to start/status/stop.
// So the state should contain all information needed to execute the action and even more important: to be able to stop it.
func (a *cpuStressAction) Prepare(ctx context.Context, state *StressActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	if _, err := CheckTargetHostname(request.Target.Attributes); err != nil {
		return nil, err
	}

	err := utils.IsExecutableOperational(steadybitStressCpuExecutableName, "--version")

	if err != nil {
		return nil, err
	}

	opts, err := a.optsProvider(request)
	if err != nil {
		return nil, err
	}

	state.StressOpts = *opts
	state.ExecutionId = request.ExecutionId
	return nil, nil
}

func (a *cpuStressAction) Start(ctx context.Context, state *StressActionState) (*action_kit_api.StartResult, error) {
	command := exec.CommandContext(context.Background(), steadybitStressCpuExecutableName, state.StressOpts.Args()...)

	go func() {
		output, err := command.CombinedOutput()

		if err != nil {
			log.Error().Msg("Failed to start cpu stress attack.")
		}

		log.Info().Msgf("%s", output)
	}()

	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{
			{
				Level:   extutil.Ptr(action_kit_api.Info),
				Message: fmt.Sprintf("Starting stress host with args: %s.", fmt.Sprintf("\"%s\"", strings.Join(state.StressOpts.Args(), " "))),
			},
		}),
	}, nil
}

func (a *cpuStressAction) Status(_ context.Context, state *StressActionState) (*action_kit_api.StatusResult, error) {
	isRunning, err := utils.IsProcessRunning(steadybitStressCpuExecutableName)

	if err != nil {
		return nil, err
	}

	if isRunning {
		return &action_kit_api.StatusResult{Completed: false}, nil
	}

	return &action_kit_api.StatusResult{
		Completed: true,
		Messages: &[]action_kit_api.Message{
			{
				Level:   extutil.Ptr(action_kit_api.Info),
				Message: "Stress host stopped",
			},
		},
	}, nil

}

func (a *cpuStressAction) Stop(_ context.Context, state *StressActionState) (*action_kit_api.StopResult, error) {
	messages := make([]action_kit_api.Message, 0)

	err := utils.StopProcess(steadybitStressCpuExecutableName)

	if err != nil {
		return nil, err
	}

	messages = append(messages, action_kit_api.Message{
		Level:   extutil.Ptr(action_kit_api.Info),
		Message: "Canceled stress host",
	})

	return &action_kit_api.StopResult{
		Messages: &messages,
	}, nil
}
