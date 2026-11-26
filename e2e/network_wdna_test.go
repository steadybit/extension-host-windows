// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package e2e

import (
	"strconv"
	"testing"
	"time"

	"github.com/steadybit/extension-host-windows/exthostwindows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The attacks below affect the UDP echo port only. On loopback both the request and the reply are outgoing, so each
// datagram passes the attack twice.

func testNetworkPackageLoss(t *testing.T, l Environment, e Extension) {
	port := startUdpEcho(t)
	require.GreaterOrEqual(t, exchangeUdp(t, port, 400, 400, 2*time.Second, nil).intactShare(), 0.99)

	config := map[string]any{"duration": 60000, "percentage": 50, "port": []string{strconv.Itoa(port)}}
	action, err := e.RunAction(exthostwindows.BaseActionID+".network_package_loss", l.BuildTarget(t.Context()), config, defaultExecutionContext)
	defer func() { _ = action.Cancel() }()
	require.NoError(t, err)

	// Half of the requests and half of the replies are lost: a quarter of the datagrams come back.
	assert.InDelta(t, 0.25, exchangeUdp(t, port, 400, 400, 2*time.Second, nil).intactShare(), 0.12)

	require.NoError(t, action.Cancel())
	assert.GreaterOrEqual(t, exchangeUdp(t, port, 400, 400, 2*time.Second, nil).intactShare(), 0.99)
}

func testNetworkPackageCorruption(t *testing.T, l Environment, e Extension) {
	port := startUdpEcho(t)
	config := map[string]any{"duration": 60000, "networkCorruption": 50, "port": []string{strconv.Itoa(port)}}
	action, err := e.RunAction(exthostwindows.BaseActionID+".network_package_corruption", l.BuildTarget(t.Context()), config, defaultExecutionContext)
	defer func() { _ = action.Cancel() }()
	require.NoError(t, err)

	x := exchangeUdp(t, port, 400, 400, 2*time.Second, nil)
	// A quarter of the datagrams pass both ways uncorrupted. A corrupted one keeps the checksum of the intact datagram,
	// so it is discarded unless the stack skips the check on loopback; it never arrives as intact.
	assert.InDelta(t, 0.25, x.intactShare(), 0.12)

	require.NoError(t, action.Cancel())
	assert.GreaterOrEqual(t, exchangeUdp(t, port, 400, 400, 2*time.Second, nil).intactShare(), 0.99)
}

// testNetworkDelayUnderLoad keeps 2000 datagrams per second delayed for 30 s, the kind of load in which wdna v0.0.15
// stopped forwarding traffic after some seconds.
func testNetworkDelayUnderLoad(t *testing.T, l Environment, e Extension) {
	port := startUdpEcho(t)
	config := map[string]any{"duration": 120000, "networkDelay": 20, "port": []string{strconv.Itoa(port)}}
	action, err := e.RunAction(exthostwindows.BaseActionID+".network_delay", l.BuildTarget(t.Context()), config, defaultExecutionContext)
	defer func() { _ = action.Cancel() }()
	require.NoError(t, err)

	x := exchangeUdp(t, port, 60000, 2000, 5*time.Second, nil)
	assert.GreaterOrEqual(t, x.intactShare(), 0.99, "datagrams must not be lost under load")
	// Delayed twice by 20 ms, see above.
	assert.GreaterOrEqual(t, x.percentile(0.5), 38*time.Millisecond)
	assert.Less(t, x.percentile(0.5), 100*time.Millisecond)
	assert.Less(t, x.percentile(0.99), 500*time.Millisecond, "the delay must not grow under load")
}

// testNetworkDelayReleasedOnStop stops a long delay while datagrams are held: they have to be sent right away instead
// of after the delay, or be lost.
func testNetworkDelayReleasedOnStop(t *testing.T, l Environment, e Extension) {
	port := startUdpEcho(t)
	config := map[string]any{"duration": 60000, "networkDelay": 5000, "port": []string{strconv.Itoa(port)}}
	action, err := e.RunAction(exthostwindows.BaseActionID+".network_delay", l.BuildTarget(t.Context()), config, defaultExecutionContext)
	defer func() { _ = action.Cancel() }()
	require.NoError(t, err)

	// 40 datagrams over 2 s, the attack is stopped after the first second: those sent until then are held for 5 s.
	var stopped time.Time
	x := exchangeUdp(t, port, 40, 20, 5*time.Second, func() {
		require.NoError(t, action.Cancel())
		stopped = time.Now()
	})
	assert.Equal(t, x.sent, x.intact, "held datagrams must not be lost")
	assert.Less(t, x.lastReply.Sub(stopped), 2*time.Second, "held datagrams must be sent right away")
}
