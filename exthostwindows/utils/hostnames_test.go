// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ResolveHostnames(t *testing.T) {
	ips, err := Resolve(t.Context(), "localhost")
	require.NoError(t, err)
	require.Contains(t, ips, net.IPv4(127, 0, 0, 1).To4())
	require.Contains(t, ips, net.IPv6loopback)
}

func Test_ResolveHostnames_Unresolvable(t *testing.T) {
	ips, err := Resolve(t.Context(), "localhost", "does-not-exist.invalid")
	require.EqualError(t, err, "could not resolve hostnames: 'does-not-exist.invalid'")
	require.Nil(t, ips)
}
