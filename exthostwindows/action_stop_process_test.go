// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionStopProcess_Prepare(t *testing.T) {

	osHostname = func() (string, error) {
		return "myhostname", nil
	}

	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError string
		wantedState *StopProcessActionState
	}{
		{
			name: "Should return config",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "10000",
					"delay":    "1000",
					"graceful": "true",
					"process":  "tail",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						hostNameAttribute: {"myhostname"},
					},
				}),
			},

			wantedState: &StopProcessActionState{
				ProcessFilter: "tail",
				Graceful:      true,
				Duration:      10 * time.Second,
				Delay:         1 * time.Second,
			},
		}, {
			name: "Should return error too low duration",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":   "prepare",
					"duration": "0",
					"delay":    "1000",
					"graceful": "true",
					"process":  "tail",
				},
				ExecutionId: uuid.New(),
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						hostNameAttribute: {"myhostname"},
					},
				}),
			},

			wantedError: "Duration is required",
		},
	}
	action := NewStopProcessAction()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//Given
			state := StopProcessActionState{}
			request := tt.requestBody
			now := time.Now()

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
				assert.Equal(t, tt.wantedState.ProcessFilter, state.ProcessFilter)
				assert.Equal(t, tt.wantedState.Graceful, state.Graceful)
				assert.Equal(t, tt.wantedState.Delay, state.Delay)
				deadline := now.Add(state.Duration * time.Second)
				assert.GreaterOrEqual(t, deadline.Unix(), state.Deadline.Unix())
			}
		})
	}
}

func TestActionStopProcess_StatusReportsError(t *testing.T) {
	action := &stopProcessAction{}
	executionID := uuid.New()
	state := &StopProcessActionState{ExecutionID: executionID}

	stopper := &processStopper{
		cancel: func() {},
		err:    atomic.Pointer[error]{},
	}
	e := errors.New("failed to stop process")
	stopper.err.Store(&e)
	action.processStoppers.Store(executionID, stopper)

	result, err := action.Status(context.Background(), state)
	require.NoError(t, err)
	assert.True(t, result.Completed)
	require.NotNil(t, result.Error)
	assert.Equal(t, "failed to stop process", result.Error.Title)
	assert.Equal(t, action_kit_api.Failed, *result.Error.Status)
}

func TestActionStopProcess_StatusReportsNotCompleted(t *testing.T) {
	action := &stopProcessAction{}
	executionID := uuid.New()
	state := &StopProcessActionState{ExecutionID: executionID}

	stopper := &processStopper{
		cancel: func() {},
	}
	action.processStoppers.Store(executionID, stopper)

	result, err := action.Status(context.Background(), state)
	require.NoError(t, err)
	assert.False(t, result.Completed)
	assert.Nil(t, result.Error)
}

func TestActionStopProcess_StatusCompletedWhenNotFound(t *testing.T) {
	action := &stopProcessAction{}
	state := &StopProcessActionState{ExecutionID: uuid.New()}

	result, err := action.Status(context.Background(), state)
	require.NoError(t, err)
	assert.True(t, result.Completed)
	assert.Nil(t, result.Error)
}

func TestActionStopProcess_StopCancelsAndRemovesStopper(t *testing.T) {
	action := &stopProcessAction{}
	executionID := uuid.New()
	state := &StopProcessActionState{ExecutionID: executionID}

	cancelled := false
	stopper := &processStopper{
		cancel: func() { cancelled = true },
	}
	action.processStoppers.Store(executionID, stopper)

	result, err := action.Stop(context.Background(), state)
	require.NoError(t, err)
	assert.Nil(t, result)
	assert.True(t, cancelled)

	_, loaded := action.processStoppers.Load(executionID)
	assert.False(t, loaded)
}
