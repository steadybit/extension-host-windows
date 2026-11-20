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

// IsProcessRunning reports whether a process with the given name is running. Like Get-Process -Name, the name is
// matched case-insensitively against the executable name without its ".exe" extension.
func IsProcessRunning(processName string) (bool, error) {
	pids, err := findProcessIdsByName(processName)
	return len(pids) > 0, err
}

// StopProcess forcefully terminates all processes with the given name, see IsProcessRunning for the matching.
func StopProcess(processName string) error {
	pids, err := findProcessIdsByName(processName)
	if err != nil {
		return err
	}
	return StopProcesses(pids, true)
}

func findProcessIdsByName(processName string) ([]int, error) {
	processes, err := ps.Processes()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %w", err)
	}

	var pids []int
	for _, p := range processes {
		name := p.Executable()
		if len(name) > 4 && strings.EqualFold(name[len(name)-4:], ".exe") {
			name = name[:len(name)-4]
		}
		if strings.EqualFold(name, processName) {
			pids = append(pids, p.Pid())
		}
	}
	return pids, nil
}

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
		return fmt.Errorf("fail to stop processes : %s", strings.Join(errs, ", "))
	}
	return nil
}

func stopProcessWindows(pid int, force bool) error {
	if force {
		return terminateProcess(pid)
	}

	// use absolute path to resolve untrusted search path issue reported  by Sonar
	taskkill, err := exec.LookPath("taskkill.exe")
	if err != nil {
		return fmt.Errorf("fail to find taskkill.exe: %w", err)
	}
	err = exec.Command(taskkill, "/pid", fmt.Sprintf("%d", pid)).Run()
	if err != nil {
		if isProcessNotFound(err) {
			log.Debug().Int("pid", pid).Msg("Process already exited")
			return nil
		}
		return fmt.Errorf("failed to kill process %d via taskkill: %w", pid, err)
	}
	return nil
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
