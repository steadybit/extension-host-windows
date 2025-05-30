// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/rs/zerolog/log"
)

type Shell = string

const (
	PSStart Shell = "PSStart"
	PSRun   Shell = "PSRun"
)

// ExecutePowershellCommand runs the given commands in a powershell session.
// Callers must make sure that passed in commands are properly sanitizes.
func ExecutePowershellCommand(ctx context.Context, cmds []string, shell Shell) (string, error) {
	log.Debug().Strs("cmds", cmds).Msg("running commands")

	commands := strings.Join(cmds, ";")

	if shell == PSRun {
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
	} else {
		cmd := exec.Command("powershell", "-Command", commands) //NOSONAR commands are sanitized
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return "", cmd.Start()
	}
}

// BuildSystemCommandFor builds up the commands to wrap the given one into a scheduled task executed in SYSTEM scope.
func BuildSystemCommandFor(cmd string) []string {
	scheduledTaskAction := fmt.Sprintf("$A=New-ScheduledTaskAction -Execute powershell -Argument \"-WindowStyle Hidden -Command %s\"", cmd)
	principal := "$P=New-ScheduledTaskPrincipal -UserId \"SYSTEM\" -RunLevel Highest"
	registerScheduledTask := "Register-ScheduledTask SteadybitTempQoSPolicyTask -Action $A -Principal $P"
	startTask := "Start-ScheduledTask SteadybitTempQoSPolicyTask"
	awaitExecution := "for($i=0;$i -lt 20;$i++){if((Get-ScheduledTask -TaskName SteadybitTempQoSPolicyTask).State -ne 'Running'){break};Start-Sleep -Milliseconds 100};"
	unregisterScheduledTask := "try{ Unregister-ScheduledTask SteadybitTempQoSPolicyTask -Confirm:$false } catch {}"
	return []string{unregisterScheduledTask, scheduledTaskAction, principal, registerScheduledTask, startTask, awaitExecution, unregisterScheduledTask}
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
