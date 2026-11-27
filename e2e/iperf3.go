// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/yalp/jsonpath"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	iperf3ReleaseUrl = "https://github.com/ar51an/iperf3-win-builds/releases/download/3.18/iperf-3.18-win64.zip"
)

type Iperf struct {
	Ip         string
	Port       int
	executable string
}

func NewIperf(server string, port int) *Iperf {
	return &Iperf{Ip: server, Port: port}
}

func (n *Iperf) Install() error {
	log.Info().Msgf("Downloading iperf")
	tmpDir, err := os.MkdirTemp("", "steadybit-extension-windows-host-iperf3-*")
	if err != nil {
		return err
	}

	zipPath := filepath.Join(tmpDir, "iperf.zip")
	err = downloadFile(zipPath, iperf3ReleaseUrl)
	if err != nil {
		return err
	}

	extractPath := filepath.Join(tmpDir, "extracted")
	err = os.MkdirAll(extractPath, 0755)
	if err != nil {
		return err
	}

	err = extractZip(zipPath, extractPath)
	if err != nil {
		return err
	}

	n.executable = filepath.Join(extractPath, "iperf3.exe")
	return nil
}

func (n *Iperf) MeasureBandwidth(ctx context.Context, env Environment) (float64, error) {
	out, err := env.ExecuteProcess(ctx, n.executable, "--client", n.Ip, fmt.Sprintf("--port=%d", n.Port), "--time=5", "--json")
	if err != nil {
		return 0, fmt.Errorf("%w: %s", err, out)
	}

	var result any
	err = json.Unmarshal([]byte(out), &result)
	if err != nil {
		return 0, fmt.Errorf("failed reading results: %w", err)
	}

	bps, err := jsonpath.Read(result, "$.end.sum_sent.bits_per_second")
	if err != nil {
		return 0, fmt.Errorf("failed reading bits_per_second: %w", err)
	}
	bandwidth := bps.(float64) / 1_000_000
	log.Info().Msgf("measure bandwidth %v", bandwidth)
	return bandwidth, nil
}

func (n *Iperf) AssertBandwidth(t *testing.T, ctx context.Context, env Environment, min float64, max float64) {
	t.Helper()
	measurements := make([]float64, 0, 5)
	Retry(t, 3, 200*time.Millisecond, func(r *R) {
		bandwidth, err := n.MeasureBandwidth(ctx, env)
		if err != nil {
			r.Failed = true
			_, _ = fmt.Fprintf(r.Log, "failed to measure bandwidth bandwidth: %s", err)
		} else if bandwidth < min || bandwidth > max {
			r.Failed = true
			measurements = append(measurements, bandwidth)
			_, _ = fmt.Fprintf(r.Log, "bandwidth %f is not in expected range [%f, %f]", measurements, min, max)
		}
	})
}
