package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func IsProcessRunning(processName string) (bool, error) {
	cmd := exec.Command("powershell", "-Command", "Get-Process", "-Name", processName)
	output, err := cmd.Output()
	if err != nil {
		if !strings.Contains(string(output), "Cannot find a process with the name") {
			return false, nil
		}
		return false, err
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
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
	return exec.Command("powershell", "-Command", strings.Join(args, " "))
}
