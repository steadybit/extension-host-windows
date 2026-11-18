// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"testing"
	"time"

	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupQosPolicies(t *testing.T) {
	// Cleanup works if no leftover policies exist
	CleanupQosPolicies()

	// Cleanup skips the listing while no attack changed QoS policies
	assert.True(t, qosPoliciesClean.Load())

	// Cleanup does not run if experiment is active
	createTestQosPolicy(t, qosPolicyPrefix+"TEST_POLICY_CLEANUP")
	qosPoliciesClean.Store(false) // as if a failed revert left the policy behind
	opts := &MockNetworkOpt{}
	err := generateAndRunCommands(t.Context(), opts, ModeAdd)
	require.NoError(t, err)

	CleanupQosPolicies()

	policies := qosPolicyNames(t)
	assert.NotEmpty(t, policies)

	// Cleanup removes leftover policies after the experiment has finished
	err = generateAndRunCommands(t.Context(), opts, ModeDelete)
	require.NoError(t, err)

	CleanupQosPolicies()

	policies = qosPolicyNames(t)
	assert.Empty(t, policies)
	assert.True(t, qosPoliciesClean.Load())
}

type MockNetworkOpt struct{}

func (o *MockNetworkOpt) WinDivert() *WinDivertOpts {
	return nil
}

func (o *MockNetworkOpt) QoSPolicies() []QosPolicy {
	return nil
}

func (o *MockNetworkOpt) String() string {
	return "MockNetworkOpt"
}

func TestFailedStartLeavesNoActiveAttack(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // wdna.exe cannot be found
	opts := &DelayOpts{Filter: Filter{Include: akn.NewNetWithPortRanges(akn.NetAny, akn.PortRangeAny)}, Delay: 100 * time.Millisecond, Duration: 10 * time.Second}

	err := Apply(t.Context(), opts)
	require.ErrorContains(t, err, "wdna.exe")
	assert.False(t, activeFw())

	// Not rejected as a second attack running at the same time.
	err = Apply(t.Context(), opts)
	require.ErrorContains(t, err, "wdna.exe")
	assert.NotContains(t, err.Error(), "at the same time")

	require.NoError(t, Revert(t.Context(), opts))
}

func TestFailedStartRemovesCreatedQosPolicies(t *testing.T) {
	opts := &qosOpts{policies: []QosPolicy{
		{Name: qosPolicyPrefix + "TEST_FAILED_START_1", BitsPerSecond: 8_000_000, DstPrefix: "192.0.2.1/32"},
		{Name: qosPolicyPrefix + "TEST_FAILED_START_2", BitsPerSecond: 8_000_000, DstPrefix: "not-an-address"},
	}}

	err := Apply(t.Context(), opts)
	require.ErrorContains(t, err, "TEST_FAILED_START_2")
	assert.Empty(t, qosPolicyNames(t))
	assert.False(t, activeFw())
}

type qosOpts struct {
	policies []QosPolicy
}

func (o *qosOpts) WinDivert() *WinDivertOpts { return nil }
func (o *qosOpts) QoSPolicies() []QosPolicy  { return o.policies }
func (o *qosOpts) String() string            { return "qosOpts" }
