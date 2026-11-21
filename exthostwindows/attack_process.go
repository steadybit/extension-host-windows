// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"bytes"
	"os/exec"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
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
