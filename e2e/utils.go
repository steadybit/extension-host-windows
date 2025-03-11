// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package e2e

import (
	"bufio"
	"context"
	"fmt"
	"github.com/mholt/archiver/v3"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

func findExtensionArtifact(dir string) (string, error) {
	return findFileWithExtension(dir, ".zip")
}

func findExtensionExecutable(dir string) (string, error) {
	return findFileWithExtension(dir, ".exe")
}

func findFileWithExtension(dir string, extension string) (string, error) {
	var artifact string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), extension) {
			artifact = path
			return filepath.SkipAll
		}
		return nil
	})
	return artifact, err
}

func extractArtifact(artifact string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "extension-artifact-*")
	if err != nil {
		return "", err
	}
	err = archiver.Unarchive(artifact, tmpDir)
	if err != nil {
		return "", err
	}
	return tmpDir, nil
}

func awaitLogFn(awaitOutput string) func(string) bool {
	return func(line string) bool {
		return strings.Contains(line, awaitOutput)
	}
}

func awaitStartup(cmd *exec.Cmd, awaitFn func(string) bool) error {
	awaitFinished := false
	startupFinished := make(chan bool)

	awaitOutput := func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			if !awaitFinished && awaitFn(line) {
				startupFinished <- true
				awaitFinished = true
			}
		}
	}

	stdoutPr, stdoutPw := pipeWriter(cmd.Stdout)
	cmd.Stdout = stdoutPw
	go awaitOutput(stdoutPr)

	stderrPr, stderrPw := pipeWriter(cmd.Stderr)
	cmd.Stderr = stderrPw
	go awaitOutput(stderrPr)

	err := cmd.Start()
	if err != nil {
		return err
	}

	timeout := time.After(30 * time.Second)
	select {
	case <-startupFinished:
		break
	case <-timeout:
		log.Fatal().Msgf("Cmd %s did not start up in time", cmd.String())
	}
	return nil
}

func pipeWriter(w io.Writer) (io.Reader, io.Writer) {
	pr, pw := io.Pipe()
	if w != nil {
		return pr, io.MultiWriter(w, pw)
	} else {
		return pr, pw
	}
}

type PrefixWriter struct {
	prefix             []byte
	w                  io.Writer
	notStartWithPrefix bool
	m                  sync.Mutex
}

func (p *PrefixWriter) Write(buf []byte) (n int, err error) {
	p.m.Lock()
	defer p.m.Unlock()

	if !p.notStartWithPrefix {
		p.notStartWithPrefix = true
		_, err := p.w.Write([]byte(p.prefix))
		if err != nil {
			return 0, err
		}
	}

	remainder := buf
	for {
		var c int
		if j := slices.Index(remainder, '\n'); j >= 0 {
			c, err = p.w.Write(remainder[:j+1])
			if j+1 < len(remainder) {
				_, err = p.w.Write(p.prefix)
			} else {
				p.notStartWithPrefix = false
			}
			remainder = remainder[j+1:]
		} else {
			c, err = p.w.Write(remainder)
			remainder = nil
		}
		n += c
		if len(remainder) == 0 || err != nil {
			return
		}
	}
}

func PollForTarget(ctx context.Context, e Extension, targetId string, predicate func(target discovery_kit_api.Target) bool) (discovery_kit_api.Target, error) {
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return discovery_kit_api.Target{}, fmt.Errorf("timed out waiting for target. last error: %w", lastErr)
		case <-time.After(200 * time.Millisecond):
			result, err := e.DiscoverTargets(targetId)
			if err != nil {
				lastErr = err
				continue
			}
			for _, target := range result {
				if predicate(target) {
					return target, nil
				}
			}
		}
	}
}

func PollForEnrichmentData(ctx context.Context, e Extension, targetId string, predicate func(target discovery_kit_api.EnrichmentData) bool) (discovery_kit_api.EnrichmentData, error) {
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return discovery_kit_api.EnrichmentData{}, fmt.Errorf("timed out waiting for target. last error: %w", lastErr)
		case <-time.After(200 * time.Millisecond):
			result, err := e.DiscoverEnrichmentData(targetId)
			if err != nil {
				lastErr = err
				continue
			}
			for _, enrichmentData := range result {
				if predicate(enrichmentData) {
					return enrichmentData, nil
				}
			}
		}
	}
}

func HasAttribute(target discovery_kit_api.Target, key string) bool {
	return ContainsAttribute(target.Attributes, key)
}

func ContainsAttribute(attributes map[string][]string, key string) bool {
	_, ok := attributes[key]
	return ok
}
