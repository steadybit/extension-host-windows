// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/steadybit/extension-host-windows/exthostwindows/network"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

func TestActionNetworkBandwidth_Prepare(t *testing.T) {
	osHostname = func() (string, error) {
		return "myhostname", nil
	}

	networks, err := utils.MapToNetworks(t.Context(), "localhost")
	require.NoError(t, err)

	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError string
		wantedState *network.LimitBandwidthOpts
	}{
		{
			name: "Should return config on hostname",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "10000",
					"bandwidth": "1000mbit",
					"hostname":  []interface{}{"localhost"},
				},
				ExecutionId: uuid.New(),
				Target: &action_kit_api.Target{
					Attributes: map[string][]string{
						hostNameAttribute: {"myhostname"},
					},
				},
			},
			wantedState: &network.LimitBandwidthOpts{
				Bandwidth:    "1000MB",
				IncludeCidrs: networks,
			},
		}, {
			name: "Should return config on ip",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "10000",
					"bandwidth": "1000mbit",
					"ip":        []interface{}{"1.1.1.1", "2.2.2.2"},
				},
				ExecutionId: uuid.New(),
				Target: &action_kit_api.Target{
					Attributes: map[string][]string{
						hostNameAttribute: {"myhostname"},
					},
				},
			},
			wantedState: &network.LimitBandwidthOpts{
				Bandwidth: "1000MB",
				IncludeCidrs: []net.IPNet{
					{IP: net.ParseIP("1.1.1.1"), Mask: net.CIDRMask(32, 32)},
					{IP: net.ParseIP("2.2.2.2"), Mask: net.CIDRMask(32, 32)},
				},
			},
		}, {
			name: "Should return error on missing hostname or IP",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "10000",
					"bandwidth": "1000mbit",
				},
				ExecutionId: uuid.New(),
				Target: &action_kit_api.Target{
					Attributes: map[string][]string{
						hostNameAttribute: {"myhostname"},
					},
				},
			},
			wantedError: "hostname or IP required",
		}, {
			name: "Should return error on too low duration",
			requestBody: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "0",
					"bandwidth": "1000mbit",
					"hostname":  "steadybit.com",
				},
				ExecutionId: uuid.New(),
				Target: &action_kit_api.Target{
					Attributes: map[string][]string{
						hostNameAttribute: {"myhostname"},
					},
				},
			},
			wantedError: "duration is required",
		}, {
			name: "Should return error on restricted endpoint",
			requestBody: action_kit_api.PrepareActionRequestBody{
				ExecutionContext: &action_kit_api.ExecutionContext{
					RestrictedEndpoints: &[]action_kit_api.RestrictedEndpoint{
						{
							Cidr: "1.1.1.1/32",
						},
					},
				},
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "10000",
					"bandwidth": "1000mbit",
					"ip":        []interface{}{"1.1.1.1"},
				},
				ExecutionId: uuid.New(),
				Target: &action_kit_api.Target{
					Attributes: map[string][]string{
						hostNameAttribute: {"myhostname"},
					},
				},
			},
			wantedError: "target 1.1.1.1/32 0 overlaps with restricted endpoint 1.1.1.1/32 0",
		}, {
			name: "Should return error on restricted endpoint with matching port",
			requestBody: action_kit_api.PrepareActionRequestBody{
				ExecutionContext: &action_kit_api.ExecutionContext{
					RestrictedEndpoints: &[]action_kit_api.RestrictedEndpoint{
						{
							Cidr:    "1.1.1.1/32",
							PortMin: 123,
							PortMax: 321,
						},
					},
				},
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "10000",
					"bandwidth": "1000mbit",
					"ip":        []interface{}{"1.1.1.1"},
					"port":      "200",
				},
				ExecutionId: uuid.New(),
				Target: &action_kit_api.Target{
					Attributes: map[string][]string{
						hostNameAttribute: {"myhostname"},
					},
				},
			},
			wantedError: "target 1.1.1.1/32 200 overlaps with restricted endpoint 1.1.1.1/32 123-321",
		}, {
			name: "Should return error on restricted endpoint with matching port range",
			requestBody: action_kit_api.PrepareActionRequestBody{
				ExecutionContext: &action_kit_api.ExecutionContext{
					RestrictedEndpoints: &[]action_kit_api.RestrictedEndpoint{
						{
							Cidr:    "1.1.1.1/32",
							PortMin: 123,
							PortMax: 321,
						},
					},
				},
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "10000",
					"bandwidth": "1000mbit",
					"ip":        []interface{}{"1.1.1.1"},
					"port":      "111-222",
				},
				ExecutionId: uuid.New(),
				Target: &action_kit_api.Target{
					Attributes: map[string][]string{
						hostNameAttribute: {"myhostname"},
					},
				},
			},
			wantedError: "target 1.1.1.1/32 111-222 overlaps with restricted endpoint 1.1.1.1/32 123-321",
		}, {
			name: "Should return config on restricted endpoint with different port range",
			requestBody: action_kit_api.PrepareActionRequestBody{
				ExecutionContext: &action_kit_api.ExecutionContext{
					RestrictedEndpoints: &[]action_kit_api.RestrictedEndpoint{
						{
							Cidr:    "1.1.1.1/32",
							PortMin: 123,
							PortMax: 321,
						},
					},
				},
				Config: map[string]interface{}{
					"action":    "prepare",
					"duration":  "10000",
					"bandwidth": "1000mbit",
					"ip":        []interface{}{"1.1.1.1"},
					"port":      "111-122",
				},
				ExecutionId: uuid.New(),
				Target: &action_kit_api.Target{
					Attributes: map[string][]string{
						hostNameAttribute: {"myhostname"},
					},
				},
			},
			wantedState: &network.LimitBandwidthOpts{
				Bandwidth: "1000MB",
				PortRange: akn.PortRange{
					From: 111,
					To:   122,
				},
				IncludeCidrs: []net.IPNet{
					{IP: net.ParseIP("1.1.1.1"), Mask: net.CIDRMask(32, 32)},
				},
			},
		},
	}

	action := NewNetworkLimitBandwidthContainerAction()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//Given
			state := NetworkActionState{}
			request := tt.requestBody

			//When
			result, err := action.Prepare(context.Background(), &state, request)

			//Then
			if err != nil && tt.wantedError == "" {
				require.NoError(t, err, "No error expected, but got one")
			}

			if tt.wantedError != "" {
				if err != nil {
					assert.EqualError(t, err, tt.wantedError)
				} else if result != nil && result.Error != nil {
					assert.Equal(t, tt.wantedError, result.Error.Title)
				} else {
					assert.Fail(t, "Expected error but no error or result with error was returned")
				}
			}
			if tt.wantedState != nil {
				var opts network.LimitBandwidthOpts
				err := json.Unmarshal(state.NetworkOpts, &opts)
				require.NoError(t, err)

				assert.NoError(t, err)
				assert.Equal(t, tt.wantedState.Bandwidth, opts.Bandwidth)
				assert.True(t, assertContainsCidrs(tt.wantedState.IncludeCidrs, opts.IncludeCidrs), "IncludeCidrs not found in network")
				assert.Equal(t, tt.wantedState.PortRange, opts.PortRange)
			}
		})
	}
}

func assertContainsCidrs(wantedCidrs []net.IPNet, actualCidrs []net.IPNet) bool {
	for _, wantedCidr := range wantedCidrs {
		if !assertContainsCidr(wantedCidr, actualCidrs) {
			return false
		}
	}
	return true
}

func assertContainsCidr(wantedCidr net.IPNet, actualCidrs []net.IPNet) bool {
	for _, actualCidr := range actualCidrs {
		if wantedCidr.String() == actualCidr.String() {
			return true
		}
	}
	return false
}
