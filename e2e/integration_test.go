// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package e2e

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_test/validate"
	"github.com/steadybit/extension-host-windows/exthostwindows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestWithLocalhost(t *testing.T) {
	environment := newLocalEnvironment()
	extFactory := LocalExtensionFactory{
		Name: "extension-host-windows",
		Port: 8085,
		ExtraEnv: func() map[string]string {
			return map[string]string{
				"STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_HOST": "host.nic",
				"STEADYBIT_EXTENSION_LOGGING_LEVEL":                      "trace",
			}
		},
	}

	WithEnvironment(t, environment, &extFactory, []WithTestCase{
		{
			Name: "validate discovery",
			Test: validateDiscovery,
		}, {
			Name: "target discovery",
			Test: testDiscovery,
		}, {
			Name: "stop process",
			Test: testStopProcess,
		},
	})
}

func validateDiscovery(t *testing.T, _ Environment, e Extension) {
	assert.NoError(t, validate.ValidateEndpointReferences("/", e.Client()))
}

func testDiscovery(t *testing.T, _ Environment, e Extension) {
	log.Info().Msg("Starting testDiscovery")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	target, err := PollForTarget(ctx, e, exthostwindows.BaseActionID+".host", func(target discovery_kit_api.Target) bool {
		log.Debug().Msgf("targetHost: %v", target.Attributes["host.hostname"])
		return HasAttribute(target, "host.hostname")
	})

	require.NoError(t, err)
	assert.Equal(t, target.TargetType, exthostwindows.BaseActionID+".host")
	assert.NotContains(t, target.Attributes, "host.nic")
}

func testStopProcess(t *testing.T, l Environment, e Extension) {
	config := struct {
		Duration int    `json:"duration"`
		Graceful bool   `json:"graceful"`
		Process  string `json:"process"`
		Delay    int    `json:"delay"`
	}{
		Duration: 10000,
		Graceful: false,
		Process:  "PING.EXE",
		Delay:    1,
	}

	cancel := startProcess(t, l, "PING.EXE", " 127.0.0.1: ", "-n", "30", "127.0.0.1")
	assertProcessRunning(t, l, "PING.EXE")
	t.Cleanup(cancel)

	action, err := e.RunAction(exthostwindows.BaseActionID+".stop-process", l.BuildTarget(), config, nil)
	require.NoError(t, err)

	timeout := time.Now().Add(1 * time.Second)
	for time.Now().Before(timeout) {
		pids := l.FindProcessIds("PING.EXE")
		if len(pids) == 0 {
			break
		}
	}
	require.NoError(t, action.Cancel())
}

func startProcess(t *testing.T, l Environment, cmd string, awaitOutput string, parameters ...string) func() {
	cancel, err := l.StartProcess(cmd, awaitLogFn(awaitOutput), parameters...)
	require.NoError(t, err)
	return cancel
}

func assertProcessRunning(t *testing.T, l Environment, name string) {
	processes := l.FindProcessIds(name)
	require.NotEmpty(t, processes)
}

func assertProcessNotRunning(t *testing.T, l Environment, name string) {
	processes := l.FindProcessIds(name)
	require.Empty(t, processes)
}
