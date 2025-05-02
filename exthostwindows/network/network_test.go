// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCleanupQosPolicies(t *testing.T) {
	// Cleanup works if no leftover policies exist
	CleanupQosPolicies()

	// Cleanup does not run if experiment is active
	createQosPolicy(t, qosPolicyPrefix+"test_policy_cleanup")
	opts := &MockNetworkOpt{}
	err := generateAndRunCommands(t.Context(), opts, ModeAdd)
	require.NoError(t, err)

	CleanupQosPolicies()

	policies, err := listSteadybitQosPolicyNames(t.Context())
	require.NoError(t, err)
	assert.NotEmpty(t, policies)

	// Cleanup removes leftover policies after the experiment has finished
	err = generateAndRunCommands(t.Context(), opts, ModeDelete)
	require.NoError(t, err)

	CleanupQosPolicies()

	policies, err = listSteadybitQosPolicyNames(t.Context())
	require.NoError(t, err)
	assert.Empty(t, policies)
}

type MockNetworkOpt struct{}

func (o *MockNetworkOpt) WinDivertCommands(_ Mode) ([]string, error) {
	return nil, nil
}

func (o *MockNetworkOpt) QoSCommands(_ Mode) ([]string, error) {
	return nil, nil
}

func (o *MockNetworkOpt) String() string {
	return "MockNetworkOpt"
}
