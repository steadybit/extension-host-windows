package e2e

import (
	"context"
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	stopprocess "github.com/steadybit/extension-host-windows/exthostwindows/process"
	"os"
	"os/exec"
	"path"
)

var rootPath = ".."
var distPath = path.Join(rootPath, "dist")

type LocalEnvironment struct {
	Profile string
}

func newLocalEnvironment() *LocalEnvironment {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	return &LocalEnvironment{
		Profile: hostname,
	}
}

func (l *LocalEnvironment) BuildTarget(_ context.Context) *action_kit_api.Target {
	return &action_kit_api.Target{
		Attributes: map[string][]string{
			"host.hostname": {l.Profile},
		},
	}
}

func (l *LocalEnvironment) FindProcessIds(_ context.Context, name string) []int {
	return stopprocess.FindProcessIds(name)
}

func (l *LocalEnvironment) StartProcess(ctx context.Context, command string, parameters ...string) error {
	_, err := l.StartAndAwaitProcess(ctx, command, awaitStop(), parameters...)
	return err
}

func (l *LocalEnvironment) StartAndAwaitProcess(ctx context.Context, command string, awaitFn func(string) bool, parameters ...string) (func(), error) {
	cmd := exec.CommandContext(ctx, command, parameters...)
	cmd.Dir = rootPath
	cmd.Stdout = &PrefixWriter{prefix: []byte("🏠 "), w: os.Stdout}
	cmd.Stderr = &PrefixWriter{prefix: []byte("🏠 "), w: os.Stderr}
	err := awaitStartup(cmd, awaitFn)
	if err != nil {
		return nil, err
	}
	return func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}, nil
}

func (l *LocalEnvironment) StopProcess(_ context.Context, commandOrPid string) error {
	pids := stopprocess.FindProcessIds(commandOrPid)
	if len(pids) != 1 {
		return fmt.Errorf("cannot stop process %q, found candidates %v", commandOrPid, pids)
	}
	return stopprocess.StopProcesses(pids, true)
}
