// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"strings"
	"time"
)

type DelayOpts struct {
	Filter
	Delay    time.Duration
	Duration time.Duration
	Jitter   bool
}

func (o *DelayOpts) QoSPolicies() []QosPolicy {
	return nil
}

func (o *DelayOpts) WinDivert() *WinDivertOpts {
	return &WinDivertOpts{Filter: o.Filter, Mode: WinDivertModeDelay, Duration: o.Duration, Delay: o.Delay, Jitter: o.Jitter}
}

func (o *DelayOpts) String() string {
	var sb strings.Builder
	sb.WriteString("delaying traffic by ")
	sb.WriteString(o.Delay.String())
	sb.WriteString(" (jitter: ")
	if o.Jitter {
		sb.WriteString("yes")
	} else {
		sb.WriteString("no")
	}
	sb.WriteString(")")
	o.Filter.writeStringForFilters(&sb)
	return sb.String()
}
