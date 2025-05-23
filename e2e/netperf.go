package e2e

import (
	"bytes"
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"testing"
	"text/template"
	"time"
)

type HttpNetperf struct {
	Ip          string
	Port        int
	numRequests int
	client      *http.Client
	cancel      func()
}

func NewHttpNetperf(port int) *HttpNetperf {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	return &HttpNetperf{"127.0.0.1", port, 1, client, nil}
}

func (n *HttpNetperf) Deploy(ctx context.Context, env Environment) error {
	log.Info().Msgf("Starting HTTP test server on port %d", n.Port)
	httpServerCommandTemplate, err := os.ReadFile("startHttpServer.ps1")
	if err != nil {
		return err
	}

	tmpl, err := template.New("startHttpServer").
		Funcs(template.FuncMap{
			"nextPort": func(port int) int {
				return port + 1
			},
		}).
		Parse(string(httpServerCommandTemplate))
	if err != nil {
		return err
	}

	var scriptBuffer bytes.Buffer
	err = tmpl.Execute(&scriptBuffer, n)
	if err != nil {
		return err
	}

	n.cancel, err = env.StartAndAwaitProcess(ctx, "powershell", awaitLog("Listening"), "-Command", scriptBuffer.String())
	return err
}

func (n *HttpNetperf) Delete() error {
	if n.cancel != nil {
		log.Info().Msgf("Stopping HTTP test server")
		n.cancel()
	}
	return nil
}

func (n *HttpNetperf) MeasureLatency() (time.Duration, error) {
	var totalTime time.Duration

	// Just a rough measurement but should be good enough for our use-case.
	for i := 0; i < n.numRequests; i++ {
		start := time.Now()
		resp, err := n.client.Get(n.url())
		if err != nil {
			return 0, fmt.Errorf("request failed: %w", err)
		}
		_ = resp.Body.Close()
		totalTime += time.Since(start)
	}

	meanTime := totalTime / time.Duration(n.numRequests)
	return meanTime, nil
}

func (n *HttpNetperf) AssertLatency(t *testing.T, min time.Duration, max time.Duration) {
	t.Helper()

	measurements := make([]time.Duration, 0, 5)
	Retry(t, 8, 500*time.Millisecond, func(r *R) {
		latency, err := n.MeasureLatency()
		if err != nil {
			r.Failed = true
			_, _ = fmt.Fprintf(r.Log, "failed to measure package latency: %s", err)
		}
		if latency < min || latency > max {
			r.Failed = true
			measurements = append(measurements, latency)
			_, _ = fmt.Fprintf(r.Log, "package latency %v is not in expected range [%s, %s]", measurements, min, max)
		}
	})
}

func (n *HttpNetperf) url() string {
	return fmt.Sprintf("http://%s:%d", n.Ip, n.Port)
}

func (n *HttpNetperf) IsReachable() bool {
	resp, err := n.client.Get(n.url())
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()
	if err != nil {
		log.Debug().Err(err).Msgf("Can't reach %s", n.url())
	} else {
		log.Info().Str("status", resp.Status).Msgf("Reached %s", n.url())
	}
	return err == nil && resp.StatusCode == 200
}

func (n *HttpNetperf) CanReach(targetUrl string) bool {
	resp, err := n.client.Get(fmt.Sprintf("%s?url=%s", n.url(), targetUrl))
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()
	if err != nil {
		log.Debug().Err(err).Msgf("Can't reach %s", targetUrl)
	} else {
		log.Info().Str("status", resp.Status).Msgf("Reached %s", n.url())
	}
	return err == nil && resp.StatusCode == 200
}
