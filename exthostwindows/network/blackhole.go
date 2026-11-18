// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"strings"
	"time"
)

type BlackholeOpts struct {
	Filter
	Duration time.Duration
}

func (o *BlackholeOpts) QoSPolicies() []QosPolicy {
	return nil
}

func (o *BlackholeOpts) WinDivert() *WinDivertOpts {
	return &WinDivertOpts{Filter: o.Filter, Mode: WinDivertModeDrop, Duration: o.Duration, Percentage: 100, FlushDNS: true}
}

func (o *BlackholeOpts) String() string {
	var sb strings.Builder
	sb.WriteString("blocking traffic ")
	if !o.ExcludeLoopback {
		sb.WriteString("(including localhost) ")
	}
	o.Filter.writeStringForFilters(&sb)
	return sb.String()
}
