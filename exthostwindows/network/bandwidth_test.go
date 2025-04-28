package network

import (
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_LimitBandwidth_create_QoSPolicy(t *testing.T) {
	expectedPolicyName := "steadybit_qos_100mb"
	limitBandwidthOpts := LimitBandwidthOpts{
		Bandwidth: "100MB",
		Port:      9876,
	}

	err := Apply(t.Context(), &limitBandwidthOpts)
	require.NoError(t, err)
	defer func() {
		err := Revert(t.Context(), &limitBandwidthOpts)
		require.NoError(t, err)
		out, err := listQoSPolicies(t)
		require.NoError(t, err)
		require.NotContains(t, out, expectedPolicyName)
	}()

	out, err := listQoSPolicies(t)
	require.NoError(t, err)
	require.Contains(t, out, expectedPolicyName)
}

func listQoSPolicies(t *testing.T) (string, error) {
	listQosPoliciesCommand := []string{"Get-NetQosPolicy -PolicyStore ActiveStore"}
	return utils.Execute(t.Context(), listQosPoliciesCommand, utils.PSRun)
}
