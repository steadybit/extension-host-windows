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
	"github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"golang.org/x/sync/syncmap"
)

type Opts struct {
	Cores    *int
	CpuLoad  int
	Priority string
	Duration time.Duration
}

var (
	_ action_kit_sdk.Action[StressActionState]           = (*cpuStressAction)(nil)
	_ action_kit_sdk.ActionWithStatus[StressActionState] = (*cpuStressAction)(nil)
	_ action_kit_sdk.ActionWithStop[StressActionState]   = (*cpuStressAction)(nil) // Optional, needed when the action needs a stop method
)

type stressOptsProvider func(request action_kit_api.PrepareActionRequestBody) (Opts, error)

type cpuStressAction struct {
	description  action_kit_api.ActionDescription
	optsProvider stressOptsProvider
	stresses     syncmap.Map
}

type StressActionState struct {
	StressOpts  Opts
	ExecutionId uuid.UUID
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
		Technology: extutil.Ptr("Host"),
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
				Label:        "Host CPUs",
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

func NewStressCpuAction(description func() action_kit_api.ActionDescription, optsProvider stressOptsProvider) action_kit_sdk.Action[StressActionState] {
	return &cpuStressAction{
		description:  description(),
		optsProvider: optsProvider,
		stresses:     syncmap.Map{},
	}
}

func (a *cpuStressAction) NewEmptyState() StressActionState {
	return StressActionState{}
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

	opts, err := a.optsProvider(request)
	if err != nil {
		return nil, err
	}

	adaptCpuHosts(&opts)

	state.StressOpts = opts
	state.ExecutionId = request.ExecutionId
	return nil, nil
}

func adaptCpuHosts(s *Opts) {
	if s.Cores == nil || *s.Cores != 0 {
		return
	}

	//stress-ng will use all configured processors, we deem this to be wrong and expect all online cpus to be used.
	if c, err := utils.ReadCpusAllowedCount("/proc/1/status"); err == nil {
		s.Cores = extutil.Ptr(c)
	} else {
		log.Debug().Err(err).Msg("failed to read cpus allowed for pid 1")
	}
}

func (a *cpuStressAction) Start(ctx context.Context, state *StressActionState) (*action_kit_api.StartResult, error) {
	s, err := stress.New(ctx, a.runc, state.Sidecar, state.StressOpts)
	if err != nil {
		return nil, extension_kit.ToError("Failed to stress host", err)
	}

	a.stresses.Store(state.ExecutionId, s)

	if err := s.Start(); err != nil {
		return nil, extension_kit.ToError("Failed to stress host", err)
	}

	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{
			{
				Level:   extutil.Ptr(action_kit_api.Info),
				Message: fmt.Sprintf("Starting stress host with args %s", strings.Join(state.StressOpts.Args(), " ")),
			},
		}),
	}, nil
}

func (a *cpuStressAction) Status(_ context.Context, state *StressActionState) (*action_kit_api.StatusResult, error) {
	exited, err := a.stressExited(state.ExecutionId)
	if !exited {
		return &action_kit_api.StatusResult{Completed: false}, nil
	}

	if err == nil {
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

	errMessage := err.Error()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode := exitErr.ExitCode()
		if len(exitErr.Stderr) > 0 {
			errMessage = fmt.Sprintf("%s\n%s", exitErr.Error(), string(exitErr.Stderr))
		}

		for _, ignore := range state.IgnoreExitCodes {
			if exitCode == ignore {
				return &action_kit_api.StatusResult{
					Completed: true,
					Messages: &[]action_kit_api.Message{
						{
							Level:   extutil.Ptr(action_kit_api.Warn),
							Message: fmt.Sprintf("stress-ng exited unexpectedly: %s", errMessage),
						},
					},
				}, nil
			}
		}
	}

	return &action_kit_api.StatusResult{
		Completed: true,
		Error: &action_kit_api.ActionKitError{
			Status: extutil.Ptr(action_kit_api.Failed),
			Title:  fmt.Sprintf("Failed to stress host: %s", errMessage),
		},
	}, nil
}

func (a *cpuStressAction) Stop(_ context.Context, state *StressActionState) (*action_kit_api.StopResult, error) {
	messages := make([]action_kit_api.Message, 0)

	stopped := a.stopStressHost(state.ExecutionId)
	if stopped {
		messages = append(messages, action_kit_api.Message{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: "Canceled stress host",
		})
	}

	return &action_kit_api.StopResult{
		Messages: &messages,
	}, nil
}

func (a *cpuStressAction) stressExited(executionId uuid.UUID) (bool, error) {
	s, ok := a.stresses.Load(executionId)
	if !ok {
		return true, nil
	}
	return s.(*stress.Stress).Exited()
}

func (a *cpuStressAction) stopStressHost(executionId uuid.UUID) bool {
	s, ok := a.stresses.LoadAndDelete(executionId)
	if !ok {
		return false
	}
	s.(*stress.Stress).Stop()
	return true
}

func isSteadybitStressCpuInstalled() bool {
	cmd := exec.Command("steadybit-stress-cpu", "--version")
	cmd.Dir = os.TempDir()
	var outputBuffer bytes.Buffer
	cmd.Stdout = &outputBuffer
	cmd.Stderr = &outputBuffer
	err := cmd.Run()
	if err != nil {
		log.Error().Err(err).Msg("failed to Start steadybit-stress-cpu")
		return false
	}
	success := cmd.ProcessState.Success()
	if !success {
		log.Error().Err(err).Msgf("steadybit-stress-cpu is not installed: 'stress-ng -V' in %v returned: %v", os.TempDir(), outputBuffer.Bytes())
	}
	return success
}
