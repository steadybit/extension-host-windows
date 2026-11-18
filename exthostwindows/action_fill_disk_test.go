package exthostwindows

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
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
				Config: map[string]any{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      "C:\\",
					"method":    "AT_ONCE",
					"blocksize": "5",
				},
				ExecutionId: getExecutionId(0),
				Target: new(action_kit_api.Target{
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
				Config: map[string]any{
					"action":    "prepare",
					"duration":  "0",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      "C:\\",
					"method":    "AT_ONCE",
					"blocksize": "5",
				},
				ExecutionId: uuid.New(),
				Target: new(action_kit_api.Target{
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
				Config: map[string]any{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "RANDOM",
					"size":      "80",
					"path":      "C:\\",
					"method":    "AT_ONCE",
					"blocksize": "5",
				},
				ExecutionId: uuid.New(),
				Target: new(action_kit_api.Target{
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
				Config: map[string]any{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      "C:\\",
					"method":    "RANDOM",
					"blocksize": "5",
				},
				ExecutionId: uuid.New(),
				Target: new(action_kit_api.Target{
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
				Config: map[string]any{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      "",
					"method":    "AT_ONCE",
					"blocksize": "5",
				},
				ExecutionId: uuid.New(),
				Target: new(action_kit_api.Target{
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
				Config: map[string]any{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      ".\\somewhere",
					"method":    "AT_ONCE",
					"blocksize": "5",
				},
				ExecutionId: uuid.New(),
				Target: new(action_kit_api.Target{
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
				Config: map[string]any{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      "C:\\",
					"method":    "AT_ONCE",
					"blocksize": "0",
				},
				ExecutionId: uuid.New(),
				Target: new(action_kit_api.Target{
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
				Config: map[string]any{
					"action":    "prepare",
					"duration":  "1000",
					"mode":      "PERCENTAGE",
					"size":      "80",
					"path":      "C:\\",
					"method":    "AT_ONCE",
					"blocksize": "0",
				},
				ExecutionId: uuid.New(),
				Target: new(action_kit_api.Target{
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

func TestPipeDevzeroIntoDd(t *testing.T) {
	// ping stands in for devzero: it writes output for about 100 s unless it is killed.
	longRunning := func() *exec.Cmd { return exec.Command("ping", "-n", "100", "127.0.0.1") }

	tests := []struct {
		name    string
		devzero *exec.Cmd
		dd      *exec.Cmd
	}{
		{name: "devzero does not start", devzero: exec.Command("no-such-devzero"), dd: exec.Command("cmd.exe", "/c", "exit 0")},
		{name: "dd does not start", devzero: longRunning(), dd: exec.Command("no-such-dd")},
		{name: "dd ends without reading", devzero: longRunning(), dd: exec.Command("cmd.exe", "/c", "exit 0")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				pipeDevzeroIntoDd(tt.devzero, tt.dd)
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("pipeDevzeroIntoDd did not return")
			}
			if tt.devzero.Process != nil {
				assert.NotNil(t, tt.devzero.ProcessState, "devzero is still running")
			} else {
				assert.Nil(t, tt.dd.Process, "dd started without devzero")
			}
		})
	}
}

func TestActionFillDisk_Stop_afterFailedStart(t *testing.T) {
	state := FillDiskActionState{StressOpts: FillDiskOpts{Method: OverTime, FilePath: filepath.Join(t.TempDir(), "never-created")}}
	_, err := (&fillDiskAction{}).Stop(context.Background(), &state)
	assert.NoError(t, err)
}
