package exthostwindows

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
)

func TestActionFillMem_Prepare(t *testing.T) {
	getExecutionId := getExecutionIdByTestId()
	osHostname = func() (string, error) {
		return "myhostname", nil
	}
	hostname := "myhostname"
	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError string
		wantedState *FillMemActionState
	}{
		{
			name: "Should return config",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "1000",
					"mode":     "usage",
					"size":     "80",
					"unit":     "%",
				},
				ExecutionId: getExecutionId(0),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedState: &FillMemActionState{
				ExecutionId: getExecutionId(0),
				StressOpts: FillMemOpts{
					Duration: time.Second,
					Mode:     FillMemoryModes.Usage,
					Unit:     FillMemoryUnits.Percent,
					Size:     80,
				},
			},
		},
		{
			name: "Should return error too low duration",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "0",
					"mode":     "usage",
					"size":     "80",
					"unit":     "%",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "duration must be greater / equal than 1s",
		},
		{
			name: "Should return error too low duration",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "0",
					"mode":     "usage",
					"size":     "80",
					"unit":     "%",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "duration must be greater / equal than 1s",
		},
		{
			name: "Should return error invalid mode",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "1000",
					"mode":     "newmode",
					"size":     "80",
					"unit":     "%",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: fmt.Sprintf("mode must be one of the following: %s, %s", FillMemoryModes.Absolute, FillMemoryModes.Usage),
		},
		{
			name: "Should return error invalid unit",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "1000",
					"mode":     "usage",
					"size":     "80",
					"unit":     "GB",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: fmt.Sprintf("unit must be one of the following: %s, %s", FillMemoryUnits.Megabyte, FillMemoryUnits.Percent),
		},
		{
			name: "Should return error invalid size",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "1000",
					"mode":     "usage",
					"size":     "0",
					"unit":     "%",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "size must be more than 0",
		},
	}
	action := NewFillMemAction()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//Given
			state := FillMemActionState{}
			request := tt.requestBody
			//When
			result, err := action.Prepare(context.Background(), &state, request)

			//Then
			if tt.wantedError != "" {
				if err != nil {
					assert.EqualError(t, err, tt.wantedError)
				} else if result != nil && result.Error != nil {
					assert.Equal(t, tt.wantedError, result.Error.Title)
				} else {
					assert.Fail(t, "Expected error but no error or result with error was returned")
				}
			}
			if tt.wantedState != nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantedState.StressOpts.Mode, state.StressOpts.Mode)
				assert.Equal(t, tt.wantedState.StressOpts.Unit, state.StressOpts.Unit)
			}
		})
	}
}
