// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import (
	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

func Test_MapToNetworks(t *testing.T) {
	ipnets, err := MapToNetworks(t.Context(), "localhost", "127.0.0.0/8")
	require.NoError(t, err)
	require.NotZero(t, len(ipnets))
}

func Test_ParsePortRanges(t *testing.T) {
	testCases := []struct {
		name           string
		input          []string
		expectedRanges []akn.PortRange
		expectError    bool
	}{
		{
			name:           "nil input",
			input:          nil,
			expectedRanges: nil,
			expectError:    false,
		},
		{
			name:           "empty input",
			input:          []string{},
			expectedRanges: nil,
			expectError:    false,
		},
		{
			name:           "single empty string",
			input:          []string{""},
			expectedRanges: nil,
			expectError:    false,
		},
		{
			name:           "multiple empty strings",
			input:          []string{"", "", ""},
			expectedRanges: nil,
			expectError:    false,
		},
		{
			name:  "single port",
			input: []string{"80"},
			expectedRanges: []akn.PortRange{
				{
					From: 80,
					To:   80,
				},
			},
			expectError: false,
		},
		{
			name:  "port range",
			input: []string{"8080-8090"},
			expectedRanges: []akn.PortRange{
				{
					From: 8080,
					To:   8090,
				},
			},
			expectError: false,
		},
		{
			name:  "multiple port ranges",
			input: []string{"80", "443", "8080-8090"},
			expectedRanges: []akn.PortRange{
				{
					From: 80,
					To:   80,
				},
				{
					From: 443,
					To:   443,
				},
				{
					From: 8080,
					To:   8090,
				},
			},
			expectError: false,
		},
		{
			name:  "mix of empty and valid ranges",
			input: []string{"", "80", "", "443"},
			expectedRanges: []akn.PortRange{
				{
					From: 80,
					To:   80,
				},
				{
					From: 443,
					To:   443,
				},
			},
			expectError: false,
		},
		{
			name:           "invalid port (negative)",
			input:          []string{"-80"},
			expectedRanges: nil,
			expectError:    true,
		},
		{
			name:           "invalid port (too large)",
			input:          []string{"65536"},
			expectedRanges: nil,
			expectError:    true,
		},
		{
			name:           "invalid port range (From > To)",
			input:          []string{"8090-8080"},
			expectedRanges: nil,
			expectError:    true,
		},
		{
			name:           "invalid port range (non-numeric)",
			input:          []string{"abc"},
			expectedRanges: nil,
			expectError:    true,
		},
		{
			name:           "mixed valid and invalid",
			input:          []string{"80", "invalid", "443"},
			expectedRanges: nil,
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ranges, err := ParsePortRanges(tc.input)
			if tc.expectError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tc.expectedRanges == nil {
				assert.Nil(t, ranges)
			} else {
				require.Equal(t, len(tc.expectedRanges), len(ranges))
				for i, expected := range tc.expectedRanges {
					assert.Equal(t, expected.From, ranges[i].From)
					assert.Equal(t, expected.To, ranges[i].To)
				}
			}
		})
	}
}

func Test_ParsePortRanges_EdgeCases(t *testing.T) {
	t.Run("minimum valid port", func(t *testing.T) {
		ranges, err := ParsePortRanges([]string{"1"})
		require.NoError(t, err)
		require.Len(t, ranges, 1)
		assert.Equal(t, uint16(1), ranges[0].From)
		assert.Equal(t, uint16(1), ranges[0].To)
	})

	t.Run("maximum valid port", func(t *testing.T) {
		ranges, err := ParsePortRanges([]string{"65534"})
		require.NoError(t, err)
		require.Len(t, ranges, 1)
		assert.Equal(t, uint16(65534), ranges[0].From)
		assert.Equal(t, uint16(65534), ranges[0].To)
	})

	t.Run("full range", func(t *testing.T) {
		ranges, err := ParsePortRanges([]string{"1-65534"})
		require.NoError(t, err)
		require.Len(t, ranges, 1)
		assert.Equal(t, uint16(1), ranges[0].From)
		assert.Equal(t, uint16(65534), ranges[0].To)
	})
}

func Test_CondenseNetWithPortRange(t *testing.T) {
	// Helper function to create NetWithPortRange objects for testing
	createNWP := func(cidr string, portFrom, portTo uint16) akn.NetWithPortRange {
		_, ipNet, _ := net.ParseCIDR(cidr)
		return akn.NetWithPortRange{
			Net: *ipNet,
			PortRange: akn.PortRange{
				From: portFrom,
				To:   portTo,
			},
		}
	}

	// Helper function to check if a network contains another
	contains := func(container, contained akn.NetWithPortRange) bool {
		// Check if the network contains the other network
		containerMaskSize, _ := container.Net.Mask.Size()
		containedMaskSize, _ := contained.Net.Mask.Size()

		if containerMaskSize > containedMaskSize {
			return false
		}

		// Check if the IP address is contained
		containerIP := container.Net.IP.Mask(container.Net.Mask)
		containedIP := contained.Net.IP.Mask(container.Net.Mask)

		for i := range containerIP {
			if containerIP[i] != containedIP[i] {
				return false
			}
		}

		// Check if port range is contained
		return container.PortRange.From <= contained.PortRange.From &&
			container.PortRange.To >= contained.PortRange.To
	}

	// Helper function to verify all original networks are covered by the condensed result
	verifyCoverage := func(t *testing.T, original, condensed []akn.NetWithPortRange) {
		for _, orig := range original {
			covered := false
			for _, cond := range condensed {
				if contains(cond, orig) {
					covered = true
					break
				}
			}
			assert.True(t, covered, "Original network %v with ports %d-%d is not covered by condensed result",
				orig.Net.String(), orig.PortRange.From, orig.PortRange.To)
		}
	}

	t.Run("input under limit returns unchanged", func(t *testing.T) {
		input := []akn.NetWithPortRange{
			createNWP("192.168.1.0/24", 80, 80),
			createNWP("10.0.0.0/8", 443, 443),
		}
		limit := 5

		result := CondenseNetWithPortRange(input, limit)

		assert.Equal(t, len(input), len(result), "Length should be unchanged when under limit")
		assert.ElementsMatch(t, input, result, "Elements should match when under limit")
	})

	t.Run("input at limit returns unchanged", func(t *testing.T) {
		input := []akn.NetWithPortRange{
			createNWP("192.168.1.0/24", 80, 80),
			createNWP("10.0.0.0/8", 443, 443),
		}
		limit := 2

		result := CondenseNetWithPortRange(input, limit)

		assert.Equal(t, len(input), len(result), "Length should be unchanged when at limit")
		assert.ElementsMatch(t, input, result, "Elements should match when at limit")
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		var input []akn.NetWithPortRange
		limit := 10

		result := CondenseNetWithPortRange(input, limit)

		assert.Empty(t, result, "Empty input should return empty result")
	})

	t.Run("mergeable networks with same port range", func(t *testing.T) {
		input := []akn.NetWithPortRange{
			createNWP("192.168.1.0/25", 80, 80),   // First half of 192.168.1.0/24
			createNWP("192.168.1.128/25", 80, 80), // Second half of 192.168.1.0/24
			createNWP("10.0.0.0/8", 443, 443),
		}
		limit := 2

		result := CondenseNetWithPortRange(input, limit)

		assert.LessOrEqual(t, len(result), limit, "Result should be at or under limit")
		verifyCoverage(t, input, result)

		// Check if the two 192.168.1.x networks were merged
		merged := false
		for _, nwp := range result {
			if nwp.Net.String() == "192.168.1.0/24" && nwp.PortRange.From == 80 && nwp.PortRange.To == 80 {
				merged = true
				break
			}
		}
		assert.True(t, merged, "Networks should be merged when possible")
	})

	t.Run("non-mergeable networks with different port ranges", func(t *testing.T) {
		input := []akn.NetWithPortRange{
			createNWP("192.168.1.0/25", 80, 80),
			createNWP("192.168.1.128/25", 443, 443), // Different port range
			createNWP("10.0.0.0/8", 8080, 8090),
		}
		limit := 2

		result := CondenseNetWithPortRange(input, limit)

		assert.Equal(t, len(input), len(result), "Length should be unchanged when at limit")
		assert.ElementsMatch(t, input, result, "Elements should match when at limit")
	})

	t.Run("prioritize longer prefix networks when condensing", func(t *testing.T) {
		input := []akn.NetWithPortRange{
			createNWP("192.168.1.0/26", 80, 80),   // 1st quarter
			createNWP("192.168.1.64/26", 80, 80),  // 2nd quarter
			createNWP("192.168.1.128/26", 80, 80), // 3rd quarter
			createNWP("192.168.1.192/26", 80, 80), // 4th quarter
			createNWP("10.0.0.0/24", 443, 443),
			createNWP("10.0.1.0/24", 443, 443),
		}
		limit := 2

		result := CondenseNetWithPortRange(input, limit)

		assert.LessOrEqual(t, len(result), limit, "Result should be at or under limit")
		verifyCoverage(t, input, result)

		// Check if the four 192.168.1.x networks were merged into one
		merged := false
		for _, nwp := range result {
			if nwp.Net.String() == "192.168.1.0/24" && nwp.PortRange.From == 80 && nwp.PortRange.To == 80 {
				merged = true
				break
			}
		}
		assert.True(t, merged, "Networks should be merged when possible")
	})

	t.Run("complex mixed scenario", func(t *testing.T) {
		input := []akn.NetWithPortRange{
			// Mergeable group 1 (same port)
			createNWP("192.168.1.0/26", 80, 80),
			createNWP("192.168.1.64/26", 80, 80),

			// Mergeable group 2 (same port, different network)
			createNWP("10.1.0.0/24", 443, 443),
			createNWP("10.2.0.0/24", 443, 443),

			// Non-mergeable (different ports)
			createNWP("172.16.0.0/24", 8080, 8080),
			createNWP("172.16.0.0/24", 9090, 9090),

			// Standalone
			createNWP("10.10.10.10/32", 22, 22),
		}
		limit := 4

		result := CondenseNetWithPortRange(input, limit)

		assert.Equal(t, 5, len(result), "Networks can not be mered further")
		verifyCoverage(t, input, result)
	})

	t.Run("limit of 1 forces maximum condensation", func(t *testing.T) {
		input := []akn.NetWithPortRange{
			createNWP("192.168.1.0/25", 80, 80),
			createNWP("192.168.1.128/25", 80, 80),
			createNWP("10.0.0.0/8", 443, 443),
		}
		limit := 1

		result := CondenseNetWithPortRange(input, limit)

		assert.Equal(t, 2, len(result), "Networks can not be mered further")
		verifyCoverage(t, input, result)
	})
}
