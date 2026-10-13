// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package stopprocess

import (
	"errors"

	"github.com/mitchellh/go-ps"
	"github.com/stretchr/testify/require"
	"os/exec"
	"testing"
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
