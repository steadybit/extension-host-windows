package utils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_PSPowershellCommand(t *testing.T) {
	arg0 := "test"
	arg1 := "--arg"
	cmd := PowershellCommand(arg0, arg1)
	require.Len(t, cmd.Args, 4)
	require.Contains(t, cmd.Args, arg0)
	require.Contains(t, cmd.Args, arg1)
}

func Test_PSIsProcessRunning_ProcessExists(t *testing.T) {
	isRunning, err := IsProcessRunning("explorer")
	require.NoError(t, err)
	require.True(t, isRunning)
}

func Test_PSIsProcessRunning_NoProcess(t *testing.T) {
	isRunning, err := IsProcessRunning("explorer-wsad")
	require.NoError(t, err)
	require.False(t, isRunning)
}

func Test_PSStopProcess_NoProcess(t *testing.T) {
	err := StopProcess("explorer-wsad")
	require.NoError(t, err)
}

func Test_PSStopProcess(t *testing.T) {
	err := StopProcess("explorer")
	require.NoError(t, err)
}

func Test_PSIsExecutableOperational_Yes(t *testing.T) {
	err := IsExecutableOperational("powershell", "-h")
	require.NoError(t, err)
}

func Test_PSIsExecutableOperational_No(t *testing.T) {
	err := IsExecutableOperational("powershell", "test")
	require.Error(t, err)
}

func Test_PSGetAvailableDriveLetters_AtLeastOne(t *testing.T) {
	driveLetters, err := GetAvailableDriveLetters()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(driveLetters), 1)
}

func Test_PSGetDriveSpace_Available(t *testing.T) {
	driveLetters, err := GetAvailableDriveLetters()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(driveLetters), 1)
	space, err := GetDriveSpace(driveLetters[0], Available)
	require.NoError(t, err)
	require.Greater(t, space, uint64(0))
}

func Test_PSGetDriveSpace_Total(t *testing.T) {
	driveLetters, err := GetAvailableDriveLetters()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(driveLetters), 1)
	space, err := GetDriveSpace(driveLetters[0], Available)
	require.NoError(t, err)
	require.Greater(t, space, uint64(0))
	totalSpace, err := GetDriveSpace(driveLetters[0], Total)
	require.NoError(t, err)
	require.Greater(t, totalSpace, space)
}

func Test_IsTestSigningEnabled_Enabled(t *testing.T) {
	mockOutput := []byte(`
	Windows Boot Manager
	--------------------
	identifier              {bootmgr}
	device                  partition=\Device\HarddiskVolume1
    testsigning             Yes
    `)

	result, err := CheckTestSigningAttribute(func() ([]byte, error) {
		return mockOutput, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected true, got false")
	}
}

func Test_IsTestSigningEnabled_Disabled(t *testing.T) {
	mockOutput := []byte(`
	Windows Boot Manager
	--------------------
	identifier              {bootmgr}
	device                  partition=\Device\HarddiskVolume1
    testsigning             No
    `)

	result, err := CheckTestSigningAttribute(func() ([]byte, error) {
		return mockOutput, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected false, got true")
	}
}

func Test_IsTestSigningEnabled_Error(t *testing.T) {
	expectedErr := errors.New("command failed")
	_, err := CheckTestSigningAttribute(func() ([]byte, error) {
		return nil, expectedErr
	})

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}
