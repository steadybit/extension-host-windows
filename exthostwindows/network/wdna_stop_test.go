// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package network

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// fakeWdnaEnv makes the test binary act as wdna: with "stop" it creates its stop event like wdna does and exits once it
// is set, with "ignore" it keeps running until it is killed.
const fakeWdnaEnv = "STEADYBIT_TEST_FAKE_WDNA"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeWdnaEnv); mode != "" {
		os.Exit(runFakeWdna(mode))
	}
	os.Exit(m.Run())
}

func TestStopWdnaSetsTheStopEvent(t *testing.T) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Skip(`creating a Global\ event like wdna needs an elevated process`)
	}
	executionId := uuid.New()
	cmd := startFakeWdna(t, executionId, "stop")

	require.NoError(t, stopWdna(executionId, wdnaStopTimeout))

	require.NoError(t, cmd.Wait(), "wdna must exit on its own, not be terminated")
}

func TestStopWdnaTerminatesWdnaThatDoesNotStop(t *testing.T) {
	executionId := uuid.New()
	cmd := startFakeWdna(t, executionId, "ignore")

	start := time.Now()
	require.NoError(t, stopWdna(executionId, 500*time.Millisecond))

	assert.Error(t, cmd.Wait(), "wdna must be terminated")
	assert.GreaterOrEqual(t, time.Since(start), 500*time.Millisecond)
}

func TestStopWdnaWithoutWdna(t *testing.T) {
	require.NoError(t, stopWdna(uuid.New(), wdnaStopTimeout))
}

// startFakeWdna runs the test binary as the wdna of an attack and waits until it is ready.
func startFakeWdna(t *testing.T, executionId uuid.UUID, mode string) *exec.Cmd {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.Command(self, "-test.run=^$")
	cmd.Env = append(os.Environ(), fakeWdnaEnv+"="+mode)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, utils.StartAttackProcess(executionId, cmd))
	t.Cleanup(func() {
		_ = stopWdna(executionId, 0)
		_ = cmd.Wait()
	})

	line, _ := bufio.NewReader(stdout).ReadString('\n')
	require.Equal(t, "ready\n", line)
	return cmd
}

func runFakeWdna(mode string) int {
	if mode == "ignore" {
		_, _ = os.Stdout.WriteString("ready\n")
		time.Sleep(time.Hour)
		return 0
	}

	name, _ := windows.UTF16PtrFromString(fmt.Sprintf(wdnaStopEventFormat, os.Getpid()))
	event, err := windows.CreateEvent(nil, 1, 0, name)
	if err != nil {
		println("failed to create the stop event:", err.Error())
		return 2
	}
	_, _ = os.Stdout.WriteString("ready\n")
	if _, err := windows.WaitForSingleObject(event, windows.INFINITE); err != nil {
		println("failed to wait for the stop event:", err.Error())
		return 2
	}
	return 0
}
