// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package network

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// fakeWdnaEnv makes the test binary act as wdna.exe: "mailslot" exits once a shutdown request arrives on wdna's
// mailslot, "ignore" keeps running until it is killed.
const fakeWdnaEnv = "STEADYBIT_TEST_FAKE_WDNA"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeWdnaEnv); mode != "" {
		os.Exit(runFakeWdna(mode == "mailslot"))
	}
	os.Exit(m.Run())
}

func TestStopWdnaSignalsShutdown(t *testing.T) {
	cmd := startFakeWdna(t, "mailslot")

	start := time.Now()
	require.NoError(t, stopWdna(wdnaStopTimeout))

	require.NoError(t, cmd.Wait(), "wdna must exit on its own, not be killed")
	assert.Less(t, time.Since(start), wdnaStopTimeout)
}

func TestStopWdnaKillsWdnaThatDoesNotShutDown(t *testing.T) {
	cmd := startFakeWdna(t, "ignore")

	start := time.Now()
	require.NoError(t, stopWdna(500*time.Millisecond))

	assert.Error(t, cmd.Wait(), "wdna must be killed")
	assert.GreaterOrEqual(t, time.Since(start), 500*time.Millisecond)
}

func TestStopWdnaWithoutWdna(t *testing.T) {
	require.NoError(t, stopWdna(wdnaStopTimeout))
}

// startFakeWdna runs a copy of the test binary named wdna.exe, as stopWdna finds wdna by name, and waits until it
// listens on the mailslot.
func startFakeWdna(t *testing.T, mode string) *exec.Cmd {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	binary, err := os.ReadFile(self)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "wdna.exe")
	require.NoError(t, os.WriteFile(path, binary, 0o755))

	cmd := exec.Command(path)
	cmd.Env = append(os.Environ(), fakeWdnaEnv+"="+mode)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	line, _ := bufio.NewReader(stdout).ReadString('\n')
	require.Equal(t, "ready\n", line)
	return cmd
}

var procCreateMailslot = windows.NewLazySystemDLL("kernel32.dll").NewProc("CreateMailslotW")

func runFakeWdna(listen bool) int {
	if !listen {
		_, _ = os.Stdout.WriteString("ready\n")
		time.Sleep(time.Hour)
		return 0
	}

	name, _ := windows.UTF16PtrFromString(wdnaMailslot)
	slot, _, err := procCreateMailslot.Call(uintptr(unsafe.Pointer(name)), 0, windows.INFINITE, 0)
	if windows.Handle(slot) == windows.InvalidHandle {
		println("failed to create the mailslot:", err.Error())
		return 2
	}
	_, _ = os.Stdout.WriteString("ready\n")
	var read uint32
	if err := windows.ReadFile(windows.Handle(slot), make([]byte, 64), &read, nil); err != nil {
		println("failed to read the mailslot:", err.Error())
		return 2
	}
	return 0
}
