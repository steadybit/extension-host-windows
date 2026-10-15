// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package exthostwindows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/steadybit/extension-host-windows/config"
	"github.com/steadybit/extension-host-windows/exthostwindows/network"
	"net"
	"slices"
	"time"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

func NewNetworkBlackholeContainerAction() action_kit_sdk.Action[NetworkActionState] {
	return &networkAction{
		optsProvider: blackhole(),
		optsDecoder:  blackholeDecode,
		description:  getNetworkBlackholeDescription(),
	}
}

func getNetworkBlackholeDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.network_blackhole", BaseActionID),
		Label:       "Block Traffic",
		Description: "Block network traffic (incoming and outgoing).",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        new(blackHoleIcon),
		TargetSelection: &action_kit_api.TargetSelection{
			TargetType:         targetID,
			SelectionTemplates: &targetSelectionTemplates,
		},
		Technology:  new(WindowsHostTechnology),
		Category:    new("Network"),
		Kind:        action_kit_api.Attack,
		TimeControl: action_kit_api.TimeControlExternal,
		Parameters: append(
			commonNetworkParameters,
			networkInterfaceParameter,
			action_kit_api.ActionParameter{
				Name:         "blockLocalhost",
				Label:        "Block Localhost Traffic",
				Description:  new("Also block matching traffic on the loopback interface (127.0.0.0/8, ::1), e.g. between two processes on the same host. Enabled by default on Windows, where Block Traffic has always included localhost traffic; disable it to leave localhost traffic untouched."),
				Type:         action_kit_api.ActionParameterTypeBoolean,
				DefaultValue: new("true"),
				Advanced:     new(true),
				Order:        new(107),
			},
		),
	}
}

func blackhole() networkOptsProvider {
	return func(ctx context.Context, request action_kit_api.PrepareActionRequestBody) (network.WinOpts, action_kit_api.Messages, error) {
		_, err := CheckTargetHostname(request.Target.Attributes)
		if err != nil {
			return nil, nil, err
		}

		duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond
		if duration < time.Second {
			return nil, nil, errors.New("duration must be greater / equal than 1s")
		}

		filter, messages, err := mapToNetworkFilter(ctx, request.Config, getRestrictedEndpoints(request))
		if err != nil {
			return nil, nil, err
		}

		// Block incoming and outgoing traffic
		filter.Direction = network.DirectionAll

		blockLocalhost := blockLocalhostFromConfig(request.Config)
		// selecting a loopback interface only makes sense for loopback traffic
		filter.ExcludeLoopback = !blockLocalhost && !selectsLoopbackInterface(extutil.ToStringArray(request.Config["networkInterface"]))
		if !filter.ExcludeLoopback {
			filter.Exclude = append(filter.Exclude, localhostExcludesForOwnPorts()...)
		}

		return &network.BlackholeOpts{
			Filter:   filter,
			Duration: duration,
		}, messages, nil
	}
}

func blackholeDecode(data json.RawMessage) (network.WinOpts, error) {
	var opts network.BlackholeOpts
	err := json.Unmarshal(data, &opts)
	return &opts, err
}

// blockLocalhostFromConfig defaults to true when the parameter is missing, e.g. for
// experiments saved before it existed: Block Traffic on Windows always blocked
// matching localhost traffic.
func blockLocalhostFromConfig(config map[string]any) bool {
	v, ok := config["blockLocalhost"]
	if !ok || v == nil {
		return true
	}
	return extutil.ToBool(v)
}

func selectsLoopbackInterface(interfaces []string) bool {
	loopback := akn.GetLoopbackNetworkInterfaces()
	return slices.ContainsFunc(interfaces, func(name string) bool {
		return slices.Contains(loopback, name)
	})
}

// localhostExcludesForOwnPorts keeps the extension reachable via localhost (e.g. an
// agent on the same host calling http://localhost:8085) when localhost traffic is
// blocked.
func localhostExcludesForOwnPorts() []akn.NetWithPortRange {
	loopback := []net.IPNet{
		{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)},
		{IP: net.IPv6loopback, Mask: net.CIDRMask(128, 128)},
	}
	ports := []akn.PortRange{{From: config.Config.Port, To: config.Config.Port}}
	if config.Config.HealthPort > 0 && config.Config.HealthPort != config.Config.Port {
		ports = append(ports, akn.PortRange{From: config.Config.HealthPort, To: config.Config.HealthPort})
	}
	excludes := akn.NewNetWithPortRanges(loopback, ports...)
	for i := range excludes {
		excludes[i].Comment = "ext. port (localhost)"
	}
	return excludes
}
