// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import "github.com/steadybit/action-kit/go/action_kit_api/v2"

const (
	groupTrafficFilter = "Traffic Filter"
)

var (
	// WinDivert captures loopback traffic too, so unchanged filters include localhost on Windows.
	trafficFilterGroup = action_kit_api.ParameterGroup{
		Name:        groupTrafficFilter,
		Description: "Leave unchanged to affect all traffic including localhost.",
	}
)
