// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows"
)

func IsExecutableOperational(executableName string, args ...string) error {
	cmd := exec.Command(executableName, args...)
	cmd.Dir = os.TempDir()
	var outputBuffer bytes.Buffer
	cmd.Stdout = &outputBuffer
	cmd.Stderr = &outputBuffer
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start '%s': %s \n'%s' is not installed or not present in %%PATH%%", executableName, err, executableName)
	}
	success := cmd.ProcessState.Success()
	if !success {
		return fmt.Errorf("%s is not operational: '%s' in %v returned: %v", executableName, executableName, os.TempDir(), outputBuffer.Bytes())
	}

	return nil
}

func PowershellCommand(args ...string) *exec.Cmd {
	args = append([]string{"-Command"}, args...)
	return exec.Command("powershell", args...)
}

func GetAvailableDriveLetters() ([]string, error) {
	cmd := PowershellCommand("Get-Volume | ForEach-Object { $_.DriveLetter }")
	output, err := cmd.Output()
	if err != nil {
		return []string{}, err
	}

	reader := strings.NewReader(string(output))
	scanner := bufio.NewScanner(reader)
	driveLetters := make([]string, 0)

	for scanner.Scan() {
		line := scanner.Text()
		letter := strings.TrimSpace(line)
		driveLetters = append(driveLetters, letter)
	}
	return driveLetters, nil
}

// GetDriveSpace returns the free and the total bytes of the given drive.
func GetDriveSpace(driveLetter string) (free uint64, total uint64, err error) {
	root, err := windows.UTF16PtrFromString(driveLetter + `:\`)
	if err != nil {
		return 0, 0, err
	}

	var freeToCaller uint64
	if err := windows.GetDiskFreeSpaceEx(root, &freeToCaller, &total, &free); err != nil {
		return 0, 0, fmt.Errorf("failed to read the space of drive %s: %w", driveLetter, err)
	}
	return free, total, nil
}

type CmdOutputProvider func() ([]byte, error)

func IsTestSigningEnabled() (bool, error) {
	cmd := exec.Command(SystemExecutable("bcdedit.exe"))
	isEnabled, err := CheckTestSigningAttribute(cmd.Output)

	if err != nil {
		return false, err
	}

	return isEnabled, nil
}

func CheckTestSigningAttribute(provider CmdOutputProvider) (bool, error) {
	output, err := provider()

	if err != nil {
		return false, err
	}

	reader := bytes.NewReader(output)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "testsigning") && strings.Contains(line, "Yes") {
			return true, nil
		}
	}

	return false, nil
}
