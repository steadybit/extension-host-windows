// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package utils

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mitchellh/go-ps"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-kit/extutil"
	"golang.org/x/sys/windows"
)

func StopProcesses(pids []int, force bool) error {
	if len(pids) == 0 {
		return nil
	}

	errs := make([]string, 0)
	for _, pid := range pids {
		log.Info().Int("pid", pid).Msg("Stopping process")
		if err := stopProcessWindows(pid, force); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		err := fmt.Errorf("failed to stop processes: %s", strings.Join(errs, ", "))
		if !force {
			return fmt.Errorf("%w. %s", err, gracefulStopHint)
		}
		return err
	}
	return nil
}

// gracefulStopHint explains a failed graceful stop. taskkill's own reason is localized, so it can't tell apart why the
// close request was refused; a process without a window is the usual case on servers.
const gracefulStopHint = "A graceful stop asks a process to close through its window, so it does not work for processes " +
	"without one, such as Windows services and IIS. Disable 'graceful' to stop them forcibly."

func stopProcessWindows(pid int, force bool) error {
	if force {
		return terminateProcess(pid)
	}

	// use absolute path to resolve untrusted search path issue reported  by Sonar
	taskkill, err := exec.LookPath("taskkill.exe")
	if err != nil {
		return fmt.Errorf("fail to find taskkill.exe: %w", err)
	}
	output, err := exec.Command(taskkill, "/pid", fmt.Sprintf("%d", pid)).CombinedOutput()
	if err != nil {
		if isProcessNotFound(err) {
			log.Debug().Int("pid", pid).Msg("Process already exited")
			return nil
		}
		reason := strings.TrimRight(strings.Join(strings.Fields(strings.ToValidUTF8(string(output), "?")), " "), ".")
		if reason == "" {
			reason = err.Error()
		}
		return fmt.Errorf("process %d%s did not close gracefully (taskkill: %s)", pid, processName(pid), reason)
	}
	return nil
}

func processName(pid int) string {
	if p, err := ps.FindProcess(pid); err == nil && p != nil {
		return fmt.Sprintf(" (%s)", p.Executable())
	}
	return ""
}

const terminateWaitTimeout = 5 * time.Second

// terminateProcess kills the process directly; starting taskkill.exe /F for the same effect takes up to a second.
func terminateProcess(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		log.Debug().Int("pid", pid).Msg("process already exited")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to open process %d: %w", pid, err)
	}
	defer func() { _ = windows.CloseHandle(h) }()

	terminateErr := windows.TerminateProcess(h, 1)
	exited, _ := windows.WaitForSingleObject(h, uint32(terminateWaitTimeout.Milliseconds()))
	// TerminateProcess fails with access denied for a process that is already exiting, e.g. the parent of a
	// process stopped just before that exits on its own.
	if terminateErr != nil && exited != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("failed to force kill process %d: %w", pid, terminateErr)
	}
	return nil
}

func isProcessNotFound(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 128
}

// FindProcessIds returns the given pid, or the pids of all processes whose executable name contains processOrPid.
func FindProcessIds(processOrPid string) []int {
	pid := extutil.ToInt(processOrPid)
	if pid > 0 {
		return []int{pid}
	}

	var pids []int
	processes, err := ps.Processes()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list processes")
		return nil
	}
	for _, process := range processes {
		if strings.Contains(strings.TrimSpace(process.Executable()), processOrPid) {
			pids = append(pids, process.Pid())
		}
	}
	return pids
}
