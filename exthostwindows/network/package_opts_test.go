// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package network

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFilter(t *testing.T) Filter {
	t.Helper()
	include, err := akn.ParseCIDR("1.1.1.0/24")
	require.NoError(t, err)
	exclude, err := akn.ParseCIDR("1.1.1.1/32")
	require.NoError(t, err)
	return Filter{
		Direction:  DirectionOutgoing,
		Include:    []akn.NetWithPortRange{{Net: *include, PortRange: akn.PortRangeAny}},
		Exclude:    []akn.NetWithPortRange{{Net: *exclude, PortRange: akn.PortRangeAny}},
		Interfaces: []Interface{{Index: 7, Name: "Ethernet 2"}},
	}
}

func TestCorruptPackagesOpts(t *testing.T) {
	opts := &CorruptPackagesOpts{Filter: testFilter(t), Corruption: 15, Duration: 30 * time.Second}

	assert.Empty(t, opts.QoSPolicies())

	assert.Equal(t, &WinDivertOpts{Filter: opts.Filter, Mode: WinDivertModeCorrupt, Duration: 30 * time.Second, Percentage: 15}, opts.WinDivert())

	assert.Equal(t, "corrupting packages of 15%\ndirection: Outgoing\nto/from:\n 1.1.1.0/24\nbut not from/to:\n 1.1.1.1/32\non interfaces:\n 7    Ethernet 2\n", opts.String())
}

func TestPackageLossOpts(t *testing.T) {
	opts := &PackageLossOpts{Filter: testFilter(t), Loss: 70, Duration: 90 * time.Second}

	assert.Empty(t, opts.QoSPolicies())

	assert.Equal(t, &WinDivertOpts{Filter: opts.Filter, Mode: WinDivertModeDrop, Duration: 90 * time.Second, Percentage: 70}, opts.WinDivert())

	assert.Equal(t, "loosing packages of 70%\ndirection: Outgoing\nto/from:\n 1.1.1.0/24\nbut not from/to:\n 1.1.1.1/32\non interfaces:\n 7    Ethernet 2\n", opts.String())
}

func TestPackageLossOpts_MultipleInterfaces(t *testing.T) {
	filter := testFilter(t)
	filter.Interfaces = []Interface{{Index: 7, Name: "Ethernet 2"}, {Index: 12, Name: "vEthernet (WSL)"}}
	opts := &PackageLossOpts{Filter: filter, Loss: 70, Duration: 90 * time.Second}

	assert.Contains(t, opts.String(), "on interfaces:\n 7    Ethernet 2\n 12   vEthernet (WSL)\n")
}

func TestWinDivertFilterFile(t *testing.T) {
	filter := testFilter(t)
	executionId := uuid.New()
	file, err := buildWinDivertFilterFile(executionId, filter)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(file) })

	expected, err := buildWinDivertFilter(filter)
	require.NoError(t, err)
	content, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, expected, string(content))

	require.NoError(t, stopWinDivert(executionId))
	assert.NoFileExists(t, file)
}

func TestWinDivertArgs(t *testing.T) {
	tests := []struct {
		name string
		opts WinDivertOpts
		want []string
	}{
		{"delay", WinDivertOpts{Mode: WinDivertModeDelay, Duration: 30 * time.Second, Delay: 100 * time.Millisecond}, []string{"--file=f.txt", "--mode=delay", "--duration=30", "--time=100"}},
		{"delay with jitter", WinDivertOpts{Mode: WinDivertModeDelay, Duration: 30 * time.Second, Delay: 100 * time.Millisecond, Jitter: true}, []string{"--file=f.txt", "--mode=delay", "--duration=30", "--time=100", "--jitter"}},
		{"drop", WinDivertOpts{Mode: WinDivertModeDrop, Duration: 90 * time.Second, Percentage: 70}, []string{"--file=f.txt", "--mode=drop", "--duration=90", "--percentage=70"}},
		{"corrupt", WinDivertOpts{Mode: WinDivertModeCorrupt, Duration: 5 * time.Second, Percentage: 15}, []string{"--file=f.txt", "--mode=corrupt", "--duration=5", "--percentage=15"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.opts.args("f.txt"))
		})
	}
}
