// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/stretchr/testify/require"
)

func Test_LimitBandwidth_create_one_policy_per_ip(t *testing.T) {
	err := removeSteadybitQosPolicies(t.Context())
	require.NoError(t, err)

	execution := Execution{Id: uuid.MustParse("8f8b7c4e-1d2a-4b6c-9e0f-123456789abc")}
	expectedPolicyName1 := "STEADYBIT_QOS_100MB_0"
	_, ipNet1, _ := net.ParseCIDR("1.1.1.1/32")

	expectedPolicyName2 := "STEADYBIT_QOS_100MB_1"
	_, ipNet2, _ := net.ParseCIDR("1.1.1.2/32")

	limitBandwidthOpts := LimitBandwidthOpts{
		Bandwidth:     "100MB",
		BitsPerSecond: 100_000_000,
		IncludeCidrs: []net.IPNet{
			*ipNet1, *ipNet2,
		},
		PortRange: network.PortRange{
			From: 9876,
			To:   9876,
		},
	}

	err = Apply(t.Context(), execution, &limitBandwidthOpts)
	require.NoError(t, err)

	defer func() {
		err := Revert(t.Context(), execution, &limitBandwidthOpts)
		require.NoError(t, err)
		policies := qosPolicyNames(t)
		require.NotContains(t, policies, expectedPolicyName1)
		require.NotContains(t, policies, expectedPolicyName2)
	}()

	policies, err := listSteadybitQosPolicies(t.Context())
	require.NoError(t, err)
	require.Contains(t, policies, QosPolicy{Name: expectedPolicyName1, BitsPerSecond: 100_000_000, DstPrefix: "1.1.1.1", DstPortFrom: 9876, DstPortTo: 9876})
	require.Contains(t, policies, QosPolicy{Name: expectedPolicyName2, BitsPerSecond: 100_000_000, DstPrefix: "1.1.1.2", DstPortFrom: 9876, DstPortTo: 9876})
}
