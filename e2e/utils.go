// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package e2e

import (
	"bufio"
	"fmt"
	"github.com/mholt/archiver/v3"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
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

func awaitLog(awaitOutput string) func(string) bool {
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
				awaitFinished = true
				startupFinished <- true
			}
		}
	}

	if awaitFn != nil {
		stdoutPr, stdoutPw := pipeWriter(cmd.Stdout)
		cmd.Stdout = stdoutPw
		go awaitOutput(stdoutPr)

		stderrPr, stderrPw := pipeWriter(cmd.Stderr)
		cmd.Stderr = stderrPw
		go awaitOutput(stderrPr)
	}

	err := cmd.Start()
	if err != nil {
		return err
	}

	var cmdErr error
	go func() {
		cmdErr = cmd.Wait()
		startupFinished <- true
	}()

	if awaitFn != nil {
		<-startupFinished
	}

	return cmdErr
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
		_, err := p.w.Write(p.prefix)
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

func HasAttribute(target discovery_kit_api.Target, key string) bool {
	return ContainsAttribute(target.Attributes, key)
}

func ContainsAttribute(attributes map[string][]string, key string) bool {
	_, ok := attributes[key]
	return ok
}

func getCIDRsFor(s string, maskLen int) (cidrs []string) {
	ips, _ := net.LookupIP(s)
	for _, p := range ips {
		cidr := net.IPNet{IP: p.To4(), Mask: net.CIDRMask(maskLen, 32)}
		cidrs = append(cidrs, cidr.String())
	}
	return
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

func IsPortAvailable(port int) bool {
	address := ":" + strconv.Itoa(port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	defer func(listener net.Listener) {
		_ = listener.Close()
	}(listener)
	return true
}

func FindAvailablePort(startPort, endPort int) (int, error) {
	for port := startPort; port <= endPort; port++ {
		if IsPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", startPort, endPort)
}

func FindAvailablePorts(startPort, endPort int, count int) (int, error) {
	missing := count
	for port := startPort; port <= endPort; port++ {
		if IsPortAvailable(port) {
			missing--
			if missing == 0 {
				return port - (count - 1), nil
			}
		} else {
			missing = count
		}
	}
	return 0, fmt.Errorf("no %d contiguous available ports found in range %d-%d", count, startPort, endPort)
}
