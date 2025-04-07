// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package stopprocess

import (
	"fmt"
	"github.com/mitchellh/go-ps"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-kit/extutil"
	"os/exec"
	"strings"
)

func StopProcesses(pids []int, force bool) error {
	if len(pids) == 0 {
		return nil
	}

	errors := make([]string, 0)
	for _, pid := range pids {
		if process, err := ps.FindProcess(pid); err == nil && process != nil {
			log.Info().Int("pid", pid).Msg("Stopping process")
			err := stopProcessWindows(pid, force)
			if err != nil {
				errors = append(errors, err.Error())
			}
		} else {
			log.Info().Int("pid", pid).Msg("Process not found")
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("fail to stop processes : %s", strings.Join(errors, ", "))
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
		err := exec.Command(taskkill, "/F", "/pid", fmt.Sprintf("%d", pid)).Run()
		if err != nil {
			return fmt.Errorf("failed to force kill process via taskkill: %w", err)
		}
		return nil
	}

	err = exec.Command(taskkill, "/pid", fmt.Sprintf("%d", pid)).Run()
	if err != nil {
		return fmt.Errorf("failed to kill process via taskkill: %w", err)
	}
	return err
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
