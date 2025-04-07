// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package shutdown

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestShutdownCommand(t *testing.T) {
	command := NewCommand()
	require.True(t, command.IsShutdownCommandExecutable())
}
