// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package stopprocess

import (
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
