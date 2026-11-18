// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

func TestActivateWindowsServiceHandler_NotInService(t *testing.T) {
	stopped := false
	// tests are not executed as Windows service, so this must be a no-op
	ActivateWindowsServiceHandler(func() { stopped = true })
	assert.False(t, stopped)
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatal("nothing received")
		var zero T
		return zero
	}
}

// startExecute runs Execute until it reported RUNNING, or only START_PENDING if running is false.
func startExecute(t *testing.T, stopHandler func(), running bool) (*extensionService, chan svc.ChangeRequest, chan svc.Status, <-chan uint32) {
	s := &extensionService{stopHandler: stopHandler, ready: make(chan struct{}), failed: make(chan struct{})}
	changeRequests := make(chan svc.ChangeRequest)
	statuses := make(chan svc.Status, 1)
	done := make(chan uint32, 1)
	go func() {
		ssec, errno := s.Execute(nil, changeRequests, statuses)
		assert.False(t, ssec)
		done <- errno
	}()
	assert.Equal(t, svc.Status{State: svc.StartPending, WaitHint: waitHint}, receive(t, statuses))
	if running {
		close(s.ready)
		assert.Equal(t, svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}, receive(t, statuses))
	}
	return s, changeRequests, statuses, done
}

func TestExtensionService_Execute(t *testing.T) {
	s, changeRequests, statuses, _ := startExecute(t, func() { t.Error("stop handler must not be called") }, false)

	pending := svc.Status{State: svc.StartPending, CheckPoint: 42}
	changeRequests <- svc.ChangeRequest{Cmd: svc.Interrogate, CurrentStatus: pending}
	assert.Equal(t, pending, receive(t, statuses))

	close(s.ready)
	assert.Equal(t, svc.Running, receive(t, statuses).State)

	// unexpected requests are only logged: the next status is the Interrogate answer
	changeRequests <- svc.ChangeRequest{Cmd: svc.Pause}
	running := svc.Status{State: svc.Running, CheckPoint: 43}
	changeRequests <- svc.ChangeRequest{Cmd: svc.Interrogate, CurrentStatus: running}
	assert.Equal(t, running, receive(t, statuses))
}

func TestExtensionService_Execute_Stop(t *testing.T) {
	for name, cmd := range map[string]svc.Cmd{"stop": svc.Stop, "shutdown": svc.Shutdown} {
		t.Run(name, func(t *testing.T) {
			stopHandlerCalled := false
			_, changeRequests, statuses, done := startExecute(t, func() { stopHandlerCalled = true }, true)

			changeRequests <- svc.ChangeRequest{Cmd: cmd}

			assert.Equal(t, svc.StopPending, receive(t, statuses).State)
			assert.Zero(t, receive(t, done))
			assert.True(t, stopHandlerCalled)
		})
	}
}

func TestExtensionService_Execute_Failed(t *testing.T) {
	for name, running := range map[string]bool{"starting": false, "running": true} {
		t.Run(name, func(t *testing.T) {
			s, _, statuses, done := startExecute(t, func() { t.Error("stop handler must not be called") }, running)

			close(s.failed)

			assert.Equal(t, fatalExitCode, receive(t, done))
			assert.Empty(t, statuses)
		})
	}
}

func TestEventLogWriter(t *testing.T) {
	elog, err := eventlog.Open(serviceName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = elog.Close() })
	w := &eventLogWriter{log: elog}

	for _, line := range []string{
		`{"level":"info","message":"info message"}`,
		`{"level":"warn","message":"warn message"}`,
		`{"level":"error","message":"error message"}`,
		`{"level":"debug","message":"debug message"}`,
		`{"level":"","message":"no level"}`,
		`{"level":"warn"}`,
		`{"message":"no level field"}`,
	} {
		t.Run(line, func(t *testing.T) {
			_, err := w.Write([]byte(line))
			assert.NoError(t, err)
		})
	}

	t.Run("invalid json", func(t *testing.T) {
		_, err := w.Write([]byte("not json"))
		assert.Error(t, err)
	})
}

func TestEventLogWriter_mapLevel_Disabled(t *testing.T) {
	w := &eventLogWriter{}
	for _, level := range []string{"", "disabled"} {
		assert.NoError(t, w.mapLevel(level)(1, "dropped"), "level %q", level)
	}
	for _, level := range []string{"trace", "debug", "info", "warn", "error", "fatal", "panic", "unknown"} {
		assert.NotNil(t, w.mapLevel(level), "level %q", level)
	}
}
