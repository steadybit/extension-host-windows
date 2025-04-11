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
	PS       Shell = "PowerShell" // regular powershell.
	PSInvoke Shell = "PSInvoke"   // powershell Invoke-Command
)

func Execute(ctx context.Context, cmds []string, shell Shell) (string, error) {
	log.Info().Strs("cmds", cmds).Msg("running commands")

	if shell == PSInvoke {
		var outb, errb bytes.Buffer
		joinedCommands := "\"" + strings.Join(cmds, ";") + "\""
		cmd := exec.CommandContext(ctx, "powershell", "-Command", "Invoke-Expression", joinedCommands)
		cmd.Stdout = &outb
		cmd.Stderr = &errb
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("execution failed: %w, output: %s, error: %s", err, outb.String(), errb.String())
		}
		return outb.String(), err
	} else {
		cmd := exec.Command("powershell", "-Command", strings.Join(cmds, ";"))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return "", cmd.Start()
	}
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
