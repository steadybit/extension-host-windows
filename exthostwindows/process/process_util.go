// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package stopprocess

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mitchellh/go-ps"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-kit/extutil"
)

func StopProcesses(pids []int, force bool) error {
	if len(pids) == 0 {
		return nil
	}

	errs := make([]string, 0)
	for _, pid := range pids {
		if process, err := ps.FindProcess(pid); err == nil && process != nil {
			log.Info().Int("pid", pid).Msg("Stopping process")
			err := stopProcessWindows(pid, force)
			if err != nil {
				errs = append(errs, err.Error())
			}
		} else {
			log.Info().Int("pid", pid).Msg("Process not found")
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("fail to stop processes : %s", strings.Join(errs, ", "))
	}
	return nil
}

func stopProcessWindows(pid int, force bool) error {
	// use absolute path to resolve untrusted search path issue reported  by Sonar
	taskkill, err := exec.LookPath("taskkill.exe")
	if err != nil {
		return fmt.Errorf("fail to find taskkill.exe: %w", err)
	}
	if force {
		err := exec.Command(taskkill, "/F", "/T", "/pid", fmt.Sprintf("%d", pid)).Run()
		if err != nil {
			if isProcessNotFound(err) {
				log.Debug().Int("pid", pid).Msg("process already exited")
				return nil
			}
			return fmt.Errorf("failed to force kill process via taskkill: %w", err)
		}
		return nil
	}

	err = exec.Command(taskkill, "/T", "/pid", fmt.Sprintf("%d", pid)).Run()
	if err != nil {
		if isProcessNotFound(err) {
			log.Debug().Int("pid", pid).Msg("Process already exited")
			return nil
		}
		return fmt.Errorf("failed to kill process via taskkill: %w", err)
	}
	return nil
}

func isProcessNotFound(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 128
}

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
