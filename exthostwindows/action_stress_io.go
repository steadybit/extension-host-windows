package exthostwindows

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type StressLayer int

const (
	FileSystem StressLayer = iota
	NamedPartition
	PhysicalDisk
)

var ioStressLayer = map[StressLayer]string{
	FileSystem:     "File System",
	NamedPartition: "Named Partition",
	PhysicalDisk:   "Physical Disk",
}

func (sl StressLayer) String() string {
	return ioStressLayer[sl]
}

type ioStressAction struct {
	description  action_kit_api.ActionDescription
	optsProvider ioStressOptsProvider
}

type IoStressOpts struct {
	StressLayer        StressLayer
	StressLayerInput   string
	ThreadCount        uint
	Duration           time.Duration
	DisableSwHwCaching bool
}

func (o *IoStressOpts) Args() []string {
	args := []string{fmt.Sprintf("-d%d", int(o.Duration.Seconds()))}
	args = append(args, fmt.Sprintf("-F%d", o.ThreadCount))
	if o.DisableSwHwCaching {
		args = append(args, "-Sh")
	}

	if o.StressLayer == PhysicalDisk {
		args = append(args, fmt.Sprintf("#%s", o.StressLayerInput))
	} else if o.StressLayer == NamedPartition {
		args = append(args, fmt.Sprintf("%s:", o.StressLayerInput))
	} else {
		args = append(args, o.StressLayerInput)
	}

	return args
}

type IoStressActionState struct {
	StressOpts  IoStressOpts
	ExecutionId uuid.UUID
}

var (
	_ action_kit_sdk.Action[IoStressActionState]           = (*ioStressAction)(nil)
	_ action_kit_sdk.ActionWithStatus[IoStressActionState] = (*ioStressAction)(nil)
	_ action_kit_sdk.ActionWithStop[IoStressActionState]   = (*ioStressAction)(nil) // Optional, needed when the action needs a stop method
)

type ioStressOptsProvider func(request action_kit_api.PrepareActionRequestBody) (*IoStressOpts, error)

func NewStressIoAction() action_kit_sdk.Action[IoStressActionState] {
	return &ioStressAction{
		description:  getStressIoDescription(),
		optsProvider: stressIo(),
	}
}

func (a *ioStressAction) NewEmptyState() IoStressActionState {
	return IoStressActionState{}
}

func stressIo() ioStressOptsProvider {
	return func(request action_kit_api.PrepareActionRequestBody) (*IoStressOpts, error) {
		duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond

		if duration < 1*time.Second {
			return nil, errors.New("duration must be greater / equal than 1s")
		}

		stressLayer := extutil.ToString(request.Config["stressLayer"])

		if stressLayer != FileSystem.String() && stressLayer != NamedPartition.String() && stressLayer != PhysicalDisk.String() {
			return nil, fmt.Errorf("stress layer must be one of the following: %s, %s, %s", FileSystem.String(), NamedPartition.String(), PhysicalDisk.String())
		}

		stressLayerInput := extutil.ToString(request.Config["stressLayerInput"])

		var stressLayerEnum StressLayer

		if stressLayer == FileSystem.String() {
			stressLayerEnum = FileSystem
			if _, err := os.Stat(stressLayerInput); err != nil {
				return nil, err
			}
		}

		if stressLayer == NamedPartition.String() {
			stressLayerEnum = NamedPartition
			stressLayerInput = string(bytes.ToUpper([]byte(stressLayerInput)))

			if len(stressLayerInput) != 1 {
				return nil, fmt.Errorf("disk letter must be a letter from A-Z")
			}

			character := rune(stressLayerInput[0])

			if !unicode.IsLetter(character) && (character >= 'A' && character <= 'Z') {
				return nil, fmt.Errorf("disk letter must be a letter from A-Z")
			}
		}

		if stressLayer == PhysicalDisk.String() {
			stressLayerEnum = PhysicalDisk
			deviceId, err := strconv.ParseUint(stressLayerInput, 10, 0)
			if err != nil {
				return nil, err
			}

			isDeviceAvailable, err := isPhysicalDeviceAvailable(deviceId)

			if err != nil {
				return nil, err
			}

			if !isDeviceAvailable {
				return nil, fmt.Errorf("physical device %d is not available", deviceId)
			}
		}

		threadCount := extutil.ToInt(request.Config["threadCount"])
		availableThreadCount := runtime.NumCPU()

		if threadCount == 0 {
			threadCount = availableThreadCount
		}

		if threadCount > availableThreadCount {
			return nil, fmt.Errorf("number of cores must not be more than maximum available number of cores (%d)", availableThreadCount)
		}

		disableSwHwCaching := extutil.ToBool(request.Config["disableSwHwCaching"])

		return &IoStressOpts{
			Duration:           duration,
			StressLayer:        stressLayerEnum,
			StressLayerInput:   stressLayerInput,
			ThreadCount:        uint(threadCount),
			DisableSwHwCaching: disableSwHwCaching,
		}, nil
	}
}

func getStressIoDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.stress-io", BaseActionID),
		Label:       "Stress IO",
		Description: "Stresses IO on the host using read/write operations for the given duration.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(stressIOIcon),
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
				Name:         "stressLayer",
				Label:        "IO Stress Layer",
				Description:  extutil.Ptr("On which layer IO is stressed?"),
				Type:         action_kit_api.ActionParameterTypeString,
				DefaultValue: extutil.Ptr(NamedPartition.String()),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
				Options: &[]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "File System - requires file path in the next field",
						Value: FileSystem.String(),
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Named Partition - requires drive letter in the next field",
						Value: NamedPartition.String(),
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Physical Disk - requires disk id in the next field",
						Value: PhysicalDisk.String(),
					},
				},
			},
			{
				Name:        "stressLayerInput",
				Label:       "Stress Layer Input",
				Description: extutil.Ptr("Based on the previous answer add the value here."),
				Type:        action_kit_api.ActionParameterTypeString,
				Required:    extutil.Ptr(true),
				Order:       extutil.Ptr(2),
			},
			{
				Name:         "threadCount",
				Label:        "Thread Count",
				Description:  extutil.Ptr("Total number of threads used in the attack."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				Required:     extutil.Ptr(true),
				DefaultValue: extutil.Ptr("1"),
			},
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long should IO be stressed?"),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("30s"),
				Required:     extutil.Ptr(true),
			},
			{
				Name:         "disableSwHwCaching",
				Label:        "Disable Software & Hardware Caching",
				Description:  extutil.Ptr("Disables both software and hardware write caching."),
				Type:         action_kit_api.ActionParameterTypeBoolean,
				DefaultValue: extutil.Ptr("true"),
				Required:     extutil.Ptr(true),
				Advanced:     extutil.Ptr(true),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

// Describe returns the action description for the platform with all required information.
func (a *ioStressAction) Describe() action_kit_api.ActionDescription {
	return a.description
}

// Prepare is called before the action is started.
// It can be used to validate the parameters and prepare the action.
// It must not cause any harmful effects.
// The passed in state is included in the subsequent calls to start/status/stop.
// So the state should contain all information needed to execute the action and even more important: to be able to stop it.
func (a *ioStressAction) Prepare(ctx context.Context, state *IoStressActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	if _, err := CheckTargetHostname(request.Target.Attributes); err != nil {
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

func (a *ioStressAction) Start(ctx context.Context, state *IoStressActionState) (*action_kit_api.StartResult, error) {
	command := exec.CommandContext(context.Background(), "diskspd", state.StressOpts.Args()...)

	log.Info().Msgf("Running command: %s, %s.", command.Path, command.Args)

	go func() {
		output, err := command.CombinedOutput()

		if err != nil {
			log.Error().Msgf("Failed to start io stress attack: %s.", err)
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

func (a *ioStressAction) Status(_ context.Context, state *IoStressActionState) (*action_kit_api.StatusResult, error) {
	isRunning, err := isDskSpdRunning()

	if err != nil {
		return &action_kit_api.StatusResult{
			Completed: true,
			Error: &action_kit_api.ActionKitError{
				Status: extutil.Ptr(action_kit_api.Failed),
				Title:  fmt.Sprintf("unable to retrieve 'diskspd' process status: %s", err),
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
				Message: "Stress host stopped",
			},
		},
	}, nil

}

func (a *ioStressAction) Stop(_ context.Context, state *IoStressActionState) (*action_kit_api.StopResult, error) {
	messages := make([]action_kit_api.Message, 0)
	isRunning, err := isDskSpdRunning()

	if err != nil {
		return nil, err
	}

	if isRunning {
		cmd := exec.Command("powershell", "-Command", "Stop-Process", "-Name", "diskspd", "-Force")
		out, err := cmd.CombinedOutput()

		if err != nil {
			return nil, err
		}

		log.Info().Msgf("%s", out)
	}

	messages = append(messages, action_kit_api.Message{
		Level:   extutil.Ptr(action_kit_api.Info),
		Message: "Canceled stress host",
	})

	return &action_kit_api.StopResult{
		Messages: &messages,
	}, nil
}

func isDskSpdInstalled() bool {
	cmd := exec.Command("diskspd")
	cmd.Dir = os.TempDir()
	var outputBuffer bytes.Buffer
	cmd.Stdout = &outputBuffer
	cmd.Stderr = &outputBuffer
	err := cmd.Run()
	if err != nil {
		log.Error().Err(err).Msg("failed to start diskspd")
		return false
	}
	success := cmd.ProcessState.Success()
	if !success {
		log.Error().Err(err).Msgf("diskspd is not installed: 'diskspd' in %v returned: %v", os.TempDir(), outputBuffer.Bytes())
	}
	return success
}

func isDskSpdRunning() (bool, error) {
	cmd := exec.Command("powershell", "-Command", "Get-Process", "-Name", "diskspd", "-ErrorAction", "SilentlyContinue")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

func isPhysicalDeviceAvailable(deviceId uint64) (bool, error) {
	cmd := exec.Command("powershell", "-Command", "Get-PhysicalDisk | ForEach-Object { $_.DeviceId }")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	reader := strings.NewReader(string(output))
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		id, err := strconv.ParseUint(line, 10, 0)
		if err != nil {
			return false, err
		}

		if deviceId == id {
			return true, nil
		}
	}

	return false, nil
}
