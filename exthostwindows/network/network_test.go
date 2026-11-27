// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
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
	execution := Execution{Id: uuid.New()}
	opts := &MockNetworkOpt{}
	require.NoError(t, Apply(t.Context(), execution, opts))

	CleanupQosPolicies()

	policies := qosPolicyNames(t)
	assert.NotEmpty(t, policies)

	// Cleanup removes leftover policies after the experiment has finished
	require.NoError(t, Revert(t.Context(), execution, opts))

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
	execution := Execution{Id: uuid.New()}
	opts := &DelayOpts{Filter: Filter{Include: akn.NewNetWithPortRanges(akn.NetAny, akn.PortRangeAny)}, Delay: 100 * time.Millisecond, Duration: 10 * time.Second}

	err := Apply(t.Context(), execution, opts)
	require.ErrorContains(t, err, "wdna.exe")
	assert.Nil(t, active)
	assert.NoFileExists(t, winDivertFilterFile(execution.Id))

	require.NoError(t, Revert(t.Context(), execution, opts))
}

func TestFailedStartRemovesCreatedQosPolicies(t *testing.T) {
	opts := bandwidthOpts("192.0.2.1/32")
	opts.IncludeCidrs = append(opts.IncludeCidrs, net.IPNet{}) // not an address, its policy fails

	execution := Execution{Id: uuid.New()}
	err := Apply(t.Context(), execution, opts)
	require.ErrorContains(t, err, opts.QoSPolicies()[1].Name)
	assert.Empty(t, qosPolicyNames(t))
	assert.Nil(t, active)
}

func TestSecondAttackIsRejected(t *testing.T) {
	for name, second := range map[string]*LimitBandwidthOpts{
		"identical": bandwidthOpts("192.0.2.1/32"),
		"different": bandwidthOpts("192.0.2.2/32"),
	} {
		t.Run(name, func(t *testing.T) {
			running := Execution{Id: uuid.New(), ExperimentKey: "ADM-1", ExperimentExecutionId: 42}
			require.NoError(t, Apply(t.Context(), running, bandwidthOpts("192.0.2.1/32")))
			t.Cleanup(func() { _ = Revert(context.Background(), running, bandwidthOpts("192.0.2.1/32")) })

			rejected := Execution{Id: uuid.New()}
			err := Apply(t.Context(), rejected, second)
			require.ErrorContains(t, err, "running multiple network attacks at the same time is not supported. Already running attack started by ADM-1 (#42) in targetExecution "+running.Id.String())

			// The platform may stop the rejected attack, which must keep the running one.
			require.NoError(t, Revert(t.Context(), rejected, second))
			assert.Equal(t, []string{qosPolicyPrefix + "8MB_0"}, qosPolicyNames(t))

			require.NoError(t, Revert(t.Context(), running, bandwidthOpts("192.0.2.1/32")))
			assert.Empty(t, qosPolicyNames(t))
		})
	}
}

func bandwidthOpts(cidr string) *LimitBandwidthOpts {
	_, ipNet, _ := net.ParseCIDR(cidr)
	return &LimitBandwidthOpts{Bandwidth: "8MB", BitsPerSecond: 8_000_000, IncludeCidrs: []net.IPNet{*ipNet}}
}
