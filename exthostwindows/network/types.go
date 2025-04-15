// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"strconv"
	"strings"
)

type Mode = akn.Mode

var (
	ModeAdd    = akn.ModeAdd
	ModeDelete = akn.ModeDelete
)

type Family = akn.Family

var (
	FamilyV4 = akn.FamilyV4
	FamilyV6 = akn.FamilyV6
)

type WinOpts interface {
	QoSCommands(mode Mode) ([]string, error)
	WinDivertCommands(mode Mode) ([]string, error)
	String() string
}

type Filter struct {
	Filter           akn.Filter
	InterfaceIndexes []int
}

func (filter *Filter) writeStringForFilters(sb *strings.Builder) {
	f := filter.Filter
	sb.WriteString("\nto/from:\n")
	for _, inc := range f.Include {
		sb.WriteString(" ")
		sb.WriteString(inc.String())
		sb.WriteString("\n")
	}
	if len(f.Exclude) > 0 {
		sb.WriteString("but not from/to:\n")
		for _, exc := range f.Exclude {
			sb.WriteString(" ")
			sb.WriteString(exc.String())
			sb.WriteString("\n")
		}
	}
	if len(filter.InterfaceIndexes) > 0 {
		sb.WriteString("on interfaces:\n")
		for _, ifIdx := range filter.InterfaceIndexes {
			sb.WriteString(" ")
			sb.WriteString(strconv.Itoa(ifIdx))
			sb.WriteString("\n")
		}
	}
}
