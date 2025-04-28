// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import (
	"bytes"
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type Shell = string

const (
	PSStart Shell = "PSStart"
	PSRun   Shell = "PSRun"
)

// Execute runs the given commands in a powershell session.
// Callers must make sure that passed in commands are properly sanitizes.
func Execute(ctx context.Context, cmds []string, shell Shell) (string, error) {
	log.Info().Strs("cmds", cmds).Msg("running commands")

	commands := strings.Join(cmds, ";")

	if shell == PSRun {
		var outb, errb bytes.Buffer
		cmd := exec.CommandContext(ctx, "powershell", "-Command", commands) //NOSONAR commands are sanitizes
		cmd.Stdout = &outb
		cmd.Stderr = &errb
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("execution failed: %w, output: %s, error: %s", err, outb.String(), errb.String())
		}
		return outb.String(), err
	} else {
		cmd := exec.Command("powershell", "-Command", commands) //NOSONAR commands are sanitizes
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return "", cmd.Start()
	}
}

func SanitizePowershellArgs(args ...string) []string {
	var sanitizedArgs []string
	for _, arg := range args {
		sanitizedArgs = append(sanitizedArgs, SanitizePowershellArg(arg))
	}
	return sanitizedArgs
}

func SanitizePowershellArg(arg string) string {
	// First escape backticks (since we use them for escaping other chars)
	arg = strings.ReplaceAll(arg, "`", "``")

	// Escape other special characters
	arg = strings.ReplaceAll(arg, "$", "`$")
	arg = strings.ReplaceAll(arg, "\"", "`\"")
	arg = strings.ReplaceAll(arg, "'", "''") // PowerShell uses doubled single quotes
	arg = strings.ReplaceAll(arg, "(", "`(")
	arg = strings.ReplaceAll(arg, ")", "`)")
	arg = strings.ReplaceAll(arg, "{", "`{")
	arg = strings.ReplaceAll(arg, "}", "`}")
	arg = strings.ReplaceAll(arg, ";", "`;")
	arg = strings.ReplaceAll(arg, "|", "`|")
	arg = strings.ReplaceAll(arg, "&", "`&")
	arg = strings.ReplaceAll(arg, ">", "`>")
	arg = strings.ReplaceAll(arg, "<", "`<")

	return arg
}
