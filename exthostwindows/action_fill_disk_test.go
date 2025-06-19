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

func TestActionFillDisk_Prepare(t *testing.T) {
	getExecutionId := getExecutionIdByTestId()
	osHostname = func() (string, error) {
		return "myhostname", nil
	}
	hostname := "myhostname"
	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError string
		wantedState *FillDiskActionState
	}{
		{
			name: "Should return config",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      "C:\\",
					"method":    "AT_ONCE",
					"blocksize": "5",
				},
				ExecutionId: getExecutionId(0),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedState: &FillDiskActionState{
				ExecutionId: getExecutionId(0),
				StressOpts: FillDiskOpts{
					Duration:  time.Second,
					FillMode:  FillDiskModes.Percentage,
					Method:    FillDiskMethods.AtOnce,
					Path:      "C:\\",
					BlockSize: 5,
				},
			},
		},
		{
			name: "Should return error too low duration",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "0",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      "C:\\",
					"method":    "AT_ONCE",
					"blocksize": "5",
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
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "RANDOM",
					"size":      "80",
					"path":      "C:\\",
					"method":    "AT_ONCE",
					"blocksize": "5",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: fmt.Sprintf("mode must be one of the following: %s, %s, %s", FillDiskModes.MBToFill, FillDiskModes.MBLeft, FillDiskModes.Percentage),
		},
		{
			name: "Should return error invalid unit",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      "C:\\",
					"method":    "RANDOM",
					"blocksize": "5",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: fmt.Sprintf("unit must be one of the following: %s, %s", FillDiskMethods.AtOnce, FillDiskMethods.OverTime),
		},
		{
			name: "Should return error path must not be empty",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      "",
					"method":    "AT_ONCE",
					"blocksize": "5",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "path must not be empty",
		},
		{
			name: "Should return error path must not be empty",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      ".\\somewhere",
					"method":    "AT_ONCE",
					"blocksize": "5",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "path must be absolute and start with a drive letter, given: .\\somewhere",
		},
		{
			name: "Should return error blocksize must be greater or equal to 1",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      "C:\\",
					"method":    "AT_ONCE",
					"blocksize": "0",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "blocksize must be at least 1 and lesser or equal to 1024",
		},
		{
			name: "Should return error blocksize must be lesser or equal to 1024",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      "C:\\",
					"method":    "AT_ONCE",
					"blocksize": "0",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"host.hostname": {hostname},
					},
				}),
			},

			wantedError: "blocksize must be at least 1 and lesser or equal to 1024",
		},
	}
	action := NewFillDiskAction()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//Given
			state := FillDiskActionState{}
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
				assert.Equal(t, tt.wantedState.StressOpts.BlockSize, state.StressOpts.BlockSize)
				assert.Equal(t, tt.wantedState.StressOpts.FillMode, state.StressOpts.FillMode)
				assert.Equal(t, tt.wantedState.StressOpts.Method, state.StressOpts.Method)
				assert.Equal(t, tt.wantedState.StressOpts.Path, state.StressOpts.Path)
			}
		})
	}
}
