// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"fmt"
	"strings"
	"time"
)

type PackageLossOpts struct {
	Filter
	Loss     uint
	Duration time.Duration
}

func (o *PackageLossOpts) QoSCommands(_ Mode) ([]string, error) {
	return nil, nil
}

func (o *PackageLossOpts) WinDivert() *WinDivertOpts {
	return &WinDivertOpts{Filter: o.Filter, Mode: WinDivertModeDrop, Duration: o.Duration, Percentage: o.Loss}
}

func (o *PackageLossOpts) String() string {
	var sb strings.Builder
	sb.WriteString("loosing packages of ")
	sb.WriteString(fmt.Sprintf("%d%%", o.Loss))
	o.Filter.writeStringForFilters(&sb)
	return sb.String()
}
