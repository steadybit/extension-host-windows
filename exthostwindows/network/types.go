// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"fmt"
	"strings"
	"time"

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
	// QoSPolicies returns the QoS policies for attacks implemented with Windows QoS, nil otherwise.
	QoSPolicies() []QosPolicy
	// WinDivert returns the wdna.exe run for attacks implemented with WinDivert, nil otherwise.
	WinDivert() *WinDivertOpts
	String() string
}

type WinDivertMode string

const (
	WinDivertModeDelay   WinDivertMode = "delay"
	WinDivertModeDrop    WinDivertMode = "drop"
	WinDivertModeCorrupt WinDivertMode = "corrupt"
)

type WinDivertOpts struct {
	Filter   Filter
	Mode     WinDivertMode
	Duration time.Duration
	// Delay and Jitter apply to WinDivertModeDelay, Percentage to WinDivertModeDrop and WinDivertModeCorrupt.
	Delay      time.Duration
	Jitter     bool
	Percentage uint
	FlushDNS   bool
}

type Direction string

var (
	DirectionIncoming Direction = "Incoming"
	DirectionOutgoing Direction = "Outgoing"
	DirectionAll      Direction = "All"
)

// Interface is a network interface the attack is restricted to. WinDivert filters on the index, the name is what users
// configured and recognize.
type Interface struct {
	Index int
	Name  string
}

type Filter struct {
	Include    []akn.NetWithPortRange
	Exclude    []akn.NetWithPortRange
	Interfaces []Interface
	Direction  Direction
	// ExcludeLoopback leaves loopback traffic (127.0.0.0/8, ::1) untouched.
	// WinDivert captures loopback packets, so they are affected unless excluded.
	ExcludeLoopback bool
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
	if len(filter.Interfaces) > 0 {
		sb.WriteString("on interfaces:\n")
		for _, iface := range filter.Interfaces {
			fmt.Fprintf(sb, " %-5d%s\n", iface.Index, iface.Name)
		}
	}
}
