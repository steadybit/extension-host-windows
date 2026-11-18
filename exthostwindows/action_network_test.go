// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"context"
	"net"
	"testing"

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
	require.ErrorContains(t, err, "network interface no-such-nic not found")
}
