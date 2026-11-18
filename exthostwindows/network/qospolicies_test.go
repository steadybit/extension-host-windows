// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"
)

func TestListSteadybitQosPolicies(t *testing.T) {
	require.NoError(t, removeSteadybitQosPolicies(t.Context()))

	policy := QosPolicy{
		Name:          qosPolicyPrefix + "TEST_" + strconv.Itoa(rand.Intn(100000)),
		BitsPerSecond: 1_000_000,
		DstPrefix:     "1.1.1.1",
		DstPortFrom:   80,
		DstPortTo:     443,
	}
	require.NoError(t, createQosPolicies(t.Context(), []QosPolicy{policy}))
	defer func() {
		require.NoError(t, removeQosPolicies(t.Context(), []string{policy.Name}))
	}()

	policies, err := listSteadybitQosPolicies(t.Context())
	require.NoError(t, err)
	require.Contains(t, policies, policy)
}

func TestRemoveSteadybitQosPolicies(t *testing.T) {
	createTestQosPolicy(t, qosPolicyPrefix+"TEST_LEFTOVER_0")
	createTestQosPolicy(t, qosPolicyPrefix+"TEST_LEFTOVER_1")

	require.NoError(t, removeSteadybitQosPolicies(t.Context()))
	require.Empty(t, qosPolicyNames(t))
}

func TestRemoveQosPolicies_Missing(t *testing.T) {
	require.NoError(t, removeQosPolicies(t.Context(), []string{qosPolicyPrefix + "DOES_NOT_EXIST"}))
}

func TestQosPolicyName_Invalid(t *testing.T) {
	require.Error(t, createQosPolicies(t.Context(), []QosPolicy{{Name: "STEADYBIT_QOS_x' OR '1'='1", BitsPerSecond: 8}}))
	require.Error(t, removeQosPolicies(t.Context(), []string{"STEADYBIT_QOS_x' OR '1'='1"}))
}

func TestCreateQosPolicies_RejectsMissingRate(t *testing.T) {
	err := createQosPolicies(t.Context(), []QosPolicy{{Name: qosPolicyPrefix + "8MB_0", DstPrefix: "1.1.1.1"}})
	require.ErrorContains(t, err, "prepare the attack again")
	require.NotContains(t, qosPolicyNames(t), qosPolicyPrefix+"8MB_0")
}

func createTestQosPolicy(t *testing.T, name string) {
	require.NoError(t, createQosPolicies(t.Context(), []QosPolicy{{Name: name, BitsPerSecond: 100_000_000, DstPrefix: "1.1.1.1"}}))
}

func qosPolicyNames(t *testing.T) []string {
	policies, err := listSteadybitQosPolicies(t.Context())
	require.NoError(t, err)
	names := make([]string, 0, len(policies))
	for _, p := range policies {
		names = append(names, p.Name)
	}
	return names
}
