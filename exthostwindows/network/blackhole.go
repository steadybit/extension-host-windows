// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type BlackholeOpts struct {
	Filter
	Duration   time.Duration
	FilterFile string
}

func (o *BlackholeOpts) QoSCommands(_ Mode) ([]string, error) {
	return nil, nil
}

func (o *BlackholeOpts) WinDivertCommands(mode Mode) ([]string, error) {
	var cmds []string

	if mode == ModeAdd {
		filterFile, err := buildWinDivertFilterFile(o.Filter)
		if err != nil {
			return nil, err
		}
		o.FilterFile = filterFile

		cmds = append(cmds, "ipconfig /flushdns")
		cmds = append(cmds, fmt.Sprintf("wdna.exe --file=%q --mode=drop --percentage=100 --duration=%d", filterFile, int(o.Duration.Seconds())))

	} else {
		cmds = append(cmds, "wdna_shutdown")
		cmds = append(cmds, "cmd /c \"sc stop windivert || exit /b 0\"") // don't fail on error
		_ = os.Remove(o.FilterFile)
	}

	return cmds, nil
}

func (o *BlackholeOpts) String() string {
	var sb strings.Builder
	sb.WriteString("blocking traffic ")
	o.Filter.writeStringForFilters(&sb)
	return sb.String()
}
