// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type CorruptPackagesOpts struct {
	Filter
	Corruption uint
	Duration   time.Duration
	FilterFile string
}

func (o *CorruptPackagesOpts) QoSCommands(_ Mode) ([]string, error) {
	return nil, nil
}

func (o *CorruptPackagesOpts) WinDivertCommands(mode Mode) ([]string, error) {
	var cmds []string

	if mode == ModeAdd {
		filterFile, err := buildWinDivertFilterFile(o.Filter)
		if err != nil {
			return nil, err
		}
		o.FilterFile = filterFile
		cmds = append(cmds, fmt.Sprintf("wdna.exe --file=%q --mode=corrupt --duration=%d --percentage=%d", filterFile, int(o.Duration.Seconds()), o.Corruption))

	} else {
		cmds = append(cmds, "wdna_shutdown")
		cmds = append(cmds, "cmd /c \"sc stop windivert || exit /b 0\"") // don't fail on error
		_ = os.Remove(o.FilterFile)
	}

	return cmds, nil
}

func (o *CorruptPackagesOpts) String() string {
	var sb strings.Builder
	sb.WriteString("corrupting packages of ")
	sb.WriteString(fmt.Sprintf("%d%%", o.Corruption))
	o.Filter.writeStringForFilters(&sb)
	return sb.String()
}
