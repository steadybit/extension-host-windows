package exthostwindows

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type FillMode string
type FillMethod string

const (
	Percentage FillMode   = "PERCENTAGE"
	MBToFill   FillMode   = "MB_TO_FILL"
	MBLeft     FillMode   = "MB_LEFT"
	AtOnce     FillMethod = "AT_ONCE"
	OverTime   FillMethod = "OVER_TIME"
)

var FillDiskModes = struct {
	Percentage FillMode
	MBToFill   FillMode
	MBLeft     FillMode
}{
	Percentage: Percentage,
	MBToFill:   MBToFill,
	MBLeft:     MBLeft,
}

var FillDiskMethods = struct {
	AtOnce   FillMethod
	OverTime FillMethod
}{
	AtOnce:   AtOnce,
	OverTime: OverTime,
}

func (fm FillMode) IsValid() bool {
	switch fm {
	case FillDiskModes.MBLeft, FillDiskModes.Percentage, FillDiskModes.MBToFill:
		return true
	default:
		return false
	}
}

func (fm FillMethod) IsValid() bool {
	switch fm {
	case FillDiskMethods.AtOnce, FillDiskMethods.OverTime:
		return true
	default:
		return false
	}
}

type fillDiskAction struct {
	description  action_kit_api.ActionDescription
	optsProvider fillDiskOptsProvider
}

type FillDiskOpts struct {
	Duration  time.Duration
	FillMode  FillMode
	Method    FillMethod
	Path      string
	ByteSize  uint64
	BlockSize uint
	FilePath  string
}

func BytesToMegabytes(bytes uint64) uint64 {
	return bytes / 1000 / 1000
}

func (o *FillDiskOpts) Args() []string {
	args := []string{}

	if o.Method == AtOnce {
		args = []string{"file", "createNew", o.FilePath}
		args = append(args, fmt.Sprintf("%d", o.ByteSize))
	}

	if o.Method == OverTime {
		args = []string{"dd", fmt.Sprintf("of=%s", o.FilePath)}

		allocationInMB := BytesToMegabytes(o.ByteSize)

		if uint64(o.BlockSize) > allocationInMB {
			o.BlockSize = uint(allocationInMB)
		}

		numberOfBlocks := o.ByteSize / uint64(o.BlockSize)

		args = append(args, "iflag=fullblock", fmt.Sprintf("bs=%dM", o.BlockSize), fmt.Sprintf("count=%d", BytesToMegabytes(numberOfBlocks)))
	}

	return args
}

type FillDiskActionState struct {
	StressOpts  FillDiskOpts
	ExecutionId uuid.UUID
}

var (
	_ action_kit_sdk.Action[FillDiskActionState]         = (*fillDiskAction)(nil)
	_ action_kit_sdk.ActionWithStop[FillDiskActionState] = (*fillDiskAction)(nil) // Optional, needed when the action needs a stop method
)

type fillDiskOptsProvider func(request action_kit_api.PrepareActionRequestBody) (*FillDiskOpts, error)

func NewFillDiskAction() action_kit_sdk.Action[FillDiskActionState] {
	return &fillDiskAction{
		description:  getFillDiskDescription(),
		optsProvider: fillDisk(),
	}
}

func (a *fillDiskAction) NewEmptyState() FillDiskActionState {
	return FillDiskActionState{}
}

func fillDisk() fillDiskOptsProvider {
	return func(request action_kit_api.PrepareActionRequestBody) (*FillDiskOpts, error) {
		duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond

		if duration < 1*time.Second {
			return nil, errors.New("duration must be greater / equal than 1s")
		}

		mode := FillMode(extutil.ToString(request.Config["mode"]))

		if !mode.IsValid() {
			return nil, fmt.Errorf("mode must be one of the following: %s, %s, %s", FillDiskModes.MBToFill, FillDiskModes.MBLeft, FillDiskModes.Percentage)
		}

		method := FillMethod(extutil.ToString(request.Config["method"]))

		if !method.IsValid() {
			return nil, fmt.Errorf("unit must be one of the following: %s, %s", FillDiskMethods.AtOnce, FillDiskMethods.OverTime)
		}

		path := extutil.ToString(request.Config["path"])

		if len(path) == 0 {
			return nil, errors.New("path must not be empty")
		}

		splitPath := strings.Split(path, ":")

		if len(splitPath) != 2 || len(splitPath[0]) != 1 {
			return nil, fmt.Errorf("path must be absolute and start with a drive letter, given: %s", path)
		}

		driveLetter := splitPath[0]

		fileInfo, err := os.Stat(path)

		if err != nil {
			return nil, err
		}

		if !fileInfo.IsDir() {
			return nil, errors.New("path must be a directory")
		}

		size := extutil.ToUInt(request.Config["size"])

		amountToAllocate, err := calculateAllocation(mode, driveLetter, uint64(size))

		if err != nil {
			return nil, err
		}

		blockSize := extutil.ToUInt(request.Config["blocksize"])

		if blockSize <= 1 || blockSize > 1024 {
			return nil, fmt.Errorf("blocksize must be at least 1 and lesser or equal to 1024")
		}

		return &FillDiskOpts{
			Duration:  duration,
			ByteSize:  amountToAllocate,
			Method:    method,
			FillMode:  mode,
			Path:      path,
			BlockSize: blockSize,
			FilePath:  filepath.Join(path, fmt.Sprintf("steadybit-disk-fill-%s", uuid.NewString())),
		}, nil
	}
}

func calculateAllocation(fillMode FillMode, driveLetter string, percentageOrMegabytes uint64) (uint64, error) {
	availableSpace, err := utils.GetDriveSpace(driveLetter, utils.Available)

	if err != nil {
		return 0, err
	}

	if fillMode == MBLeft {
		wantToAllocate := percentageOrMegabytes * 1000 * 1000
		if availableSpace < wantToAllocate {
			return 0, fmt.Errorf("not enough space on the drive")
		}

		return availableSpace - wantToAllocate, nil
	}

	if fillMode == MBToFill {
		wantToAllocate := percentageOrMegabytes * 1000 * 1000
		if availableSpace < wantToAllocate {
			return 0, fmt.Errorf("not enough space on the drive")
		}

		return wantToAllocate, nil
	}

	if fillMode == Percentage {
		totalSpace, err := utils.GetDriveSpace(driveLetter, utils.Total)

		if err != nil {
			return 0, err
		}

		currentlyAllocated := ((float64(totalSpace) - float64(availableSpace)) / float64(totalSpace)) * 100

		if currentlyAllocated > float64(percentageOrMegabytes) {
			return 0, nil
		}

		totalModifier := (float64(percentageOrMegabytes) - currentlyAllocated) / 100

		wantToAllocate := float64(totalSpace) * totalModifier

		return uint64(wantToAllocate), nil
	}

	return 0, fmt.Errorf("unknown fill mode")
}

func getFillDiskDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.fill_disk", BaseActionID),
		Label:       "Fill Disk",
		Description: "Fills the disk of the host for the given duration.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(string(fillDiskIcon)),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType:         targetID,
			SelectionTemplates: &targetSelectionTemplates,
		}),
		Technology: extutil.Ptr(WindowsHostTechnology),
		Category:   extutil.Ptr("Resource"),

		Kind: action_kit_api.Attack,

		TimeControl: action_kit_api.TimeControlExternal,

		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long should the disk be filled?"),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("30s"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			{
				Name:         "mode",
				Label:        "Mode",
				Description:  extutil.Ptr("Decide how to specify the amount to fill the disk:\n\noverall percentage of filled disk space in percent,\n\nMegabytes to write,\n\nMegabytes to leave free on disk"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
				DefaultValue: extutil.Ptr("PERCENTAGE"),
				Type:         action_kit_api.String,
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "Overall percentage of filled disk space in percent",
						Value: string(Percentage),
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Megabytes to write",
						Value: string(MBToFill),
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Megabytes to leave free on disk",
						Value: string(MBLeft),
					},
				}),
			},
			{
				Name:         "size",
				Label:        "Fill Value (depending on Mode)",
				Description:  extutil.Ptr("Depending on the mode, specify the percentage of filled disk space or the number of Megabytes to be written or left free."),
				Type:         action_kit_api.Integer,
				DefaultValue: extutil.Ptr("80"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(3),
			},
			{
				Name:         "path",
				Label:        "File Destination",
				Description:  extutil.Ptr("Where to temporarily write the file for filling the disk. It will be cleaned up afterwards."),
				Type:         action_kit_api.String,
				DefaultValue: extutil.Ptr("C:\\"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(4),
			},
			{
				Name:         "method",
				Label:        "Method used to fill disk",
				Description:  extutil.Ptr("Should the disk filled at once or over time?"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(5),
				DefaultValue: extutil.Ptr("AT_ONCE"),
				Type:         action_kit_api.String,
				Advanced:     extutil.Ptr(true),
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "At once (fsutil)",
						Value: string(AtOnce),
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Over time (dd)",
						Value: string(OverTime),
					},
				}),
			},
			{
				Name:         "blocksize",
				Label:        "Block Size (in MBytes) of the File to Write for method `OverTime`",
				Description:  extutil.Ptr("Define the block size for writing the file with the dd command. If the block size is larger than the fill value, the fill value will be used as block size."),
				Type:         action_kit_api.Integer,
				DefaultValue: extutil.Ptr("5"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(6),
				MinValue:     extutil.Ptr(1),
				MaxValue:     extutil.Ptr(1024),
				Advanced:     extutil.Ptr(true),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func (a *fillDiskAction) Describe() action_kit_api.ActionDescription {
	return a.description
}

func (a *fillDiskAction) Prepare(ctx context.Context, state *FillDiskActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
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

func (a *fillDiskAction) Start(ctx context.Context, state *FillDiskActionState) (*action_kit_api.StartResult, error) {
	if state.StressOpts.Method == AtOnce {
		err := utils.IsExecutableOperational("fsutil")

		if err != nil {
			return nil, err
		}
		command := exec.CommandContext(context.Background(), "fsutil", state.StressOpts.Args()...)
		go func() {
			log.Info().Msgf("Running command: %s, %s.", command.Path, command.Args)
			output, err := command.CombinedOutput()

			if err != nil {
				log.Error().Msgf("Failed to start disk fill attack: %s.", err)
			}

			log.Info().Msgf("%s", output)
		}()
	} else {
		executable := resolveExecutable("coreutils", "STEADYBIT_COREUTILS")

		err := utils.IsExecutableOperational(executable, "dd", "--help")

		if err != nil {
			return nil, err
		}

		err = utils.IsExecutableOperational("devzero", "--help")

		if err != nil {
			return nil, err
		}

		bgCtx := context.Background()
		devzeroCmd := exec.CommandContext(bgCtx, "devzero")
		ddCmd := exec.CommandContext(bgCtx, executable, state.StressOpts.Args()...)

		log.Info().Msgf("Running command: %s, %s.", ddCmd.Path, ddCmd.Args)

		go func() {
			pipeReader, pipeWriter := io.Pipe()
			defer pipeReader.Close()
			defer pipeWriter.Close()

			devzeroCmd.Stdout = pipeWriter
			ddCmd.Stdin = pipeReader

			ddCmd.Stdout = os.Stdout
			ddCmd.Stderr = os.Stderr

			if err := devzeroCmd.Start(); err != nil {
				log.Err(err).Msgf("failed to start devzero")
			}

			if err := ddCmd.Start(); err != nil {
				log.Err(err).Msgf("failed to start dd")
			}

			if err := ddCmd.Wait(); err != nil {
				log.Err(err).Msg("dd failed executing: might have been stopped forcefully")
			}

			if err := devzeroCmd.Process.Kill(); err != nil {
				log.Err(err).Msg("failed to stop devzero")
			}

			devzeroCmd.Wait()
		}()
	}

	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{
			{
				Level:   extutil.Ptr(action_kit_api.Info),
				Message: fmt.Sprintf("Starting disk fill with args: %s.", fmt.Sprintf("\"%s\"", strings.Join(state.StressOpts.Args(), " "))),
			},
		}),
	}, nil
}

func (a *fillDiskAction) Stop(_ context.Context, state *FillDiskActionState) (*action_kit_api.StopResult, error) {
	messages := make([]action_kit_api.Message, 0)

	err := os.Remove(state.StressOpts.FilePath)

	if err != nil {
		return nil, err
	}

	err = utils.StopProcess("coreutils")

	if err != nil {
		return nil, err
	}

	err = utils.StopProcess("devzero")

	if err != nil {
		return nil, err
	}

	messages = append(messages, action_kit_api.Message{
		Level:   extutil.Ptr(action_kit_api.Info),
		Message: "Canceled fsutil",
	})

	return &action_kit_api.StopResult{
		Messages: &messages,
	}, nil
}
