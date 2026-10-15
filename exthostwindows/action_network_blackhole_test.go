// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"testing"

	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/steadybit/extension-host-windows/config"
	"github.com/stretchr/testify/assert"
)

func Test_localhostExcludesForOwnPorts(t *testing.T) {
	config.Config.Port = 8085
	config.Config.HealthPort = 8081

	var got []string
	for _, nwp := range localhostExcludesForOwnPorts() {
		got = append(got, nwp.String())
	}

	assert.ElementsMatch(t, []string{
		"127.0.0.0/8 8085 # ext. port (localhost)",
		"127.0.0.0/8 8081 # ext. port (localhost)",
		"::1/128 8085 # ext. port (localhost)",
		"::1/128 8081 # ext. port (localhost)",
	}, got)
}

func Test_blackholeDescription_hasBlockLocalhost(t *testing.T) {
	var found bool
	for _, p := range getNetworkBlackholeDescription().Parameters {
		if p.Name == "blockLocalhost" {
			found = true
			assert.Equal(t, "true", *p.DefaultValue)
			assert.True(t, *p.Advanced)
		}
	}
	assert.True(t, found, "blockLocalhost parameter missing")
}

func Test_selectsLoopbackInterface(t *testing.T) {
	loopback := akn.GetLoopbackNetworkInterfaces()
	if len(loopback) == 0 {
		t.Skip("no loopback interface found")
	}
	assert.True(t, selectsLoopbackInterface([]string{loopback[0]}))
	assert.True(t, selectsLoopbackInterface([]string{"not-an-interface", loopback[0]}))
	assert.False(t, selectsLoopbackInterface([]string{"not-an-interface"}))
	assert.False(t, selectsLoopbackInterface(nil))
}

func Test_blockLocalhostFromConfig(t *testing.T) {
	assert.True(t, blockLocalhostFromConfig(map[string]any{}), "missing parameter keeps the previous behavior")
	assert.True(t, blockLocalhostFromConfig(map[string]any{"blockLocalhost": nil}))
	assert.True(t, blockLocalhostFromConfig(map[string]any{"blockLocalhost": true}))
	assert.True(t, blockLocalhostFromConfig(map[string]any{"blockLocalhost": "true"}))
	assert.False(t, blockLocalhostFromConfig(map[string]any{"blockLocalhost": false}))
	assert.False(t, blockLocalhostFromConfig(map[string]any{"blockLocalhost": "false"}))
}
