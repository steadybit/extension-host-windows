// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"testing"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
)

func groupMembers(desc action_kit_api.ActionDescription) map[string][]string {
	members := map[string][]string{}
	for _, p := range desc.Parameters {
		if p.Group != nil {
			members[*p.Group] = append(members[*p.Group], p.Name)
		}
	}
	return members
}

func definedGroups(desc action_kit_api.ActionDescription) []string {
	var names []string
	if desc.ParameterGroups != nil {
		for _, g := range *desc.ParameterGroups {
			names = append(names, g.Name)
		}
	}
	return names
}

// orderOf is the order platforms sort a parameter by: a missing order counts as 0.
func orderOf(p action_kit_api.ActionParameter) int {
	if p.Order == nil {
		return 0
	}
	return *p.Order
}

func TestParameterGroups(t *testing.T) {
	trafficFilter := []string{"hostname", "ip", "port", "excludeHostname", "excludeIp", "networkInterface"}

	tests := []struct {
		name    string
		desc    action_kit_api.ActionDescription
		groups  map[string][]string
		defined []string
	}{
		{"blackhole", NewNetworkBlackholeContainerAction().Describe(),
			map[string][]string{groupTrafficFilter: append(append([]string{}, trafficFilter...), "blockLocalhost")}, []string{groupTrafficFilter}},
		{"delay", NewNetworkDelayContainerAction().Describe(),
			map[string][]string{groupTrafficFilter: trafficFilter}, []string{groupTrafficFilter}},
		{"loss", NewNetworkPackageLossContainerAction().Describe(),
			map[string][]string{groupTrafficFilter: trafficFilter}, []string{groupTrafficFilter}},
		{"corrupt", NewNetworkCorruptPackagesContainerAction().Describe(),
			map[string][]string{groupTrafficFilter: trafficFilter}, []string{groupTrafficFilter}},
		// The filter of Limit Outgoing Bandwidth is shown directly under its own heading.
		{"bandwidth", NewNetworkLimitBandwidthContainerAction().Describe(),
			map[string][]string{}, nil},
		{"fillDisk", NewFillDiskAction().Describe(),
			map[string][]string{groupFillBehavior: {"method", "blocksize"}}, nil},
		{"stopProcess", NewStopProcessAction().Describe(),
			map[string][]string{action_kit_api.ParameterGroupAdditionalSettings: {"delay"}}, nil},
		{"timeTravel", NewTimetravelAction().Describe(),
			map[string][]string{action_kit_api.ParameterGroupAdditionalSettings: {"disableNtp"}}, nil},
		{"stressIo", NewStressIoAction().Describe(),
			map[string][]string{action_kit_api.ParameterGroupAdditionalSettings: {"disableSwHwCaching"}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.groups, groupMembers(tt.desc))
			assert.Equal(t, tt.defined, definedGroups(tt.desc))

			for _, p := range tt.desc.Parameters {
				advanced := p.Advanced != nil && *p.Advanced
				// No advanced parameter is left alone in "Additional Settings" on platforms with groups.
				if advanced {
					assert.NotNil(t, p.Group, "parameter %s", p.Name)
				}
				// Platforms without groups keep the filters in "Additional Settings"; the network interface was
				// always visible and stays so there.
				if p.Group != nil && *p.Group == groupTrafficFilter && p.Name != "networkInterface" {
					assert.True(t, advanced, "parameter %s", p.Name)
				}
			}

			// Platforms sort parameters by order and show a group where its first parameter is: an explicit
			// "Additional Settings" group must therefore come after every other parameter, as the fallback does.
			for _, p := range tt.desc.Parameters {
				if p.Group == nil || *p.Group != action_kit_api.ParameterGroupAdditionalSettings {
					continue
				}
				for _, other := range tt.desc.Parameters {
					if other.Group == nil || *other.Group != action_kit_api.ParameterGroupAdditionalSettings {
						assert.Less(t, orderOf(other), orderOf(p), "%s comes before Additional Settings", other.Name)
					}
				}
			}
		})
	}
}
