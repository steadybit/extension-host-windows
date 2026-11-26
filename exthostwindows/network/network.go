// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Execution identifies the run of a network attack.
type Execution struct {
	// Id is the id of the target execution.
	Id                    uuid.UUID
	ExperimentKey         string
	ExperimentExecutionId int
}

var (
	// runLock serializes the starts and stops of network attacks and guards active.
	runLock sync.Mutex
	// active is the execution of the running attack. Only one network attack runs at a time: WinDivert applies the
	// attacks of all wdna to a packet, so even identical attacks would add up.
	active *Execution

	// qosPoliciesClean is reset whenever an attack changes QoS policies; until the next cleanup, leftovers are possible.
	// It starts unset, as a crashed process may have left policies behind.
	qosPoliciesClean atomic.Bool
)

// Apply starts the attack, unless another network attack is running.
func Apply(ctx context.Context, execution Execution, opts WinOpts) error {
	runLock.Lock()
	defer runLock.Unlock()

	if active != nil {
		return fmt.Errorf("running multiple network attacks at the same time is not supported. Already running attack started by %s (#%d) in targetExecution %s",
			active.ExperimentKey, active.ExperimentExecutionId, active.Id)
	}
	active = &execution

	err := apply(ctx, execution.Id, opts)
	if err != nil {
		// Whether the platform calls stop after a failed start is not guaranteed, so the start leaves nothing behind.
		if revertErr := revert(context.WithoutCancel(ctx), execution.Id, opts); revertErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to revert the failed start: %w", revertErr))
		}
	}
	return err
}

// Revert stops the attack. It leaves the attack of another execution running, e.g. when the stop is for an attack that
// was rejected.
func Revert(ctx context.Context, execution Execution, opts WinOpts) error {
	runLock.Lock()
	defer runLock.Unlock()
	if active != nil && active.Id != execution.Id {
		return nil
	}
	return revert(ctx, execution.Id, opts)
}

func apply(ctx context.Context, executionId uuid.UUID, opts WinOpts) error {
	if policies := opts.QoSPolicies(); len(policies) > 0 {
		qosPoliciesClean.Store(false)
		if err := applyQosPolicies(ctx, policies, ModeAdd); err != nil {
			return err
		}
	}
	if attack := opts.WinDivert(); attack != nil {
		return startWinDivert(ctx, executionId, attack)
	}
	return nil
}

func revert(ctx context.Context, executionId uuid.UUID, opts WinOpts) error {
	active = nil

	var err error
	if policies := opts.QoSPolicies(); len(policies) > 0 {
		qosPoliciesClean.Store(false)
		err = applyQosPolicies(ctx, policies, ModeDelete)
	}
	if opts.WinDivert() != nil {
		err = errors.Join(err, stopWinDivert(executionId))
	}
	return err
}

// CleanupQosPolicies removes leftover Steadybit QoS policies. It only looks for them when they may exist, as it holds
// the lock that network attacks wait for.
func CleanupQosPolicies() {
	if qosPoliciesClean.Load() {
		return
	}
	runLock.Lock()
	defer runLock.Unlock()
	if active != nil {
		return
	}
	if err := removeSteadybitQosPolicies(context.Background()); err != nil {
		log.Error().Err(err).Msg("Error removing Steadybit QoS policies")
		return
	}
	qosPoliciesClean.Store(true)
}
