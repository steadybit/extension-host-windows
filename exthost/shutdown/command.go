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

type CommandImpl struct{}

func NewCommand() Command {
	return &CommandImpl{}
}

func (c *CommandImpl) IsShutdownCommandExecutable() bool {
	_, err := exec.LookPath("shutdown.exe")
	if err != nil {
		log.Debug().Msgf("Failed to find shutdown.exe %s", err)
		return false
	}
	return true
}

func (c *CommandImpl) getShutdownCommand() []string {
	return []string{"shutdown.exe", "/s", "/t", "0"}
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
	return []string{"shutdown.exe", "/r", "/t", "0"}
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
