// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-host-windows/exthostwindows/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func networkPrepareRequest(hostname string, config map[string]any) action_kit_api.PrepareActionRequestBody {
	return action_kit_api.PrepareActionRequestBody{
		Config:      config,
		ExecutionId: uuid.New(),
		Target: &action_kit_api.Target{
			Attributes: map[string][]string{hostNameAttribute: {hostname}},
		},
	}
}

func TestNetworkCorruptPackages_OptsProvider(t *testing.T) {
	osHostname = func() (string, error) { return "myhostname", nil }

	t.Run("success", func(t *testing.T) {
		opts, _, err := corruptPackages()(t.Context(), networkPrepareRequest("myhostname", map[string]any{
			"duration":          "10000",
			"networkCorruption": 25,
			"ip":                []any{"1.1.1.1"},
		}))
		require.NoError(t, err)

		corruptOpts, ok := opts.(*network.CorruptPackagesOpts)
		require.True(t, ok)
		assert.Equal(t, uint(25), corruptOpts.Corruption)
		assert.Equal(t, 10*time.Second, corruptOpts.Duration)
		assert.Equal(t, network.DirectionOutgoing, corruptOpts.Direction)
		require.Len(t, corruptOpts.Include, 1)
		assert.Equal(t, "1.1.1.1/32", corruptOpts.Include[0].Net.String())
	})

	t.Run("wrong host", func(t *testing.T) {
		_, _, err := corruptPackages()(t.Context(), networkPrepareRequest("otherhost", map[string]any{
			"duration": "10000",
			"ip":       []any{"1.1.1.1"},
		}))
		assert.ErrorContains(t, err, "is not the current host")
	})

	t.Run("duration too short", func(t *testing.T) {
		_, _, err := corruptPackages()(t.Context(), networkPrepareRequest("myhostname", map[string]any{
			"duration": "500",
			"ip":       []any{"1.1.1.1"},
		}))
		assert.ErrorContains(t, err, "duration must be greater / equal than 1s")
	})

	t.Run("invalid port", func(t *testing.T) {
		_, _, err := corruptPackages()(t.Context(), networkPrepareRequest("myhostname", map[string]any{
			"duration": "10000",
			"port":     []any{"not-a-port"},
		}))
		assert.Error(t, err)
	})
}

func TestNetworkPackageLoss_OptsProvider(t *testing.T) {
	osHostname = func() (string, error) { return "myhostname", nil }

	t.Run("success", func(t *testing.T) {
		opts, _, err := packageLoss()(t.Context(), networkPrepareRequest("myhostname", map[string]any{
			"duration":   "20000",
			"percentage": 50,
			"ip":         []any{"1.1.1.0/24"},
		}))
		require.NoError(t, err)

		lossOpts, ok := opts.(*network.PackageLossOpts)
		require.True(t, ok)
		assert.Equal(t, uint(50), lossOpts.Loss)
		assert.Equal(t, 20*time.Second, lossOpts.Duration)
		assert.Equal(t, network.DirectionOutgoing, lossOpts.Direction)
		require.Len(t, lossOpts.Include, 1)
		assert.Equal(t, "1.1.1.0/24", lossOpts.Include[0].Net.String())
	})

	t.Run("wrong host", func(t *testing.T) {
		_, _, err := packageLoss()(t.Context(), networkPrepareRequest("otherhost", map[string]any{
			"duration": "10000",
			"ip":       []any{"1.1.1.1"},
		}))
		assert.ErrorContains(t, err, "is not the current host")
	})

	t.Run("duration too short", func(t *testing.T) {
		_, _, err := packageLoss()(t.Context(), networkPrepareRequest("myhostname", map[string]any{
			"duration": "0",
			"ip":       []any{"1.1.1.1"},
		}))
		assert.ErrorContains(t, err, "duration must be greater / equal than 1s")
	})

	t.Run("invalid port", func(t *testing.T) {
		_, _, err := packageLoss()(t.Context(), networkPrepareRequest("myhostname", map[string]any{
			"duration": "10000",
			"port":     []any{"not-a-port"},
		}))
		assert.Error(t, err)
	})
}

func TestNetworkCorruptAndLoss_DecodeRoundTrip(t *testing.T) {
	corrupt := &network.CorruptPackagesOpts{Corruption: 10, Duration: 5 * time.Second, FilterFile: `C:\temp\filter.txt`}
	data, err := json.Marshal(corrupt)
	require.NoError(t, err)
	decodedCorrupt, err := corruptPackagesDecode(data)
	require.NoError(t, err)
	assert.Equal(t, corrupt, decodedCorrupt)

	loss := &network.PackageLossOpts{Loss: 20, Duration: 5 * time.Second}
	data, err = json.Marshal(loss)
	require.NoError(t, err)
	decodedLoss, err := packageLossDecode(data)
	require.NoError(t, err)
	assert.Equal(t, loss, decodedLoss)

	_, err = corruptPackagesDecode([]byte("not json"))
	assert.Error(t, err)
	_, err = packageLossDecode([]byte("not json"))
	assert.Error(t, err)
}

func TestNetworkCorruptAndLoss_Describe(t *testing.T) {
	corrupt := NewNetworkCorruptPackagesContainerAction().Describe()
	assert.Equal(t, BaseActionID+".network_package_corruption", corrupt.Id)
	assert.Equal(t, action_kit_api.Attack, corrupt.Kind)

	loss := NewNetworkPackageLossContainerAction().Describe()
	assert.Equal(t, BaseActionID+".network_package_loss", loss.Id)
	assert.Equal(t, action_kit_api.Attack, loss.Kind)
}
