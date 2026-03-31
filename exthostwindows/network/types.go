// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"strconv"
	"strings"

	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
)

type Mode string

const (
	ModeAdd    Mode = "add"
	ModeDelete Mode = "del"
)

type Family string

const (
	FamilyV4 Family = "inet"
	FamilyV6 Family = "inet6"
)

type WinOpts interface {
	QoSCommands(mode Mode) ([]string, error)
	WinDivertCommands(mode Mode) ([]string, error)
	String() string
}

type Direction string

var (
	DirectionIncoming Direction = "Incoming"
	DirectionOutgoing Direction = "Outgoing"
	DirectionAll      Direction = "All"
)

type Filter struct {
	Include          []akn.NetWithPortRange
	Exclude          []akn.NetWithPortRange
	InterfaceIndexes []int
	Direction        Direction
}

func (filter *Filter) writeStringForFilters(sb *strings.Builder) {
	if filter.Direction != "" {
		sb.WriteString("\ndirection: ")
		sb.WriteString(string(filter.Direction))
	}
	sb.WriteString("\nto/from:\n")
	for _, inc := range filter.Include {
		sb.WriteString(" ")
		sb.WriteString(inc.String())
		sb.WriteString("\n")
	}
	if len(filter.Exclude) > 0 {
		sb.WriteString("but not from/to:\n")
		for _, exc := range filter.Exclude {
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
