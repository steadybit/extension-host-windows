// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"bytes"
	"context"
	"errors"
	"github.com/rs/zerolog/log"
	aku "github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"golang.org/x/sys/windows/svc"
	"os/exec"
	"sync"
	"time"
)

var (
	runLock = aku.NewHashedKeyMutex(10)

	activeFWLock   = sync.Mutex{}
	activeFirewall = map[string][]WinOpts{}
)

func Apply(ctx context.Context, opts WinOpts) error {
	return generateAndRunCommands(ctx, opts, ModeAdd)
}

func Revert(ctx context.Context, opts WinOpts) error {
	return generateAndRunCommands(ctx, opts, ModeDelete)
}

func generateAndRunCommands(ctx context.Context, opts WinOpts, mode Mode) error {
	qosCommands, err := opts.QoSCommands(mode)
	if err != nil {
		return err
	}

	winDivertCommands, err := opts.WinDivertCommands(mode)
	if err != nil {
		return err
	}

	runLock.LockKey("windows")
	defer func() { _ = runLock.UnlockKey("windows") }()

	if mode == ModeAdd {
		if err := pushActiveFw(opts); err != nil {
			return err
		}
	}

	if len(qosCommands) > 0 {
		logCurrentQoSRules(ctx, "before")
		if _, qosErr := executeQoSCommands(ctx, qosCommands); qosErr != nil {
			err = errors.Join(err, qosErr)
		}
		logCurrentQoSRules(ctx, "after")
	}

	if len(winDivertCommands) > 0 {
		if _, wdErr := ExecuteWinDivertCommands(ctx, winDivertCommands, mode); wdErr != nil {
			err = errors.Join(err, wdErr)
		}
	}

	if mode == ModeDelete {
		popActiveFw("windows", opts)
	}

	return err
}

func logCurrentQoSRules(ctx context.Context, when string) {
	if !log.Trace().Enabled() {
		return
	}
	var outb, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, "powershell", "-Command", "Get-NetQosPolicy", "-PolicyStore", "ActiveStore", "|", "Where-Object", "{ $_.Name -like \"STEADYBIT*\" }")
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()

	if err != nil {
		log.Trace().Err(err).Msg("failed to get current firewall rules")
		return
	} else {
		log.Trace().Str("when", when).Str("rules", outb.String()).Msg("current fw rules")
	}
}

func pushActiveFw(opts WinOpts) error {
	activeFWLock.Lock()
	defer activeFWLock.Unlock()

	for _, active := range activeFirewall["windows"] {
		if !equals(opts, active) {
			return errors.New("running multiple network attacks at the same time is not supported")
		}
	}

	activeFirewall["windows"] = append(activeFirewall["windows"], opts)
	return nil
}

func popActiveFw(id string, opts WinOpts) {
	activeFWLock.Lock()
	defer activeFWLock.Unlock()

	active, ok := activeFirewall[id]
	if !ok {
		return
	}
	for i, a := range active {
		if equals(opts, a) {
			activeFirewall[id] = append(active[:i], active[i+1:]...)
			return
		}
	}
}

func equals(opts WinOpts, active WinOpts) bool {
	return opts.String() == active.String()
}

func ExecuteWinDivertCommands(ctx context.Context, cmds []string, mode Mode) (string, error) {
	if len(cmds) == 0 {
		return "", nil
	}

	out, err := utils.Execute(ctx, cmds, utils.PS)
	if err == nil {
		timeout := 10 * time.Second
		if mode == ModeAdd {
			err = awaitWinDivertServiceStatus(svc.Running, timeout)
			log.Debug().Msgf("WinDivert service is running")
		} else {
			err = awaitWinDivertServiceStatus(svc.Stopped, timeout)
			log.Debug().Msgf("WinDivert service is stopped")
		}
	}

	return out, err
}

func executeQoSCommands(ctx context.Context, cmds []string) (string, error) {
	if len(cmds) == 0 {
		return "", nil
	}

	return utils.Execute(ctx, cmds, utils.PSInvoke)
}
