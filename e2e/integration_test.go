// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package e2e

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_test/validate"
	"github.com/steadybit/extension-host-windows/exthostwindows"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"strconv"
	"testing"
	"time"
)

var (
	defaultExecutionContext = &action_kit_api.ExecutionContext{
		RestrictedEndpoints: extutil.Ptr([]action_kit_api.RestrictedEndpoint{
			{
				Name:    "extension",
				Url:     "localhost",
				Cidr:    "0.0.0.0/0",
				PortMin: 8085,
				PortMax: 8085,
			},
		}),
	}
	steadybitCIDRs = getCIDRsFor("steadybit.com", 16)
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
		}, {
			Name: "network blackhole",
			Test: testNetworkBlackhole,
		}, {
			Name: "network block dns",
			Test: testNetworkBlockDns,
		}, {
			Name: "network limit bandwidth",
			Test: testNetworkLimitBandwidth,
		}, {
			Name: "network package loss",
			Test: testNetworkPackageLoss,
		}, {
			Name: "network package corruption",
			Test: testNetworkPackageCorruption,
		}, {
			Name: "two simultaneous network attacks should error",
			Test: testTwoNetworkAttacks,
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
		{
			name:        "should delay all interfaces",
			interfaces:  network.GetOwnNetworkInterfaces(),
			wantedDelay: true,
		},
		{
			name:        "should delay none loopback interfaces",
			interfaces:  network.GetNonLoopbackNetworkInterfaces(),
			wantedDelay: false,
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
		unaffectedLatency = max(unaffectedLatency, 1*time.Millisecond)
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

func testNetworkBlackhole(t *testing.T, l Environment, e Extension) {
	t.Skip("Only works with activated Windows firewall")

	port, err := FindAvailablePorts(8080, 8800, 2)
	require.NoError(t, err)
	netperf := NewHttpNetperf(port)
	err = netperf.Deploy(t.Context(), l)
	require.NoError(t, err)
	defer func() { _ = netperf.Delete() }()

	tests := []struct {
		name             string
		ip               []string
		hostname         []string
		port             []string
		wantedReachesUrl bool
	}{
		{
			name:             "should blackhole all traffic",
			wantedReachesUrl: false,
		},
		{
			name:             "should blackhole only port 8080 traffic",
			port:             []string{"8080"},
			wantedReachesUrl: true,
		},
		{
			name:             "should blackhole only port 80, 443 traffic",
			port:             []string{"80", "443"},
			wantedReachesUrl: false,
		},
		{
			name:             "should blackhole only traffic for steadybit.com hostname",
			hostname:         []string{"steadybit.com"},
			wantedReachesUrl: false,
		},
		{
			name:             "should blackhole only traffic for steadybit.com cider",
			ip:               steadybitCIDRs,
			wantedReachesUrl: false,
		},
	}

	for _, tt := range tests {
		config := struct {
			Duration int      `json:"duration"`
			Ip       []string `json:"ip"`
			Hostname []string `json:"hostname"`
			Port     []string `json:"port"`
		}{
			Duration: 30000,
			Ip:       tt.ip,
			Hostname: tt.hostname,
			Port:     tt.port,
		}

		t.Run(tt.name, func(t *testing.T) {
			require.True(t, netperf.CanReach("steadybit.com"))

			action, err := e.RunAction(exthostwindows.BaseActionID+".network_blackhole", l.BuildTarget(t.Context()), config, defaultExecutionContext)
			defer func() { _ = action.Cancel() }()
			require.NoError(t, err)

			assert.Equal(t, tt.wantedReachesUrl, netperf.CanReach("steadybit.com"))

			require.NoError(t, action.Cancel())
			require.True(t, netperf.CanReach("steadybit.com"))
		})
	}
}

func testNetworkBlockDns(t *testing.T, l Environment, e Extension) {
	t.Skip("Only works with activated Windows firewall")

	port, err := FindAvailablePorts(8080, 8800, 2)
	require.NoError(t, err)
	netperf := NewHttpNetperf(port)
	err = netperf.Deploy(t.Context(), l)
	require.NoError(t, err)
	defer func() { _ = netperf.Delete() }()

	tests := []struct {
		name             string
		dnsPort          uint
		wantedReachesUrl bool
	}{
		{
			name:             "should block dns traffic",
			dnsPort:          53,
			wantedReachesUrl: false,
		},
		{
			name:             "should block dns traffic on port 5353",
			dnsPort:          5353,
			wantedReachesUrl: true,
		},
	}

	for _, tt := range tests {
		config := struct {
			Duration int  `json:"duration"`
			DnsPort  uint `json:"dnsPort"`
		}{
			Duration: 10000,
			DnsPort:  tt.dnsPort,
		}

		t.Run(tt.name, func(t *testing.T) {
			// Use different dns names to make sure that they are not cached.
			require.True(t, netperf.CanReach("steadybit.com"))

			action, err := e.RunAction(exthostwindows.BaseActionID+".network_block_dns", l.BuildTarget(t.Context()), config, defaultExecutionContext)
			defer func() { _ = action.Cancel() }()
			require.NoError(t, err)

			assert.Equal(t, tt.wantedReachesUrl, netperf.CanReach("chaosmesh.com"))

			require.NoError(t, action.Cancel())
			require.True(t, netperf.CanReach("google.com"))
		})
	}
}

func testNetworkLimitBandwidth(t *testing.T, l Environment, e Extension) {
	t.Skip("Limit bandwidth tests use Windows QoS, which does not apply to local loopback interfaces. Test manually.")

	// Start the iperf server on a remote system or use an online one from https://iperf.fr/iperf-servers.php
	// Due to the network dependency these tests are very flaky.
	port := 5200
	iperf := NewIperf("iperf3.moji.fr", port)
	err := iperf.Install()
	require.NoError(t, err)

	unlimited := 0.0
	// the iperf servers seem to be pretty unstable
	measured := Retry(t, 3, 1*time.Second, func(r *R) {
		unlimited, err = iperf.MeasureBandwidth(t.Context(), l)
		if err != nil {
			log.Error().Err(err).Msg("measure bandwidth failed")
			r.Failed = true
		}
	})
	require.True(t, measured)
	limited := unlimited / 3
	log.Info().Msgf("limited bandwidth: %v", limited)

	dig := network.HostnameResolver{}
	iperfIps, err := dig.Resolve(t.Context(), iperf.Ip)
	require.NoError(t, err)
	iperfIp := iperfIps[0]
	iperfNet := net.IPNet{IP: iperfIp, Mask: net.CIDRMask(24, 32)}

	log.Info().Str("ip", iperfIp.String()).Str("network", iperfNet.String()).Msg("iperf network addresses")

	tests := []struct {
		name        string
		hostname    string
		ip          string
		port        string
		wantedLimit bool
	}{
		{
			name:        "should limit bandwidth on all traffic",
			wantedLimit: true,
		},
		{
			name:        fmt.Sprintf("should limit bandwidth only on port %d traffic", port),
			port:        strconv.Itoa(port),
			wantedLimit: true,
		},
		{
			name:        "should limit bandwidth only on port 80 traffic",
			port:        "80",
			wantedLimit: false,
		},
		{
			name:        "should limit bandwidth for iperf server hostname",
			hostname:    iperf.Ip,
			wantedLimit: true,
		},
		{
			name:        "should limit bandwidth for iperf server ip",
			ip:          iperfIp.String(),
			wantedLimit: true,
		},
		{
			name:        "should limit bandwidth for iperf server cider",
			ip:          iperfNet.String(),
			wantedLimit: true,
		},
	}

	for _, tt := range tests {
		config := struct {
			Duration  int    `json:"duration"`
			Bandwidth string `json:"bandwidth"`
			Hostname  string `json:"hostname"`
			Ip        string `json:"ip"`
			Port      string `json:"port"`
		}{
			Duration:  30000,
			Bandwidth: fmt.Sprintf("%dbit", int(limited*1_000_000)),
			Hostname:  tt.hostname,
			Ip:        tt.ip,
			Port:      tt.port,
		}

		t.Run(tt.name, func(t *testing.T) {
			action, err := e.RunAction(exthostwindows.BaseActionID+".network_bandwidth", l.BuildTarget(t.Context()), config, defaultExecutionContext)
			defer func() { _ = action.Cancel() }()
			require.NoError(t, err)

			if tt.wantedLimit {
				iperf.AssertBandwidth(t, t.Context(), l, limited*0.95, limited*1.05)
			} else {
				iperf.AssertBandwidth(t, t.Context(), l, unlimited*0.95, unlimited*1.05)
			}
			require.NoError(t, action.Cancel())
			iperf.AssertBandwidth(t, t.Context(), l, unlimited*0.95, unlimited*1.05)
		})
	}
}

func testNetworkPackageLoss(t *testing.T, l Environment, e Extension) {
	port, err := FindAvailablePort(5002, 5100)
	require.NoError(t, err)
	iperf := NewIperf("127.0.0.1", port)
	err = iperf.Deploy(t.Context(), l)
	require.NoError(t, err)
	defer func() { _ = iperf.Delete() }()

	tests := []struct {
		name       string
		ip         []string
		port       []string
		wantedLoss bool
	}{
		{
			name:       "should loose packages on all traffic",
			wantedLoss: true,
		},
		{
			name:       "should loose packages only on port 5001 traffic",
			port:       []string{"5001"},
			wantedLoss: true,
		},
		{
			name:       "should loose packages only on port 80 traffic",
			port:       []string{"80"},
			wantedLoss: false,
		},
		{
			name:       "should loose packages only traffic for iperf server",
			ip:         []string{iperf.Ip},
			wantedLoss: true,
		},
	}

	for _, tt := range tests {
		config := struct {
			Duration   int      `json:"duration"`
			Percentage int      `json:"percentage"`
			Ip         []string `json:"ip"`
			Port       []string `json:"port"`
		}{
			Duration:   50000,
			Percentage: 10,
			Ip:         tt.ip,
			Port:       tt.port,
		}

		t.Run(tt.name, func(t *testing.T) {
			action, err := e.RunAction(exthostwindows.BaseActionID+".network_package_loss", l.BuildTarget(t.Context()), config, defaultExecutionContext)
			defer func() { _ = action.Cancel() }()
			require.NoError(t, err)

			if tt.wantedLoss {
				iperf.AssertPackageLoss(t, t.Context(), l, float64(config.Percentage)*0.7, float64(config.Percentage)*1.4)
			} else {
				iperf.AssertPackageLoss(t, t.Context(), l, 0, 5)
			}
			require.NoError(t, action.Cancel())

			iperf.AssertPackageLoss(t, t.Context(), l, 0, 5)
		})
	}
}

func testNetworkPackageCorruption(t *testing.T, l Environment, e Extension) {
	t.Skip("Iperf does not seem to track lost/corrupted packages on Windows. Deactivated for now.")

	ctx := t.Context()
	port, err := FindAvailablePort(5002, 5100)
	require.NoError(t, err)
	iperf := NewIperf("127.0.0.1", port)
	err = iperf.Deploy(ctx, l)
	require.NoError(t, err)
	defer func() { _ = iperf.Delete() }()

	tests := []struct {
		name             string
		ip               []string
		hostname         []string
		port             []string
		interfaces       []string
		wantedCorruption bool
	}{
		{
			name:             "should corrupt packages on all traffic",
			wantedCorruption: true,
		},
		{
			name:             "should corrupt packages on server port traffic",
			port:             []string{strconv.Itoa(iperf.Port)},
			wantedCorruption: true,
		},
		{
			name:             "should corrupt packages on port 80 traffic",
			port:             []string{"80"},
			wantedCorruption: false,
		},
		{
			name:             "should corrupt packages on server ip",
			ip:               []string{iperf.Ip},
			wantedCorruption: true,
		},
		{
			name:             "should corrupt packages on other ip",
			ip:               []string{"1.1.1.1"},
			wantedCorruption: false,
		},
		{
			name:             "should corrupt packages on all interfaces",
			interfaces:       network.GetOwnNetworkInterfaces(),
			wantedCorruption: true,
		},
		{
			name:             "should corrupt packages on none loopback interfaces",
			interfaces:       network.GetNonLoopbackNetworkInterfaces(),
			wantedCorruption: false,
		},
	}

	for _, tt := range tests {
		config := struct {
			Duration     int      `json:"duration"`
			Corruption   int      `json:"networkCorruption"`
			Ip           []string `json:"ip"`
			Hostname     []string `json:"hostname"`
			Port         []string `json:"port"`
			NetInterface []string `json:"networkInterface"`
		}{
			Duration:     60000,
			Corruption:   10,
			Ip:           tt.ip,
			Hostname:     tt.hostname,
			Port:         tt.port,
			NetInterface: tt.interfaces,
		}

		t.Run(tt.name, func(t *testing.T) {
			Retry(t, 3, 1*time.Second, func(r *R) {
				action, err := e.RunAction(exthostwindows.BaseActionID+".network_package_corruption", l.BuildTarget(t.Context()), config, defaultExecutionContext)
				defer func() { _ = action.Cancel() }()
				if err != nil {
					r.Failed = true
				}

				if tt.wantedCorruption {
					packageLossResult := iperf.AssertPackageLossWithRetry(t.Context(), l, float64(config.Corruption)*0.7, float64(config.Corruption)*1.3, 8)
					if !packageLossResult {
						r.Failed = true
					}
				} else {
					packageLossResult := iperf.AssertPackageLossWithRetry(t.Context(), l, 0, 5, 8)
					if !packageLossResult {
						r.Failed = true
					}
				}
				require.NoError(t, action.Cancel())

				packageLossResult := iperf.AssertPackageLossWithRetry(t.Context(), l, 0, 5, 8)
				if !packageLossResult {
					r.Failed = true
				}
			})
		})
	}
}

func testTwoNetworkAttacks(t *testing.T, l Environment, e Extension) {
	configDelay := struct {
		Duration int `json:"duration"`
		Delay    int `json:"networkDelay"`
	}{
		Duration: 10000,
		Delay:    200,
	}
	actionDelay, err := e.RunAction(exthostwindows.BaseActionID+".network_delay", l.BuildTarget(t.Context()), configDelay, defaultExecutionContext)
	defer func() { _ = actionDelay.Cancel() }()
	require.NoError(t, err)

	configLimit := struct {
		Duration  int    `json:"duration"`
		Bandwidth string `json:"bandwidth"`
	}{
		Duration:  10000,
		Bandwidth: "200mbit",
	}
	actionLimit, err2 := e.RunAction(exthostwindows.BaseActionID+".network_bandwidth", l.BuildTarget(t.Context()), configLimit, defaultExecutionContext)
	defer func() { _ = actionLimit.Cancel() }()
	require.ErrorContains(t, err2, "running multiple network attacks at the same time is not supported")
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
