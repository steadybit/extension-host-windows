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

func TestActionStressIO_Prepare(t *testing.T) {
	getExecutionId := getExecutionIdByTestId()
	osHostname = func() (string, error) {
		return "myhostname", nil
	}
	hostname := "myhostname"

	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError string
		wantedState *IoStressActionState
	}{
		{
			name: "Should return config",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":             "prepare",
					"duration":           "1000",
					"stressLayer":        "Named Partition",
					"stressLayerInput":   "C",
					"threadCount":        "1",
					"disableSwHwCaching": "true",
				},
				ExecutionId: getExecutionId(0),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedState: &IoStressActionState{
				ExecutionId: getExecutionId(0),
				StressOpts: IoStressOpts{
					StressLayer:        IOStressLayers.NamedPartition,
					StressLayerInput:   "C",
					ThreadCount:        1,
					DisableSwHwCaching: true,
					Duration:           time.Second,
				},
			},
		},
		{
			name: "Should return error too low duration",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":             "prepare",
					"duration":           "0",
					"stressLayer":        "Named Partition",
					"stressLayerInput":   "C",
					"threadCount":        "1",
					"disableSwHwCaching": "true",
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
			name: "Should return error invalid stress layer",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":             "prepare",
					"duration":           "1000",
					"stressLayer":        "Layer",
					"stressLayerInput":   "C",
					"threadCount":        "1",
					"disableSwHwCaching": "true",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: fmt.Sprintf("stress layer must be one of the following: %s, %s, %s, current: %s", IOStressLayers.FileSystem, IOStressLayers.NamedPartition, IOStressLayers.PhysicalDisk, "Layer"),
		},
		{
			name: "Should return error invalid disk letter",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":             "prepare",
					"duration":           "1000",
					"stressLayer":        "Named Partition",
					"stressLayerInput":   "MULTIPLE",
					"threadCount":        "1",
					"disableSwHwCaching": "true",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "disk letter must be a letter from A-Z",
		},
		{
			name: "Should return error invalid disk letter",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":             "prepare",
					"duration":           "1000",
					"stressLayer":        "Named Partition",
					"stressLayerInput":   "&",
					"threadCount":        "1",
					"disableSwHwCaching": "true",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "disk letter must be a letter from A-Z",
		},
		{
			name: "Should return error invalid number of threads",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":             "prepare",
					"duration":           "1000",
					"stressLayer":        "Named Partition",
					"stressLayerInput":   "C",
					"threadCount":        0,
					"disableSwHwCaching": "true",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "number of threads must be greater than 0",
		},
	}

	action := NewStressIoAction()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//Given
			state := IoStressActionState{}
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
				assert.Equal(t, tt.wantedState.StressOpts.StressLayer, state.StressOpts.StressLayer)
				assert.Equal(t, tt.wantedState.StressOpts.StressLayerInput, state.StressOpts.StressLayerInput)
				assert.Equal(t, tt.wantedState.StressOpts.ThreadCount, state.StressOpts.ThreadCount)
				assert.Equal(t, tt.wantedState.StressOpts.DisableSwHwCaching, state.StressOpts.DisableSwHwCaching)
			}
		})
	}
}
