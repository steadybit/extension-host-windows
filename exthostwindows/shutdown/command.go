// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package shutdown

import (
	"github.com/rs/zerolog/log"
	"os/exec"
)

type Command interface {
	IsShutdownCommandExecutable() bool
	Shutdown() error
	Reboot() error
}

const shutdownExecutableName = "shutdown.exe"

type CommandImpl struct{}

func NewCommand() Command {
	return &CommandImpl{}
}

func (c *CommandImpl) IsShutdownCommandExecutable() bool {
	p, err := exec.LookPath(shutdownExecutableName)
	if err != nil {
		log.Debug().Msgf("Failed to find shutdown.exe %s", err)
		return false
	}
	log.Trace().Msgf("Found shutdown.exe %s", p)
	return true
}

func (c *CommandImpl) getShutdownCommand() []string {
	executable, err := exec.LookPath(shutdownExecutableName)
	if err != nil {
		log.Error().Err(err).Msgf("shutdown command not available")
		return nil
	}
	return []string{executable, "/s", "/t", "0"}
}

func (c *CommandImpl) Shutdown() error {
	cmd := c.getShutdownCommand()
	err := exec.Command(cmd[0], cmd[1:]...).Run()
	if err != nil {
		log.Err(err).Msg("Failed to shutdown")
		return err
	}
	return nil
}

func (c *CommandImpl) getRebootCommand() []string {
	executable, err := exec.LookPath(shutdownExecutableName)
	if err != nil {
		log.Error().Err(err).Msgf("shutdown command not available")
		return nil
	}
	return []string{executable, "/r", "/t", "0"}
}

func (c *CommandImpl) Reboot() error {
	cmd := c.getRebootCommand()
	err := exec.Command(cmd[0], cmd[1:]...).Run()
	if err != nil {
		log.Err(err).Msg("Failed to reboot")
		return err
	}
	return nil
}
