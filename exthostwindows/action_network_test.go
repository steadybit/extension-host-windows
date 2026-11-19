// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"context"
	"net"
	"testing"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/stretchr/testify/require"
)

func Test_mapToNetworkFilter_excludeIp(t *testing.T) {
	tests := []struct {
		name         string
		actionConfig map[string]any
		wantExcluded []string
	}{
		{
			name:         "no excludeIp yields no parameter excludes",
			actionConfig: map[string]any{},
			wantExcluded: nil,
		},
		{
			name: "excludeIp CIDRs and IPs are excluded on all ports",
			actionConfig: map[string]any{
				"excludeIp": []any{"10.0.0.0/8", "192.168.1.1"},
			},
			wantExcluded: []string{"10.0.0.0/8 # parameters", "192.168.1.1/32 # parameters"},
		},
		{
			name: "excludeIp composes with include restrictions",
			actionConfig: map[string]any{
				"ip":        []any{"10.0.0.0/8"},
				"excludeIp": []any{"10.1.0.0/16"},
			},
			wantExcluded: []string{"10.1.0.0/16 # parameters"},
		},
		{
			name: "excludeHostname entries are excluded together with excludeIp",
			actionConfig: map[string]any{
				"excludeIp":       []any{"10.0.0.0/8"},
				"excludeHostname": []any{"192.168.1.1"},
			},
			wantExcluded: []string{"10.0.0.0/8 # parameters", "192.168.1.1/32 # parameters"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, _, err := mapToNetworkFilter(context.Background(), tt.actionConfig, nil)
			require.NoError(t, err)

			var parameterExcludes []string
			for _, e := range filter.Exclude {
				if e.Comment == "parameters" {
					require.Equal(t, akn.PortRangeAny, e.PortRange)
					parameterExcludes = append(parameterExcludes, e.String())
				}
			}
			require.Equal(t, tt.wantExcluded, parameterExcludes)

			for _, i := range filter.Include {
				require.Equal(t, "parameters", i.Comment)
			}
		})
	}
}

func Test_mapToNetworkFilter_networkInterface(t *testing.T) {
	ifaces, err := net.Interfaces()
	require.NoError(t, err)
	require.NotEmpty(t, ifaces)

	filter, _, err := mapToNetworkFilter(context.Background(), map[string]any{"networkInterface": []any{ifaces[0].Name}}, nil)
	require.NoError(t, err)
	require.Equal(t, []int{ifaces[0].Index}, filter.InterfaceIndexes)

	filter, _, err = mapToNetworkFilter(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	require.Empty(t, filter.InterfaceIndexes)

	filter, _, err = mapToNetworkFilter(context.Background(), map[string]any{"networkInterface": []any{""}}, nil)
	require.NoError(t, err)
	require.Empty(t, filter.InterfaceIndexes)

	_, _, err = mapToNetworkFilter(context.Background(), map[string]any{"networkInterface": []any{ifaces[0].Name, "no-such-nic"}}, nil)
	require.ErrorContains(t, err, `network interface "no-such-nic" not found`)
}

func Test_mapToNetworkFilter_excludePort(t *testing.T) {
	filter, _, err := mapToNetworkFilter(context.Background(), map[string]any{
		"excludePort": []any{"3389", "8000-8100", ""},
	}, nil)
	require.NoError(t, err)

	var parameterExcludes []string
	for _, e := range filter.Exclude {
		if e.Comment == "parameters" {
			parameterExcludes = append(parameterExcludes, e.String())
		}
	}
	// every address of both families, restricted to the excluded ports
	require.ElementsMatch(t, []string{
		"0.0.0.0/0 3389 # parameters",
		"0.0.0.0/0 8000-8100 # parameters",
		"::/0 3389 # parameters",
		"::/0 8000-8100 # parameters",
	}, parameterExcludes)

	_, _, err = mapToNetworkFilter(context.Background(), map[string]any{"excludePort": []any{"not-a-port"}}, nil)
	require.Error(t, err)
}

func Test_resolveInterfaceIndexes(t *testing.T) {
	ifaces := []net.Interface{
		{Index: 1, Name: "Loopback Pseudo-Interface 1"},
		{Index: 7, Name: "Ethernet"},
		{Index: 12, Name: "Ethernet 2"},
		{Index: 23, Name: "vEthernet (WSL (Hyper-V firewall))"},
		{Index: 31, Name: "Wi-Fi"},
	}

	tests := []struct {
		name    string
		names   []string
		want    []int
		wantErr string
	}{
		{name: "none", names: nil, want: nil},
		{name: "name with spaces", names: []string{"Ethernet 2"}, want: []int{12}},
		{name: "name with parentheses", names: []string{"vEthernet (WSL (Hyper-V firewall))"}, want: []int{23}},
		{name: "a name with spaces does not match its prefix", names: []string{"Ethernet"}, want: []int{7}},
		{name: "case and surrounding whitespace are ignored", names: []string{" ethernet 2 ", "WI-FI"}, want: []int{12, 31}},
		{name: "duplicates are reported once", names: []string{"Ethernet", "ethernet"}, want: []int{7}},
		{name: "loopback", names: []string{"Loopback Pseudo-Interface 1"}, want: []int{1}},
		{name: "empty entries are skipped", names: []string{"", "  "}, want: nil},
		{name: "unknown name lists the available ones", names: []string{"Ethernet 3"},
			wantErr: `network interface "Ethernet 3" not found, available: "Loopback Pseudo-Interface 1", "Ethernet", "Ethernet 2", "vEthernet (WSL (Hyper-V firewall))", "Wi-Fi"`},
		{name: "quoted name is not accepted", names: []string{`"Ethernet 2"`}, wantErr: "not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveInterfaceIndexes(tt.names, ifaces)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_networkInterfaceParameter_suggestsDiscoveredInterfaces(t *testing.T) {
	require.NotNil(t, networkInterfaceParameter.Options)
	require.Equal(t, []action_kit_api.ParameterOption{
		action_kit_api.ParameterOptionsFromTargetAttribute{Attribute: hostNicAttribute},
	}, *networkInterfaceParameter.Options)
}
