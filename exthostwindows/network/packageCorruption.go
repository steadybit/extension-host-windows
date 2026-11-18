// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"fmt"
	"strings"
	"time"
)

type CorruptPackagesOpts struct {
	Filter
	Corruption uint
	Duration   time.Duration
}

func (o *CorruptPackagesOpts) QoSPolicies() []QosPolicy {
	return nil
}

func (o *CorruptPackagesOpts) WinDivert() *WinDivertOpts {
	return &WinDivertOpts{Filter: o.Filter, Mode: WinDivertModeCorrupt, Duration: o.Duration, Percentage: o.Corruption}
}

func (o *CorruptPackagesOpts) String() string {
	var sb strings.Builder
	sb.WriteString("corrupting packages of ")
	sb.WriteString(fmt.Sprintf("%d%%", o.Corruption))
	o.Filter.writeStringForFilters(&sb)
	return sb.String()
}
