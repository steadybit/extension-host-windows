// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"bytes"
	"os/exec"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"github.com/steadybit/extension-kit/extutil"
)

// runAttackProcess starts cmd in the job of the attack, see utils.StartAttackProcess, and logs its output once it ends.
func runAttackProcess(executionId uuid.UUID, cmd *exec.Cmd) error {
	log.Info().Msgf("Running command: %s, %s.", cmd.Path, cmd.Args)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := utils.StartAttackProcess(executionId, cmd); err != nil {
		return err
	}
	go func() {
		// Also ends with an error when the attack is stopped and its processes are terminated.
		err := cmd.Wait()
		log.Info().Err(err).Str("output", output.String()).Msgf("%s ended", cmd.Path)
	}()
	return nil
}

// attackStatus reports the attack as completed once all processes it started have exited.
func attackStatus(executionId uuid.UUID, completedMessage string) (*action_kit_api.StatusResult, error) {
	running, err := utils.AttackProcessesRunning(executionId)
	if err != nil {
		return nil, err
	}
	if running {
		return &action_kit_api.StatusResult{Completed: false}, nil
	}
	return &action_kit_api.StatusResult{Completed: true, Messages: infoMessage(completedMessage)}, nil
}

func stopAttack(executionId uuid.UUID, stoppedMessage string) (*action_kit_api.StopResult, error) {
	if err := utils.StopAttackProcesses(executionId, 0); err != nil {
		return nil, err
	}
	return &action_kit_api.StopResult{Messages: infoMessage(stoppedMessage)}, nil
}

func infoMessage(message string) *action_kit_api.Messages {
	return &action_kit_api.Messages{{Level: extutil.Ptr(action_kit_api.Info), Message: message}}
}
