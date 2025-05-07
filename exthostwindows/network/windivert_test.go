// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"testing"

	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWinDivertGetStartEndIP1(t *testing.T) {
	parsedNet, err := akn.ParseCIDR("1.1.1.1/24")
	require.NoError(t, err)

	startIp, endIp, err := getStartEndIP(*parsedNet)
	require.NoError(t, err)

	assert.Equal(t, "1.1.1.0", startIp.String())
	assert.Equal(t, "1.1.1.255", endIp.String())
}

func TestWinDivertGetStartEndIP2(t *testing.T) {
	parsedNet, err := akn.ParseCIDR("1.1.3.120/22")
	require.NoError(t, err)

	startIp, endIp, err := getStartEndIP(*parsedNet)
	require.NoError(t, err)

	assert.Equal(t, "1.1.0.0", startIp.String())
	assert.Equal(t, "1.1.3.255", endIp.String())
}

func TestWinDivertBuildFilterInclude(t *testing.T) {
	net1, err := akn.ParseCIDR("1.1.1.1/24")
	require.NoError(t, err)
	f := Filter{
		Direction: DirectionOutgoing,
		Filter: akn.Filter{
			Include: []akn.NetWithPortRange{
				{
					Net:       *net1,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
		},
	}

	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 ))))", filter)
}

func TestWinDivertBuildFilterIncludeInbound(t *testing.T) {
	net1, err := akn.ParseCIDR("1.1.1.1/24")
	require.NoError(t, err)
	f := Filter{
		Direction: DirectionIncoming,
		Filter: akn.Filter{
			Include: []akn.NetWithPortRange{
				{
					Net:       *net1,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
		},
	}

	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and inbound and (( ip.SrcAddr >= 1.1.1.0 and ip.SrcAddr <= 1.1.1.255 and (( tcp.SrcPort >= 8000 and tcp.SrcPort <= 8002 ) or ( udp.SrcPort >= 8000 and udp.SrcPort <= 8002 ))))", filter)
}

func TestWinDivertBuildFilterIncludeInOut(t *testing.T) {
	net1, err := akn.ParseCIDR("1.1.1.1/24")
	require.NoError(t, err)
	f := Filter{
		Direction: DirectionAll,
		Filter: akn.Filter{
			Include: []akn.NetWithPortRange{
				{
					Net:       *net1,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
		},
	}

	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and (( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 ))) or ( ip.SrcAddr >= 1.1.1.0 and ip.SrcAddr <= 1.1.1.255 and (( tcp.SrcPort >= 8000 and tcp.SrcPort <= 8002 ) or ( udp.SrcPort >= 8000 and udp.SrcPort <= 8002 ))))", filter)
}

func TestWinDivertBuildFilterIncludeIpv6(t *testing.T) {
	net1, err := akn.ParseCIDR("::/0")
	require.NoError(t, err)
	f := Filter{
		Direction: DirectionOutgoing,
		Filter: akn.Filter{
			Include: []akn.NetWithPortRange{
				{
					Net:       *net1,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
		},
	}

	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (( ipv6.DstAddr >= :: and ipv6.DstAddr <= ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 ))))", filter)
}

func TestWinDivertBuildFilterExclude(t *testing.T) {
	net1, err := akn.ParseCIDR("1.1.1.1/24")
	require.NoError(t, err)
	exemptNet, err := akn.ParseCIDR("1.1.1.0")
	require.NoError(t, err)
	f := Filter{
		Direction: DirectionOutgoing,
		Filter: akn.Filter{
			Include: []akn.NetWithPortRange{
				{
					Net:       *net1,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
			Exclude: []akn.NetWithPortRange{
				{
					Net:       *exemptNet,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
		},
	}

	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 )))) and ((( ip.DstAddr == 1.1.1.0 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true) and (( ip.SrcAddr == 1.1.1.0 )? (( tcp.SrcPort < 8000 or tcp.SrcPort > 8002 ) or ( udp.SrcPort < 8000 or udp.SrcPort > 8002 )): true))", filter)
}

func TestWinDivertBuildFilterMultipleExcludes(t *testing.T) {
	net1, err := akn.ParseCIDR("1.1.1.1/24")
	require.NoError(t, err)
	exemptNet, err := akn.ParseCIDR("1.1.1.0")
	require.NoError(t, err)
	exemptNet2, err := akn.ParseCIDR("1.1.1.1")
	require.NoError(t, err)
	f := Filter{
		Direction: DirectionOutgoing,
		Filter: akn.Filter{
			Include: []akn.NetWithPortRange{
				{
					Net:       *net1,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
			Exclude: []akn.NetWithPortRange{
				{
					Net:       *exemptNet,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
				{
					Net:       *exemptNet2,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
		},
	}

	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 )))) and ((( ip.DstAddr == 1.1.1.0 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true) and (( ip.SrcAddr == 1.1.1.0 )? (( tcp.SrcPort < 8000 or tcp.SrcPort > 8002 ) or ( udp.SrcPort < 8000 or udp.SrcPort > 8002 )): true) and (( ip.DstAddr == 1.1.1.1 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true) and (( ip.SrcAddr == 1.1.1.1 )? (( tcp.SrcPort < 8000 or tcp.SrcPort > 8002 ) or ( udp.SrcPort < 8000 or udp.SrcPort > 8002 )): true))", filter)
}

func TestWinDivertBuildFilterMultipleIncludes(t *testing.T) {
	net1, err := akn.ParseCIDR("1.1.1.1/24")
	require.NoError(t, err)
	net2, err := akn.ParseCIDR("1.1.2.1/24")
	require.NoError(t, err)
	exemptNet, err := akn.ParseCIDR("1.1.1.0")
	require.NoError(t, err)
	f := Filter{
		Direction: DirectionOutgoing,
		Filter: akn.Filter{
			Include: []akn.NetWithPortRange{
				{
					Net:       *net1,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
				{
					Net:       *net2,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
			Exclude: []akn.NetWithPortRange{
				{
					Net:       *exemptNet,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
		},
	}

	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (( ip.DstAddr >= 1.1.1.0 and ip.DstAddr <= 1.1.1.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 ))) or ( ip.DstAddr >= 1.1.2.0 and ip.DstAddr <= 1.1.2.255 and (( tcp.DstPort >= 8000 and tcp.DstPort <= 8002 ) or ( udp.DstPort >= 8000 and udp.DstPort <= 8002 )))) and ((( ip.DstAddr == 1.1.1.0 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true) and (( ip.SrcAddr == 1.1.1.0 )? (( tcp.SrcPort < 8000 or tcp.SrcPort > 8002 ) or ( udp.SrcPort < 8000 or udp.SrcPort > 8002 )): true))", filter)
}

func TestWinDivertBuildFilterOnlyExclude(t *testing.T) {
	excludeNet, err := akn.ParseCIDR("1.1.1.14")
	require.NoError(t, err)
	f := Filter{
		Direction: DirectionOutgoing,
		Filter: akn.Filter{
			Exclude: []akn.NetWithPortRange{
				{
					Net:       *excludeNet,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
		},
	}

	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and ((( ip.DstAddr == 1.1.1.14 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true) and (( ip.SrcAddr == 1.1.1.14 )? (( tcp.SrcPort < 8000 or tcp.SrcPort > 8002 ) or ( udp.SrcPort < 8000 or udp.SrcPort > 8002 )): true))", filter)
}

func TestWinDivertBuildFilterInterfaces(t *testing.T) {
	filter, err := buildWinDivertFilter(Filter{
		InterfaceIndexes: []int{1, 2, 3},
	})
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (ifIdx == 1 or ifIdx == 2 or ifIdx == 3)", filter)
}

func TestWinDivertBuildFilterInterfacesAndIncludeOutgoing(t *testing.T) {
	excludeNet, err := akn.ParseCIDR("1.1.1.14")
	require.NoError(t, err)
	f := Filter{
		Direction: DirectionOutgoing,
		Filter: akn.Filter{
			Exclude: []akn.NetWithPortRange{
				{
					Net:       *excludeNet,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
		},
		InterfaceIndexes: []int{1, 2, 3},
	}

	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and outbound and (ifIdx == 1 or ifIdx == 2 or ifIdx == 3) and ((( ip.DstAddr == 1.1.1.14 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true) and (( ip.SrcAddr == 1.1.1.14 )? (( tcp.SrcPort < 8000 or tcp.SrcPort > 8002 ) or ( udp.SrcPort < 8000 or udp.SrcPort > 8002 )): true))", filter)
}

func TestWinDivertBuildFilterInterfacesAndIncludeInbound(t *testing.T) {
	excludeNet, err := akn.ParseCIDR("1.1.1.14")
	require.NoError(t, err)
	f := Filter{
		Direction: DirectionIncoming,
		Filter: akn.Filter{
			Exclude: []akn.NetWithPortRange{
				{
					Net:       *excludeNet,
					Comment:   "",
					PortRange: akn.PortRange{From: 8000, To: 8002},
				},
			},
		},
		InterfaceIndexes: []int{1, 2, 3},
	}

	filter, err := buildWinDivertFilter(f)
	assert.NoError(t, err)

	assert.Equal(t, "(tcp or udp) and inbound and (ifIdx == 1 or ifIdx == 2 or ifIdx == 3) and ((( ip.DstAddr == 1.1.1.14 )? (( tcp.DstPort < 8000 or tcp.DstPort > 8002 ) or ( udp.DstPort < 8000 or udp.DstPort > 8002 )): true) and (( ip.SrcAddr == 1.1.1.14 )? (( tcp.SrcPort < 8000 or tcp.SrcPort > 8002 ) or ( udp.SrcPort < 8000 or udp.SrcPort > 8002 )): true))", filter)
}

func TestWinDivertBuildFilterDirection(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		filter, _ := buildWinDivertFilter(Filter{})
		assert.Equal(t, "(tcp or udp) and outbound", filter)
	})

	t.Run("outgoing", func(t *testing.T) {
		filter, _ := buildWinDivertFilter(Filter{
			Direction: DirectionOutgoing,
		})
		assert.Equal(t, "(tcp or udp) and outbound", filter)
	})

	t.Run("incoming", func(t *testing.T) {
		filter, _ := buildWinDivertFilter(Filter{
			Direction: DirectionIncoming,
		})
		assert.Equal(t, "(tcp or udp) and inbound", filter)
	})

	t.Run("all", func(t *testing.T) {
		filter, _ := buildWinDivertFilter(Filter{
			Direction: DirectionAll,
		})
		assert.Equal(t, "(tcp or udp)", filter)
	})
}
