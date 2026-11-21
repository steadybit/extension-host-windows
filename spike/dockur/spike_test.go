// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

//go:build spike

// Package dockur runs attacks of the extension in a Windows VM that dockur/windows starts on a Linux runner, see
// .github/workflows/spike-dockur-windows.yml.
package dockur

import (
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	aclient "github.com/steadybit/action-kit/go/action_kit_test/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const actionPrefix = "com.steadybit.extension_host_windows."

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var (
	extensionUrl = env("SPIKE_EXTENSION_URL", "http://127.0.0.1:8085")
	probeUrl     = env("SPIKE_PROBE_URL", "http://127.0.0.1:8080")
	probeClient  = &http.Client{Timeout: 5 * time.Second}
)

func actions() aclient.ActionAPI {
	return aclient.NewActionClient("/", resty.NewWithClient(&http.Client{Timeout: 30 * time.Second}).SetBaseURL(extensionUrl))
}

func target(t *testing.T) *action_kit_api.Target {
	hostname := os.Getenv("SPIKE_HOSTNAME")
	require.NotEmpty(t, hostname, "SPIKE_HOSTNAME must name the guest")
	return &action_kit_api.Target{Attributes: map[string][]string{"host.hostname": {hostname}}}
}

// probeLatency returns the median time of requests to the probe in the guest, or an error when one fails.
func probeLatency(count int) (time.Duration, error) {
	var durations []time.Duration
	for range count {
		start := time.Now()
		resp, err := probeClient.Get(probeUrl)
		if err != nil {
			return 0, err
		}
		_ = resp.Body.Close()
		durations = append(durations, time.Since(start))
	}
	slices.Sort(durations)
	return durations[len(durations)/2], nil
}

func TestNetworkAttacks(t *testing.T) {
	baseline, err := probeLatency(20)
	require.NoError(t, err)
	t.Logf("baseline latency %v", baseline)
	executionContext := &action_kit_api.ExecutionContext{}

	t.Run("delay", func(t *testing.T) {
		config := map[string]any{"duration": 60000, "networkDelay": 200, "port": []string{"8080"}}
		action, err := actions().RunAction(actionPrefix+"network_delay", target(t), config, executionContext)
		defer func() { _ = action.Cancel() }()
		require.NoError(t, err)

		delayed, err := probeLatency(10)
		require.NoError(t, err)
		t.Logf("delayed latency %v", delayed)
		// Only the responses leave the guest, so they alone are delayed.
		assert.GreaterOrEqual(t, delayed, baseline+180*time.Millisecond)

		require.NoError(t, action.Cancel())
		after, err := probeLatency(10)
		require.NoError(t, err)
		assert.Less(t, after, baseline+50*time.Millisecond)
	})

	t.Run("blackhole", func(t *testing.T) {
		config := map[string]any{"duration": 60000, "port": []string{"8080"}}
		action, err := actions().RunAction(actionPrefix+"network_blackhole", target(t), config, executionContext)
		defer func() { _ = action.Cancel() }()
		require.NoError(t, err)

		_, err = probeLatency(1)
		assert.Error(t, err, "the probe must be unreachable")

		require.NoError(t, action.Cancel())
		require.Eventually(t, func() bool { _, err := probeLatency(1); return err == nil }, 20*time.Second, 500*time.Millisecond)
	})
}

// TestShutdown shuts the guest down; the workflow then checks that the VM, and with it the container, stopped.
func TestShutdown(t *testing.T) {
	action, err := actions().RunAction(actionPrefix+"shutdown", target(t), map[string]any{"reboot": false}, nil)
	if err != nil {
		// The extension may not answer anymore once the shutdown began.
		t.Logf("shutdown action returned %v", err)
		return
	}
	_ = action.Cancel()
}
