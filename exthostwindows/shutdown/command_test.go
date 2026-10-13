// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package shutdown

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShutdownCommand(t *testing.T) {
	command := NewCommand()
	require.True(t, command.IsShutdownCommandExecutable())
}

func TestShutdownAndRebootCommandLine(t *testing.T) {
	command := &CommandImpl{}

	shutdownCmd := command.getShutdownCommand()
	require.Len(t, shutdownCmd, 4)
	require.Equal(t, []string{"/s", "/t", "0"}, shutdownCmd[1:])
	require.True(t, strings.EqualFold(filepath.Base(shutdownCmd[0]), shutdownExecutableName))

	rebootCmd := command.getRebootCommand()
	require.Len(t, rebootCmd, 4)
	require.Equal(t, []string{"/r", "/t", "0"}, rebootCmd[1:])
	require.Equal(t, shutdownCmd[0], rebootCmd[0])
}

func TestShutdownCommandNotAvailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	command := &CommandImpl{}
	require.False(t, command.IsShutdownCommandExecutable())
	require.Nil(t, command.getShutdownCommand())
	require.Nil(t, command.getRebootCommand())
}
