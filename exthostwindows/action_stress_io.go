package exthostwindows

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type IOStressLayer string

const (
	FileSystem     IOStressLayer = "File System"
	NamedPartition IOStressLayer = "Named Partition"
	PhysicalDisk   IOStressLayer = "Physical Disk"
)

var IOStressLayers = struct {
	FileSystem     IOStressLayer
	NamedPartition IOStressLayer
	PhysicalDisk   IOStressLayer
}{
	FileSystem:     FileSystem,
	NamedPartition: NamedPartition,
	PhysicalDisk:   PhysicalDisk,
}

func (sl IOStressLayer) IsValid() bool {
	switch sl {
	case FileSystem, NamedPartition, PhysicalDisk:
		return true
	default:
		return false
	}
}

type ioStressAction struct {
	description  action_kit_api.ActionDescription
	optsProvider ioStressOptsProvider
}

type IoStressOpts struct {
	StressLayer        IOStressLayer
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

		if duration < time.Second {
			return nil, errors.New("duration must be greater / equal than 1s")
		}

		stressLayer := IOStressLayer(extutil.ToString(request.Config["stressLayer"]))

		if !stressLayer.IsValid() {
			return nil, fmt.Errorf("stress layer must be one of the following: %s, %s, %s, current: %s", IOStressLayers.FileSystem, IOStressLayers.NamedPartition, IOStressLayers.PhysicalDisk, stressLayer)
		}

		stressLayerInput := extutil.ToString(request.Config["stressLayerInput"])

		if stressLayer == IOStressLayers.FileSystem {
			if _, err := os.Stat(stressLayerInput); err != nil {
				return nil, err
			}
		}

		if stressLayer == IOStressLayers.NamedPartition {
			stressLayerInput = string(bytes.ToUpper([]byte(stressLayerInput)))

			if len(stressLayerInput) != 1 {
				return nil, fmt.Errorf("disk letter must be a letter from A-Z")
			}

			character := rune(stressLayerInput[0])

			if !unicode.IsLetter(character) || !(character >= 'A' && character <= 'Z') {
				return nil, fmt.Errorf("disk letter must be a letter from A-Z")
			}
		}

		if stressLayer == IOStressLayers.PhysicalDisk {
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

		threadCount := extutil.ToUInt(request.Config["threadCount"])

		if threadCount <= 0 {
			return nil, fmt.Errorf("number of threads must be greater than 0")
		}

		disableSwHwCaching := extutil.ToBool(request.Config["disableSwHwCaching"])

		return &IoStressOpts{
			Duration:           duration,
			StressLayer:        stressLayer,
			StressLayerInput:   stressLayerInput,
			ThreadCount:        threadCount,
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
			TargetType:         targetID,
			SelectionTemplates: &targetSelectionTemplates,
		}),
		Technology:  extutil.Ptr(WindowsHostTechnology),
		Category:    extutil.Ptr("Resource"),
		Kind:        action_kit_api.Attack,
		TimeControl: action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "stressLayer",
				Label:        "IO Stress Layer",
				Description:  extutil.Ptr("On which layer IO is stressed?"),
				Type:         action_kit_api.ActionParameterTypeString,
				DefaultValue: extutil.Ptr(string(IOStressLayers.NamedPartition)),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
				Options: &[]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "File System - requires file path in the next field",
						Value: string(IOStressLayers.FileSystem),
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Named Partition - requires drive letter in the next field",
						Value: string(IOStressLayers.NamedPartition),
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Physical Disk - requires disk id in the next field",
						Value: string(IOStressLayers.PhysicalDisk),
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
				DefaultValue: extutil.Ptr("12"),
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

func (a *ioStressAction) Describe() action_kit_api.ActionDescription {
	return a.description
}

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
	executable := resolveExecutable("diskspd", "STEADYBIT_DISKSPD")

	err := utils.IsExecutableOperational(executable, "-?")

	if err != nil {
		return nil, err
	}

	command := exec.CommandContext(context.Background(), executable, state.StressOpts.Args()...)

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
	isRunning, err := utils.IsProcessRunning("diskspd")

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

func (a *ioStressAction) Stop(_ context.Context, state *IoStressActionState) (*action_kit_api.StopResult, error) {
	messages := make([]action_kit_api.Message, 0)

	err := utils.StopProcess("diskspd")

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
