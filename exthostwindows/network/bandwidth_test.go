// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

func Test_LimitBandwidth_create_one_policy_per_ip(t *testing.T) {
	err := removeSteadybitQosPolicies(t.Context())
	require.NoError(t, err)

	expectedPolicyName1 := "STEADYBIT_QOS_100MB_0"
	_, ipNet1, _ := net.ParseCIDR("1.1.1.1/32")

	expectedPolicyName2 := "STEADYBIT_QOS_100MB_1"
	_, ipNet2, _ := net.ParseCIDR("1.1.1.2/32")

	limitBandwidthOpts := LimitBandwidthOpts{
		Bandwidth: "100MB",
		IncludeCidrs: []net.IPNet{
			*ipNet1, *ipNet2,
		},
		PortRange: network.PortRange{
			From: 9876,
			To:   9876,
		},
	}

	err = Apply(t.Context(), &limitBandwidthOpts)
	require.NoError(t, err)

	defer func() {
		err := Revert(t.Context(), &limitBandwidthOpts)
		require.NoError(t, err)
		policies, err := listSteadybitQosPolicyNames(t.Context())
		require.NoError(t, err)
		require.NotContains(t, policies, expectedPolicyName1)
		require.NotContains(t, policies, expectedPolicyName2)
	}()

	policies, err := listSteadybitQosPolicyNames(t.Context())
	require.NoError(t, err)
	require.Contains(t, policies, expectedPolicyName1)
	require.Contains(t, policies, expectedPolicyName2)
}
