// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"fmt"
	"net"
	"strings"

	"github.com/steadybit/action-kit/go/action_kit_commons/network"
)

type LimitBandwidthOpts struct {
	// Bandwidth names the limit in the policy names, e.g. "1024KB" for 1024kbit.
	Bandwidth     string
	BitsPerSecond uint64
	IncludeCidrs  []net.IPNet
	PortRange     network.PortRange
}

func (o *LimitBandwidthOpts) WinDivert() *WinDivertOpts {
	return nil
}

func (o *LimitBandwidthOpts) QoSPolicies() []QosPolicy {
	policies := make([]QosPolicy, 0, len(o.IncludeCidrs))
	for i, includeCidr := range o.IncludeCidrs {
		policy := QosPolicy{
			Name:          fmt.Sprintf("%s%s_%d", qosPolicyPrefix, o.Bandwidth, i),
			BitsPerSecond: o.BitsPerSecond,
			DstPrefix:     includeCidr.String(),
		}
		if o.PortRange.From != 0 && o.PortRange.To != 0 {
			policy.DstPortFrom, policy.DstPortTo = uint16(o.PortRange.From), uint16(o.PortRange.To)
		}
		policies = append(policies, policy)
	}
	return policies
}

func (o *LimitBandwidthOpts) String() string {
	var sb strings.Builder
	sb.WriteString("limit bandwidth to ")
	sb.WriteString(o.Bandwidth)
	if len(o.IncludeCidrs) > 0 {
		sb.WriteString(" for:\n")
		for _, includeCidr := range o.IncludeCidrs {
			sb.WriteString(" ")
			sb.WriteString(includeCidr.String())
			if o.PortRange.From != 0 && o.PortRange.To != 0 {
				sb.WriteString(fmt.Sprintf(":%d-%d", o.PortRange.From, o.PortRange.To))
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
