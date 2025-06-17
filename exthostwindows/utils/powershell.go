package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

func IsProcessRunning(processName string) (bool, error) {
	cmd := PowershellCommand("Get-Process", "-Name", processName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "Cannot find a process with the name") {
			return false, nil
		}

		return false, err
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

func StopProcess(processName string) error {
	isRunning, err := IsProcessRunning(processName)

	if err != nil {
		return err
	}

	if isRunning {
		cmd := PowershellCommand("Stop-Process", "-Name", processName, "-Force")
		out, err := cmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(out), "Cannot find a process with the name") {
				log.Err(err).Msg("Stop-Process failed")
				return err
			}
		}

		log.Info().Msgf("%s", out)
	}

	return nil
}

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

type DriveSpace string

const (
	Available DriveSpace = "SizeRemaining"
	Total     DriveSpace = "Size"
)

func GetDriveSpace(driveLetter string, kind DriveSpace) (uint64, error) {
	cmd := PowershellCommand(fmt.Sprintf("(Get-Volume -DriveLetter %s).%s", driveLetter, kind))
	output, err := cmd.Output()

	if err != nil {
		return 0, err
	}

	availableSpace, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 0)

	if err != nil {
		return 0, err
	}

	return availableSpace, nil
}
