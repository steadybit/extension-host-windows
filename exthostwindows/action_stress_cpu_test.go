package exthostwindows

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
)

func getExecutionIdByTestId() func(testId uint) uuid.UUID {
	var cache map[uint]uuid.UUID = make(map[uint]uuid.UUID)

	return func(testId uint) uuid.UUID {
		val, ok := cache[testId]

		if ok {
			return val
		}

		uuid := uuid.New()
		cache[testId] = uuid
		return uuid
	}
}

func TestActionStressCpu_Prepare(t *testing.T) {
	getExecutionId := getExecutionIdByTestId()
	cpuNum := runtime.NumCPU()
	osHostname = func() (string, error) {
		return "myhostname", nil
	}
	hostname := "myhostname"

	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError string
		wantedState *CPUStressActionState
	}{
		{
			name: "Should return config",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "1000",
					"cpuLoad":  "80",
					"cores":    "2",
					"priority": "Normal",
				},
				ExecutionId: getExecutionId(0),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedState: &CPUStressActionState{
				ExecutionId: getExecutionId(0),
				StressOpts: CpuStressOpts{
					Cores:    2,
					CpuLoad:  80,
					Priority: ProcessPriorities.Normal,
					Duration: time.Second,
				},
			},
		}, {
			name: "Should return error too low duration",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "0",
					"cpuLoad":  "80",
					"cores":    "2",
					"priority": "Normal",
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
			name: "Should return error too many cores",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "1000",
					"cpuLoad":  "80",
					"cores":    cpuNum + 1,
					"priority": "Normal",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: fmt.Sprintf("number of cores must not be more than maximum available number of cores (%d)", cpuNum),
		},
		{
			name: "Should return error invalid cpu load",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "1000",
					"cpuLoad":  "0",
					"cores":    "2",
					"priority": "Normal",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "cpu load must be in an inclusive range from 1%% to 100%%",
		},
		{
			name: "Should return error invalid cpu load",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "1000",
					"cpuLoad":  "110",
					"cores":    "2",
					"priority": "Normal",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "cpu load must be in an inclusive range from 1%% to 100%%",
		},
		{
			name: "Should return error invalid process priority",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "1000",
					"cpuLoad":  "80",
					"cores":    "2",
					"priority": "Great",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "priority must be one of the following: 'Normal', 'Above Normal', 'High', 'RealTime'.",
		},
	}

	action := NewStressCpuAction()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//Given
			state := CPUStressActionState{}
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
				assert.Equal(t, tt.wantedState.StressOpts.Cores, state.StressOpts.Cores)
				assert.Equal(t, tt.wantedState.StressOpts.CpuLoad, state.StressOpts.CpuLoad)
				assert.Equal(t, tt.wantedState.StressOpts.Priority, state.StressOpts.Priority)
			}
		})
	}
}
