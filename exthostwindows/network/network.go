// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	aku "github.com/steadybit/action-kit/go/action_kit_commons/utils"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
)

var (
	runLock = aku.NewHashedKeyMutex(10)

	activeFWLock   = sync.Mutex{}
	activeFirewall = map[string][]WinOpts{}

	// qosPoliciesClean is reset whenever an attack changes QoS policies; until the next cleanup, leftovers are possible.
	// It starts unset, as a crashed process may have left policies behind.
	qosPoliciesClean atomic.Bool
)

func Apply(ctx context.Context, opts WinOpts) error {
	return generateAndRunCommands(ctx, opts, ModeAdd)
}

func Revert(ctx context.Context, opts WinOpts) error {
	return generateAndRunCommands(ctx, opts, ModeDelete)
}

// CleanupQosPolicies removes leftover Steadybit QoS policies. It only lists them when they may exist, as listing
// starts PowerShell and holds the lock that network attacks wait for.
func CleanupQosPolicies() {
	if qosPoliciesClean.Load() {
		return
	}
	runLock.LockKey("windows")
	defer func() { _ = runLock.UnlockKey("windows") }()
	if activeFw() {
		return
	}
	if err := removeSteadybitQosPolicies(context.Background()); err != nil {
		log.Error().Err(err).Msg("Error removing Steadybit QoS policies")
		return
	}
	qosPoliciesClean.Store(true)
}

func generateAndRunCommands(ctx context.Context, opts WinOpts, mode Mode) error {
	qosCommands, err := opts.QoSCommands(mode)
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
		qosPoliciesClean.Store(false)
		if _, qosErr := executeQoSCommands(ctx, qosCommands); qosErr != nil {
			err = errors.Join(err, qosErr)
		}
	}

	if attack := opts.WinDivert(); attack != nil {
		var wdErr error
		if mode == ModeAdd {
			wdErr = startWinDivert(ctx, attack)
		} else {
			wdErr = stopWinDivert(ctx)
		}
		err = errors.Join(err, wdErr)
	}

	if mode == ModeDelete {
		popActiveFw("windows", opts)
	}

	return err
}

func activeFw() bool {
	activeFWLock.Lock()
	defer activeFWLock.Unlock()
	return len(activeFirewall["windows"]) > 0
}

func pushActiveFw(opts WinOpts) error {
	activeFWLock.Lock()
	defer activeFWLock.Unlock()

	for _, active := range activeFirewall["windows"] {
		if opts.String() != active.String() {
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
		if opts.String() == a.String() {
			activeFirewall[id] = append(active[:i], active[i+1:]...)
			return
		}
	}
}

func executeQoSCommands(ctx context.Context, cmds []string) (string, error) {
	logCurrentQoSRules(ctx, "before")
	defer logCurrentQoSRules(ctx, "after")
	return utils.ExecutePowershellCommand(ctx, cmds)
}
