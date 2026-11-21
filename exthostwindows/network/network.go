// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	aku "github.com/steadybit/action-kit/go/action_kit_commons/utils"
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

// CleanupQosPolicies removes leftover Steadybit QoS policies. It only looks for them when they may exist, as it holds
// the lock that network attacks wait for.
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
	qosPolicies := opts.QoSPolicies()

	runLock.LockKey("windows")
	defer func() { _ = runLock.UnlockKey("windows") }()

	if mode == ModeAdd {
		if err := pushActiveFw(opts); err != nil {
			return err
		}
	} else if !isActiveFw(opts) && activeFw() {
		// This attack is not running, e.g. its start failed and was reverted, but another one started since. Reverting
		// would stop that one, as both share wdna. Without any active attack, e.g. after a restart, revert anyway.
		log.Info().Str("attack", opts.String()).Msg("attack is not active, skipping revert")
		return nil
	}

	var err error
	if len(qosPolicies) > 0 {
		qosPoliciesClean.Store(false)
		err = applyQosPolicies(ctx, qosPolicies, mode)
	}

	if attack := opts.WinDivert(); attack != nil {
		var wdErr error
		if mode == ModeAdd {
			wdErr = startWinDivert(ctx, attack)
		} else {
			wdErr = stopWinDivert()
		}
		err = errors.Join(err, wdErr)
	}

	if mode == ModeDelete || err != nil {
		popActiveFw("windows", opts)
	}

	// Whether the platform calls stop after a failed start is not guaranteed, so the start leaves nothing behind; a
	// later stop finds nothing to revert, which succeeds as well. An identical attack that is still running shares the
	// policies and wdna, so it is left alone.
	if mode == ModeAdd && err != nil && !activeFw() {
		err = errors.Join(err, revertFailedStart(context.WithoutCancel(ctx), opts, qosPolicies))
	}

	return err
}

func revertFailedStart(ctx context.Context, opts WinOpts, qosPolicies []QosPolicy) error {
	var err error
	if len(qosPolicies) > 0 {
		err = applyQosPolicies(ctx, qosPolicies, ModeDelete)
	}
	if opts.WinDivert() != nil {
		err = errors.Join(err, stopWinDivert())
	}
	if err != nil {
		return fmt.Errorf("failed to revert the failed start: %w", err)
	}
	return nil
}

func activeFw() bool {
	activeFWLock.Lock()
	defer activeFWLock.Unlock()
	return len(activeFirewall["windows"]) > 0
}

func isActiveFw(opts WinOpts) bool {
	activeFWLock.Lock()
	defer activeFWLock.Unlock()
	for _, active := range activeFirewall["windows"] {
		if opts.String() == active.String() {
			return true
		}
	}
	return false
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
