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

func receiveStatus(t *testing.T, statuses <-chan svc.Status) svc.Status {
	t.Helper()
	select {
	case status := <-statuses:
		return status
	case <-time.After(5 * time.Second):
		t.Fatal("no service status received")
		return svc.Status{}
	}
}

func TestExtensionService_Execute(t *testing.T) {
	s := &extensionService{stopHandler: func() { t.Error("stop handler must not be called") }}
	changeRequests := make(chan svc.ChangeRequest)
	statuses := make(chan svc.Status, 1)

	// Execute never returns for non-stop requests (stop terminates the process via log.Fatal)
	go s.Execute(nil, changeRequests, statuses)

	assert.Equal(t, svc.StartPending, receiveStatus(t, statuses).State)
	running := receiveStatus(t, statuses)
	assert.Equal(t, svc.Running, running.State)
	assert.Equal(t, svc.AcceptStop|svc.AcceptShutdown, running.Accepts)

	current := svc.Status{State: svc.Running, Accepts: svc.AcceptStop, CheckPoint: 42}
	changeRequests <- svc.ChangeRequest{Cmd: svc.Interrogate, CurrentStatus: current}
	assert.Equal(t, current, receiveStatus(t, statuses))

	// unexpected requests are only logged
	changeRequests <- svc.ChangeRequest{Cmd: svc.Pause}
	select {
	case status := <-statuses:
		t.Fatalf("unexpected status %+v", status)
	case <-time.After(100 * time.Millisecond):
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
