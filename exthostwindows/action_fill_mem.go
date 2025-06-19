package exthostwindows

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
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

type Mode string
type Unit string

const (
	ModeUsage    Mode = "usage"
	ModeAbsolute Mode = "absolute"
	UnitPercent  Unit = "%"
	UnitMegabyte Unit = "MiB"
)

var FillMemoryModes = struct {
	Usage    Mode
	Absolute Mode
}{
	Usage:    ModeUsage,
	Absolute: ModeAbsolute,
}

var FillMemoryUnits = struct {
	Percent  Unit
	Megabyte Unit
}{
	Percent:  UnitPercent,
	Megabyte: UnitMegabyte,
}

func (m Mode) IsValid() bool {
	switch m {
	case FillMemoryModes.Absolute, FillMemoryModes.Usage:
		return true
	default:
		return false
	}
}

func (u Unit) IsValid() bool {
	switch u {
	case FillMemoryUnits.Megabyte, FillMemoryUnits.Percent:
		return true
	default:
		return false
	}
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

		mode := Mode(extutil.ToString(request.Config["mode"]))

		if !mode.IsValid() {
			return nil, fmt.Errorf("mode must be one of the following: %s, %s", FillMemoryModes.Absolute, FillMemoryModes.Usage)
		}

		unit := Unit(extutil.ToString(request.Config["unit"]))

		if !unit.IsValid() {
			return nil, fmt.Errorf("unit must be one of the following: %s, %s", FillMemoryUnits.Megabyte, FillMemoryUnits.Percent)
		}

		size := extutil.ToUInt(request.Config["size"])

		if size <= 0 {
			return nil, fmt.Errorf("size must be more than 0")
		}

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

func (a *fillMemAction) Describe() action_kit_api.ActionDescription {
	return a.description
}

func (a *fillMemAction) Prepare(ctx context.Context, state *FillMemActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	if _, err := CheckTargetHostname(request.Target.Attributes); err != nil {
		return nil, err
	}

	err := utils.IsExecutableOperational("memfill", "--help")

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
	isRunning, err := utils.IsProcessRunning("memfill")

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
				Message: "Memfill stopped",
			},
		},
	}, nil

}

func (a *fillMemAction) Stop(_ context.Context, state *FillMemActionState) (*action_kit_api.StopResult, error) {
	messages := make([]action_kit_api.Message, 0)

	err := utils.StopProcess("memfill")

	if err != nil {
		return nil, err
	}

	messages = append(messages, action_kit_api.Message{
		Level:   extutil.Ptr(action_kit_api.Info),
		Message: "Canceled memfill",
	})

	return &action_kit_api.StopResult{
		Messages: &messages,
	}, nil
}
