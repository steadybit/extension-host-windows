package network

import (
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

func Test_LimitBandwidth_create_one_policy_per_ip(t *testing.T) {
	expectedPolicyName1 := "steadybit_qos_100mb_0"
	_, ipNet1, err := net.ParseCIDR("1.1.1.1/32")
	require.NoError(t, err)

	expectedPolicyName2 := "steadybit_qos_100mb_1"
	_, ipNet2, err := net.ParseCIDR("1.1.1.2/32")
	require.NoError(t, err)

	limitBandwidthOpts := LimitBandwidthOpts{
		Bandwidth: "100MB",
		IncludeCidrs: []net.IPNet{
			*ipNet1, *ipNet2,
		},
		Port: 9876,
	}

	err = Apply(t.Context(), &limitBandwidthOpts)
	require.NoError(t, err)

	defer func() {
		err := Revert(t.Context(), &limitBandwidthOpts)
		require.NoError(t, err)
		out, err := listQoSPolicies(t)
		require.NoError(t, err)
		require.NotContains(t, out, expectedPolicyName1)
		require.NotContains(t, out, expectedPolicyName2)
	}()

	out, err := listQoSPolicies(t)
	require.NoError(t, err)
	require.Contains(t, out, expectedPolicyName1)
	require.Contains(t, out, expectedPolicyName2)
}

func listQoSPolicies(t *testing.T) (string, error) {
	listQosPoliciesCommand := []string{"Get-NetQosPolicy -PolicyStore ActiveStore"}
	return utils.Execute(t.Context(), listQosPoliciesCommand, utils.PSRun)
}
