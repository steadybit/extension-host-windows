// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package network

import (
	"fmt"
	"os"
	"testing"
	"time"

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
		Direction:        DirectionOutgoing,
		Include:          []akn.NetWithPortRange{{Net: *include, PortRange: akn.PortRangeAny}},
		Exclude:          []akn.NetWithPortRange{{Net: *exclude, PortRange: akn.PortRangeAny}},
		InterfaceIndexes: []int{7},
	}
}

func TestCorruptPackagesOpts(t *testing.T) {
	opts := &CorruptPackagesOpts{Filter: testFilter(t), Corruption: 15, Duration: 30 * time.Second}

	qosCmds, err := opts.QoSCommands(ModeAdd)
	require.NoError(t, err)
	assert.Empty(t, qosCmds)

	addCmds, err := opts.WinDivertCommands(ModeAdd)
	require.NoError(t, err)
	require.NotEmpty(t, opts.FilterFile)
	assert.Equal(t, []string{fmt.Sprintf("wdna.exe --file=%q --mode=corrupt --duration=30 --percentage=15", opts.FilterFile)}, addCmds)

	expectedFilter, err := buildWinDivertFilter(opts.Filter)
	require.NoError(t, err)
	content, err := os.ReadFile(opts.FilterFile)
	require.NoError(t, err)
	assert.Equal(t, expectedFilter, string(content))

	delCmds, err := opts.WinDivertCommands(ModeDelete)
	require.NoError(t, err)
	assert.Equal(t, []string{"wdna_shutdown", "cmd /c \"sc stop windivert || exit /b 0\""}, delCmds)
	assert.NoFileExists(t, opts.FilterFile)

	assert.Equal(t, "corrupting packages of 15%\ndirection: Outgoing\nto/from:\n 1.1.1.0/24\nbut not from/to:\n 1.1.1.1/32\non interfaces:\n 7\n", opts.String())
}

func TestPackageLossOpts(t *testing.T) {
	opts := &PackageLossOpts{Filter: testFilter(t), Loss: 70, Duration: 90 * time.Second}

	qosCmds, err := opts.QoSCommands(ModeAdd)
	require.NoError(t, err)
	assert.Empty(t, qosCmds)

	addCmds, err := opts.WinDivertCommands(ModeAdd)
	require.NoError(t, err)
	require.NotEmpty(t, opts.filterFile)
	assert.Equal(t, []string{fmt.Sprintf("wdna.exe --file=%q --mode=drop --duration=90 --percentage=70", opts.filterFile)}, addCmds)
	assert.FileExists(t, opts.filterFile)

	delCmds, err := opts.WinDivertCommands(ModeDelete)
	require.NoError(t, err)
	assert.Equal(t, []string{"wdna_shutdown", "cmd /c \"sc stop windivert || exit /b 0\""}, delCmds)
	assert.NoFileExists(t, opts.filterFile)

	assert.Equal(t, "loosing packages of 70%\ndirection: Outgoing\nto/from:\n 1.1.1.0/24\nbut not from/to:\n 1.1.1.1/32\non interfaces:\n 7\n", opts.String())
}
