// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package e2e

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const udpPayloadSize = 512

// startUdpEcho runs a UDP server on loopback that sends every datagram back, and returns its port.
func startUdpEcho(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	go func() {
		buf := make([]byte, 2*udpPayloadSize)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if errors.Is(err, net.ErrClosed) {
				return
			}
			// A corrupted datagram can reach a closed port, and the ICMP error fails the next read with WSAECONNRESET.
			if err != nil {
				continue
			}
			_, _ = conn.WriteToUDP(buf[:n], addr)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

// udpExchange is the outcome of sending datagrams to the echo server. An intact reply is equal to the datagram that
// was sent.
type udpExchange struct {
	sent   int
	intact int
	// rtts are the round trip times of the intact replies.
	rtts []time.Duration
	// lastReply is when the last intact reply arrived.
	lastReply time.Time
}

func (x udpExchange) intactShare() float64 {
	return float64(x.intact) / float64(x.sent)
}

func (x udpExchange) percentile(p float64) time.Duration {
	if len(x.rtts) == 0 {
		return 0
	}
	sorted := slices.Clone(x.rtts)
	slices.Sort(sorted)
	return sorted[int(float64(len(sorted)-1)*p)]
}

// udpPayload is a datagram whose content follows from its sequence number, so a corrupted reply is recognized.
func udpPayload(seq uint64) []byte {
	payload := make([]byte, udpPayloadSize)
	binary.BigEndian.PutUint64(payload, seq)
	for i := 8; i < len(payload); i++ {
		payload[i] = byte(seq) ^ byte(i)
	}
	return payload
}

// exchangeUdp sends count datagrams to the echo server at the given rate per second, then waits up to linger for the
// outstanding replies. Before it returns, it calls during, if not nil, once half of the datagrams are sent.
func exchangeUdp(t *testing.T, port, count, rate int, linger time.Duration, during func()) udpExchange {
	t.Helper()
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NoError(t, conn.SetReadBuffer(8<<20))

	var mu sync.Mutex
	sentAt := make([]time.Time, count)
	result := udpExchange{sent: count}
	seen := make(map[uint64]bool, count)
	replies := 0
	allReplied := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 2*udpPayloadSize)
		for {
			n, err := conn.Read(buf)
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if err != nil {
				continue
			}
			now := time.Now()
			mu.Lock()
			seq := binary.BigEndian.Uint64(buf)
			if seq < uint64(count) && !seen[seq] && bytes.Equal(buf[:n], udpPayload(seq)) {
				seen[seq] = true
				result.intact++
				result.rtts = append(result.rtts, now.Sub(sentAt[seq]))
				result.lastReply = now
			}
			if replies++; replies == count {
				close(allReplied)
			}
			mu.Unlock()
		}
	}()

	// Sent in bursts every 10 ms, as many as are due by then: sleeping between single datagrams is too coarse for
	// thousands per second.
	ticker := time.NewTicker(10 * time.Millisecond)
	start := time.Now()
	for seq := 0; seq < count; {
		due := min(count, 1+int(time.Since(start).Seconds()*float64(rate)))
		for seq < due {
			mu.Lock()
			sentAt[seq] = time.Now()
			mu.Unlock()
			_, _ = conn.Write(udpPayload(uint64(seq)))
			seq++
			if seq == count/2 && during != nil {
				during()
			}
		}
		<-ticker.C
	}
	ticker.Stop()

	select {
	case <-allReplied:
	case <-time.After(linger):
	}
	_ = conn.Close()
	<-done
	mu.Lock()
	defer mu.Unlock()
	return result
}
