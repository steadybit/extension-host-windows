// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ResolveHostnames(t *testing.T) {
	ips, err := Resolve(t.Context(), "localhost")
	require.NoError(t, err)
	require.NotZero(t, len(ips))
}

func Test_ResolveHostnames_Unresolvable(t *testing.T) {
	ips, err := Resolve(t.Context(), "localhost", "does-not-exist.invalid")
	require.EqualError(t, err, "could not resolve hostnames: 'does-not-exist.invalid'")
	require.Nil(t, ips)
}
