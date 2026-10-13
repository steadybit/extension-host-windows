// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckTargetHostname(t *testing.T) {
	osHostname = func() (string, error) {
		return "myhostname", nil
	}
	type args struct {
		attributes map[string][]string
	}
	tests := []struct {
		name    string
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Should return error when hostname is not set",
			args: args{
				attributes: map[string][]string{
					hostNameAttribute: {},
				},
			},
			wantErr: assert.Error,
		},
		{
			name: "Should return error when hostname is empty",
			args: args{
				attributes: map[string][]string{
					hostNameAttribute: {""},
				},
			},
			wantErr: assert.Error,
		},
		{
			name: "Should return no error",
			args: args{
				attributes: map[string][]string{
					hostNameAttribute: {"myhostname"},
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hn, err := CheckTargetHostname(tt.args.attributes)
			tt.wantErr(t, err, fmt.Sprintf("CheckTargetHostname(%v)", tt.args.attributes))
			if err == nil {
				assert.Equal(t, new("myhostname"), hn)
			}
		})
	}
}

func TestCheckTargetHostname_HostnameLookupFails(t *testing.T) {
	osHostname = func() (string, error) {
		return "", errors.New("boom")
	}
	t.Cleanup(func() { osHostname = os.Hostname })

	hn, err := CheckTargetHostname(map[string][]string{hostNameAttribute: {"myhostname"}})
	assert.ErrorContains(t, err, "Failed to get hostname")
	assert.Nil(t, hn)
}

func TestGetRestrictedEndpoints(t *testing.T) {
	assert.Nil(t, getRestrictedEndpoints(action_kit_api.PrepareActionRequestBody{}))
	assert.Nil(t, getRestrictedEndpoints(action_kit_api.PrepareActionRequestBody{ExecutionContext: &action_kit_api.ExecutionContext{}}))

	endpoints := []action_kit_api.RestrictedEndpoint{{Cidr: "10.0.0.1/32", Name: "agent"}}
	assert.Equal(t, endpoints, getRestrictedEndpoints(action_kit_api.PrepareActionRequestBody{
		ExecutionContext: &action_kit_api.ExecutionContext{RestrictedEndpoints: &endpoints},
	}))
}

func TestResolveExecutable(t *testing.T) {
	const envKey = "STEADYBIT_TEST_EXECUTABLE"
	dir := t.TempDir()
	executable := filepath.Join(dir, "tool.exe")
	require.NoError(t, os.WriteFile(executable, []byte{}, 0700))

	t.Run("env not set", func(t *testing.T) {
		require.NoError(t, os.Unsetenv(envKey))
		assert.Equal(t, "tool", resolveExecutable("tool", envKey))
	})
	t.Run("env points to executable", func(t *testing.T) {
		t.Setenv(envKey, executable)
		assert.Equal(t, executable, resolveExecutable("tool", envKey))
	})
	t.Run("env points to directory", func(t *testing.T) {
		t.Setenv(envKey, dir)
		assert.Equal(t, "tool", resolveExecutable("tool", envKey))
	})
	t.Run("env points to missing file", func(t *testing.T) {
		t.Setenv(envKey, filepath.Join(dir, "missing.exe"))
		assert.Equal(t, "tool", resolveExecutable("tool", envKey))
	})
}

func TestRegisterQosPolicyCleanup_Stop(t *testing.T) {
	stop := RegisterQosPolicyCleanup()
	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup routine did not stop")
	}
}
