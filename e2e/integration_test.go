// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package e2e

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_test/validate"
	"github.com/steadybit/extension-host-windows/exthostwindows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"strconv"
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
		}, {
			Name: "network delay",
			Test: testNetworkDelay,
		},
	})
}

func validateDiscovery(t *testing.T, _ Environment, e Extension) {
	assert.NoError(t, validate.ValidateEndpointReferences("/", e.Client()))
}

func testDiscovery(t *testing.T, _ Environment, e Extension) {
	log.Info().Msg("Starting testDiscovery")
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	target, err := e.PollForTarget(ctx, exthostwindows.BaseActionID+".host", func(target discovery_kit_api.Target) bool {
		log.Debug().Msgf("targetHost: %v", target.Attributes["host.hostname"])
		return HasAttribute(target, "host.hostname")
	})

	require.NoError(t, err)
	assert.Equal(t, target.TargetType, exthostwindows.BaseActionID+".host")
	assert.NotContains(t, target.Attributes, "host.nic")
}

func testStopProcess(t *testing.T, l Environment, e Extension) {
	ctx := t.Context()
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

	cancel, err := l.StartAndAwaitProcess(ctx, "PING.EXE", awaitLog(" 127.0.0.1: "), "-n", "30", "127.0.0.1")
	require.NoError(t, err)

	processes := l.FindProcessIds(t.Context(), "PING.EXE")
	require.NotEmpty(t, processes)
	t.Cleanup(cancel)

	action, err := e.RunAction(exthostwindows.BaseActionID+".stop-process", l.BuildTarget(ctx), config, nil)
	require.NoError(t, err)

	timeout := time.Now().Add(1 * time.Second)
	for time.Now().Before(timeout) {
		pids := l.FindProcessIds(ctx, "PING.EXE")
		if len(pids) == 0 {
			break
		}
	}
	require.NoError(t, action.Cancel())
}

func testNetworkDelay(t *testing.T, l Environment, e Extension) {
	port, err := FindAvailablePorts(8080, 8800, 2)
	require.NoError(t, err)
	netperf := NewHttpNetperf(port)
	err = netperf.Deploy(t.Context(), l)
	require.NoError(t, err)
	defer func() { _ = netperf.Delete() }()

	restrictedEndpointsCount := 16
	tests := []struct {
		name                string
		ip                  []string
		hostname            []string
		port                []string
		interfaces          []string
		restrictedEndpoints []action_kit_api.RestrictedEndpoint
		wantedDelay         bool
	}{
		{
			name:                "should delay all traffic",
			restrictedEndpoints: generateRestrictedEndpoints(restrictedEndpointsCount),
			wantedDelay:         true,
		},
		{
			name:                "should delay only port traffic",
			port:                []string{strconv.Itoa(port)},
			restrictedEndpoints: generateRestrictedEndpoints(restrictedEndpointsCount),
			wantedDelay:         true,
		},
		{
			name:                "should not delay other port traffic",
			port:                []string{strconv.Itoa(port + 1)},
			restrictedEndpoints: generateRestrictedEndpoints(restrictedEndpointsCount),
			wantedDelay:         false,
		},
		{
			name:                "should delay only traffic for netperf",
			ip:                  []string{netperf.Ip},
			restrictedEndpoints: generateRestrictedEndpoints(restrictedEndpointsCount),
			wantedDelay:         true,
		},
		{
			name:                "should delay only traffic for netperf using cidr",
			ip:                  []string{fmt.Sprintf("%s/32", netperf.Ip)},
			restrictedEndpoints: generateRestrictedEndpoints(restrictedEndpointsCount),
			wantedDelay:         true,
		},
	}

	for _, tt := range tests {
		config := struct {
			Duration     int      `json:"duration"`
			Delay        int      `json:"networkDelay"`
			Jitter       bool     `json:"networkDelayJitter"`
			Ip           []string `json:"ip"`
			Hostname     []string `json:"hostname"`
			Port         []string `json:"port"`
			NetInterface []string `json:"networkInterface"`
		}{
			Duration:     10000,
			Delay:        100,
			Jitter:       false,
			Ip:           tt.ip,
			Hostname:     tt.hostname,
			Port:         tt.port,
			NetInterface: tt.interfaces,
		}

		restrictedEndpoints := tt.restrictedEndpoints
		executionContext := &action_kit_api.ExecutionContext{RestrictedEndpoints: &restrictedEndpoints}

		unaffectedLatency, err := netperf.MeasureLatency()
		require.NoError(t, err)
		log.Info().Msgf("Unaffected latency: %v", unaffectedLatency)

		t.Run(tt.name, func(t *testing.T) {
			action, err := e.RunAction(exthostwindows.BaseActionID+".network_delay", l.BuildTarget(t.Context()), config, executionContext)
			defer func() { _ = action.Cancel() }()
			require.NoError(t, err)

			if tt.wantedDelay {

				// TODO: WinDivert considers all local communication as outgoing and delays it by 2x
				delayDuration := time.Duration(config.Delay) * time.Millisecond * 2

				netperf.AssertLatency(t, unaffectedLatency+delayDuration*90/100, unaffectedLatency+delayDuration*350/100)
			} else {
				netperf.AssertLatency(t, 0, unaffectedLatency*120/100)
			}
			require.NoError(t, action.Cancel())
			netperf.AssertLatency(t, 0, unaffectedLatency*120/100)
		})
	}
}

func generateRestrictedEndpoints(count int) []action_kit_api.RestrictedEndpoint {
	address := net.IPv4(192, 168, 0, 1)
	result := make([]action_kit_api.RestrictedEndpoint, 0, count)

	for i := 0; i < count; i++ {
		result = append(result, action_kit_api.RestrictedEndpoint{
			Cidr:    fmt.Sprintf("%s/32", address.String()),
			PortMin: 8085,
			PortMax: 8086,
		})
		incrementIP(address, len(address)-1)
	}

	return result
}

func incrementIP(a net.IP, idx int) {
	if idx < 0 || idx >= len(a) {
		return
	}

	if idx == len(a)-1 && a[idx] >= 254 {
		a[idx] = 1
		incrementIP(a, idx-1)
	} else if a[idx] == 255 {
		a[idx] = 0
		incrementIP(a, idx-1)
	} else {
		a[idx]++
	}
}
