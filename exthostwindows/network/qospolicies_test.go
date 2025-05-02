// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"fmt"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"
	"strconv"
	"testing"
)

func TestListSteadybitQosPolicyNames(t *testing.T) {
	err := removeSteadybitQosPolicies(t.Context())
	require.NoError(t, err)

	testQosPolicyName := qosPolicyPrefix + "test_" + strconv.Itoa(rand.Intn(100000))
	createQosPolicy(t, testQosPolicyName)
	defer func() {
		err := removeQoSPolicies(t.Context(), []string{testQosPolicyName})
		require.NoError(t, err)
	}()

	qosPolicyNames, err := listSteadybitQosPolicyNames(t.Context())
	require.NoError(t, err)
	require.Contains(t, qosPolicyNames, testQosPolicyName)
}

func TestListSteadybitQosPolicies(t *testing.T) {
	err := removeSteadybitQosPolicies(t.Context())
	require.NoError(t, err)

	testQosPolicyName := qosPolicyPrefix + "test_" + strconv.Itoa(rand.Intn(100000))
	createQosPolicy(t, testQosPolicyName)
	defer func() {
		err := removeQoSPolicies(t.Context(), []string{testQosPolicyName})
		require.NoError(t, err)
	}()

	qosPolicyNames, err := listSteadybitQosPolicies(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, qosPolicyNames)
}

func createQosPolicy(t *testing.T, name string) {
	command := fmt.Sprintf("New-NetQosPolicy -Name %s -ThrottleRateActionBitsPerSecond 100MB -Confirm:$false", name)
	_, err := utils.ExecutePowershellCommand(t.Context(), []string{command}, utils.PSRun)
	require.NoError(t, err)
}
