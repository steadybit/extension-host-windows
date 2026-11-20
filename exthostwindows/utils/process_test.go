// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import (
	"errors"

	"github.com/mitchellh/go-ps"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestStopProcesses(t *testing.T) {
	cmd := exec.Command("ping", "-n", "30", "127.0.0.1")
	err := cmd.Start()
	require.NoError(t, err)

	ids := FindProcessIds("PING")
	require.Len(t, ids, 1)
	require.Equal(t, cmd.Process.Pid, ids[0])

	err = StopProcesses(ids, true)
	require.NoError(t, err)

	p, err := ps.FindProcess(cmd.Process.Pid)
	require.NoError(t, err)
	require.Nil(t, p)
}

func TestStopProcesses_NoPids(t *testing.T) {
	require.NoError(t, StopProcesses(nil, true))
	require.NoError(t, StopProcesses([]int{}, false))
}

func TestStopProcesses_NonExistingProcess(t *testing.T) {
	// pids on Windows are multiples of 4, so this pid can never exist
	require.NoError(t, StopProcesses([]int{999_999_997}, false))
}

func TestStopProcesses_Graceful(t *testing.T) {
	cmd := exec.Command("ping", "-n", "30", "127.0.0.1")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	// console processes without a window can't be closed gracefully, taskkill reports an error
	err := StopProcesses([]int{cmd.Process.Pid}, false)
	if err != nil {
		require.Contains(t, err.Error(), "failed to kill process")
	}
}

func TestFindProcessIds_ByPid(t *testing.T) {
	require.Equal(t, []int{1234}, FindProcessIds("1234"))
}

func TestFindProcessIds_NoMatch(t *testing.T) {
	require.Empty(t, FindProcessIds("steadybit-process-which-does-not-exist"))
}

func TestIsProcessNotFound(t *testing.T) {
	require.False(t, isProcessNotFound(errors.New("some error")))

	err := exec.Command("cmd", "/c", "exit 128").Run()
	require.True(t, isProcessNotFound(err))

	err = exec.Command("cmd", "/c", "exit 1").Run()
	require.False(t, isProcessNotFound(err))
}

func TestMain(m *testing.M) {
	switch os.Getenv("STOP_PROCESSES_HELPER") {
	case "parent":
		// like `caddy start`: wait for the child and exit with it. Freeing the touched memory keeps the parent exiting
		// long enough to be terminated in the meantime.
		memory := make([]byte, 512<<20)
		for i := 0; i < len(memory); i += 4096 {
			memory[i] = 1
		}
		_ = helperCommand("child").Run()
		runtime.KeepAlive(memory)
		os.Exit(0)
	case "child":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// helperCommand runs this test binary as the given TestMain helper process.
func helperCommand(role string) *exec.Cmd {
	self, _ := os.Executable()
	cmd := exec.Command(self, "-test.run=^$")
	cmd.Env = append(os.Environ(), "STOP_PROCESSES_HELPER="+role)
	return cmd
}

func TestStopProcesses_ParentExitingAfterChild(t *testing.T) {
	for i := 0; i < 20; i++ {
		parent := helperCommand("parent")
		require.NoError(t, parent.Start())
		t.Cleanup(func() { _ = parent.Process.Kill(); _, _ = parent.Process.Wait() })

		var child int
		require.Eventually(t, func() bool {
			processes, err := ps.Processes()
			require.NoError(t, err)
			for _, p := range processes {
				if p.PPid() == parent.Process.Pid {
					child = p.Pid()
					return true
				}
			}
			return false
		}, 5*time.Second, 10*time.Millisecond)

		// the parent starts exiting as soon as the child is killed, terminating it then fails with access denied
		require.NoError(t, StopProcesses([]int{child, parent.Process.Pid}, true))
	}
}
