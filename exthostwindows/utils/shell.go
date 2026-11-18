// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package utils

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
)

// ExecutePowershellCommand runs the given commands in a powershell session.
// Callers must make sure that passed in commands are properly sanitizes.
func ExecutePowershellCommand(ctx context.Context, cmds []string) (string, error) {
	log.Debug().Strs("cmds", cmds).Msg("running commands")

	commands := strings.Join(cmds, ";")

	var outb, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, "powershell", "-Command", commands) //NOSONAR commands are sanitized
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()
	out := strings.TrimSpace(outb.String())
	if err != nil {
		return "", fmt.Errorf("execution failed: %w, output: %s, error: %s", err, out, errb.String())
	}
	return out, err
}
