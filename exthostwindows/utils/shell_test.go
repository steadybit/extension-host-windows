// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_TrimShellOutput(t *testing.T) {
	command, err := ExecutePowershellCommand(t.Context(), []string{"echo 'hello world'"})
	require.NoError(t, err)
	require.Equal(t, "hello world", command)
}
