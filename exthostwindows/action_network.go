// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-host-windows/exthostwindows/network"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-host-windows/config"
	extensionKit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
)

type networkOptsProvider func(ctx context.Context, request action_kit_api.PrepareActionRequestBody) (network.WinOpts, action_kit_api.Messages, error)

type networkOptsDecoder func(data json.RawMessage) (network.WinOpts, error)

type networkAction struct {
	description  action_kit_api.ActionDescription
	optsProvider networkOptsProvider
	optsDecoder  networkOptsDecoder
}

type NetworkActionState struct {
	NetworkOpts json.RawMessage
}

// Make sure networkAction implements all required interfaces
var _ action_kit_sdk.Action[NetworkActionState] = (*networkAction)(nil)
var _ action_kit_sdk.ActionWithStop[NetworkActionState] = (*networkAction)(nil)

var durationParamter = action_kit_api.ActionParameter{
	Name:         "duration",
	Label:        "Duration",
	Description:  new("How long should the network be affected?"),
	Type:         action_kit_api.ActionParameterTypeDuration,
	DefaultValue: new("30s"),
	Required:     new(true),
	Order:        new(0),
}

var networkInterfaceParameter = action_kit_api.ActionParameter{
	Name:  "networkInterface",
	Label: "Network Interface",
	Description: new("Restrict the attack to these network interfaces. All interfaces are affected if none is specified. " +
		"Expects the interface name (alias) exactly as Windows shows it, e.g. Ethernet, Ethernet 2, Wi-Fi or vEthernet (WSL). " +
		"Enter names containing spaces or parentheses as they are, without quotes; upper and lower case do not matter. " +
		"The names are listed in the host.nic attribute of the target. On the host, get them with the PowerShell command " +
		"Get-NetIPInterface | Select-Object InterfaceAlias -Unique (or Get-NetAdapter, which omits the loopback interface). " +
		"Localhost traffic flows through Loopback Pseudo-Interface 1."),
	Type:     action_kit_api.ActionParameterTypeStringArray,
	Required: new(false),
	Options: new([]action_kit_api.ParameterOption{
		action_kit_api.ParameterOptionsFromTargetAttribute{Attribute: hostNicAttribute},
	}),
	Group: new(groupTrafficFilter),
	Order: new(107),
}

var commonNetworkParameters = []action_kit_api.ActionParameter{
	durationParamter,
	{
		Name:         "hostname",
		Label:        "Include Hostnames",
		Description:  new("Restrict to/from which hosts the traffic is affected."),
		Type:         action_kit_api.ActionParameterTypeStringArray,
		DefaultValue: new(""),
		Advanced:     new(true),
		Group:        new(groupTrafficFilter),
		Order:        new(101),
	},
	{
		Name:         "ip",
		Label:        "Include IPs/CIDRs",
		Description:  new("Restrict to/from which IP addresses or blocks the traffic is affected."),
		Type:         action_kit_api.ActionParameterTypeStringArray,
		DefaultValue: new(""),
		Advanced:     new(true),
		Group:        new(groupTrafficFilter),
		Order:        new(102),
	},
	{
		Name:         "port",
		Label:        "Include Ports",
		Description:  new("Restrict to/from which ports the traffic is affected."),
		Type:         action_kit_api.ActionParameterTypeStringArray,
		DefaultValue: new(""),
		Advanced:     new(true),
		Group:        new(groupTrafficFilter),
		Order:        new(103),
	},
	{
		Name:        "excludeHostname",
		Label:       "Exclude Hostnames",
		Description: new("Exclude traffic to/from these hosts from being affected. Excludes always take precedence over the include restrictions above (hostnames, IPs/CIDRs, ports)."),
		Type:        action_kit_api.ActionParameterTypeStringArray,
		Required:    new(false),
		Advanced:    new(true),
		Group:       new(groupTrafficFilter),
		Order:       new(104),
	},
	{
		Name:        "excludeIp",
		Label:       "Exclude IPs/CIDRs",
		Description: new("Exclude traffic to/from these IP addresses or CIDR blocks from being affected. Excludes always take precedence over the include restrictions above (hostnames, IPs/CIDRs, ports), e.g. affect all traffic except 10.0.0.0/8."),
		Type:        action_kit_api.ActionParameterTypeStringArray,
		Required:    new(false),
		Advanced:    new(true),
		Group:       new(groupTrafficFilter),
		Order:       new(105),
	},
	{
		Name:        "excludePort",
		Label:       "Exclude Ports",
		Description: new("Exclude traffic to/from these ports or port ranges (e.g. 3389 or 8000-8100) from being affected, whatever the address. Excludes always take precedence over the include restrictions above (hostnames, IPs/CIDRs, ports), e.g. affect all traffic except RDP on port 3389."),
		Type:        action_kit_api.ActionParameterTypeStringArray,
		Required:    new(false),
		Advanced:    new(true),
		Group:       new(groupTrafficFilter),
		Order:       new(106),
	},
}

func (a *networkAction) NewEmptyState() NetworkActionState {
	return NetworkActionState{}
}

func (a *networkAction) Describe() action_kit_api.ActionDescription {
	return a.description
}

func (a *networkAction) Prepare(ctx context.Context, state *NetworkActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	_, err := CheckTargetHostname(request.Target.Attributes)
	if err != nil {
		return nil, err
	}

	opts, messages, err := a.optsProvider(ctx, request)
	if err != nil {
		return nil, extensionKit.WrapError(err)
	}

	if messages == nil {
		messages = []action_kit_api.Message{} // prevent empty messages response
	}

	rawOpts, err := json.Marshal(opts)
	if err != nil {
		return nil, extensionKit.ToError("Failed to serialize network settings.", err)
	}

	state.NetworkOpts = rawOpts

	return &action_kit_api.PrepareResult{Messages: &messages}, nil
}

func (a *networkAction) Start(ctx context.Context, state *NetworkActionState) (*action_kit_api.StartResult, error) {
	start := time.Now()
	defer func() { log.Debug().Dur("total", time.Since(start)).Msg("network action started") }()

	opts, err := a.optsDecoder(state.NetworkOpts)
	if err != nil {
		return nil, extensionKit.ToError("Failed to deserialize network settings.", err)
	}

	result := action_kit_api.StartResult{Messages: &action_kit_api.Messages{
		{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: opts.String(),
		},
	}}

	err = network.Apply(ctx, opts)
	if err != nil {
		return &result, extensionKit.ToError("Failed to apply network settings.", err)
	}

	return &result, nil
}

func (a *networkAction) Stop(ctx context.Context, state *NetworkActionState) (*action_kit_api.StopResult, error) {
	start := time.Now()
	defer func() { log.Debug().Dur("total", time.Since(start)).Msg("network action stopped") }()

	opts, err := a.optsDecoder(state.NetworkOpts)
	if err != nil {
		return nil, extensionKit.ToError("Failed to deserialize network settings.", err)
	}

	if err := network.Revert(ctx, opts); err != nil {
		return nil, extensionKit.ToError("Failed to revert network settings.", err)
	}

	return nil, nil
}

func mapToNetworkFilter(ctx context.Context, actionConfig map[string]any, restrictedEndpoints []action_kit_api.RestrictedEndpoint) (network.Filter, action_kit_api.Messages, error) {
	ipsAndHosts := append(
		extutil.ToStringArray(actionConfig["ip"]),
		extutil.ToStringArray(actionConfig["hostname"])...,
	)
	includeCidrs, err := utils.MapToNetworks(ctx, ipsAndHosts...)
	if err != nil {
		return network.Filter{}, nil, err
	}

	//if no hostname/ip specified we affect all ips
	if len(includeCidrs) == 0 {
		includeCidrs = akn.NetAny
	}

	portRanges, err := utils.ParsePortRanges(extutil.ToStringArray(actionConfig["port"]))
	if err != nil {
		return network.Filter{}, nil, err
	}
	if len(portRanges) == 0 {
		//if no hostname/ip specified we affect all ports
		portRanges = []akn.PortRange{akn.PortRangeAny}
	}

	includes := akn.NewNetWithPortRanges(includeCidrs, portRanges...)
	for i := range includes {
		includes[i].Comment = "parameters"
	}

	excludes, err := toExcludes(restrictedEndpoints)
	if err != nil {
		return network.Filter{}, nil, err
	}

	excludeCidrs, err := utils.MapToNetworks(ctx, append(
		extutil.ToStringArray(actionConfig["excludeIp"]),
		extutil.ToStringArray(actionConfig["excludeHostname"])...,
	)...)
	if err != nil {
		return network.Filter{}, nil, err
	}
	userExcludes := akn.NewNetWithPortRanges(excludeCidrs, akn.PortRangeAny)
	for i := range userExcludes {
		userExcludes[i].Comment = "parameters"
	}
	excludes = append(excludes, userExcludes...)

	excludePortRanges, err := utils.ParsePortRanges(extutil.ToStringArray(actionConfig["excludePort"]))
	if err != nil {
		return network.Filter{}, nil, err
	}
	// An excluded port is spared on every address.
	portExcludes := akn.NewNetWithPortRanges(akn.NetAny, excludePortRanges...)
	for i := range portExcludes {
		portExcludes[i].Comment = "parameters"
	}
	excludes = append(excludes, portExcludes...)

	excludes = append(excludes, akn.ComputeExcludesForOwnIpAndPorts(config.Config.Port, config.Config.HealthPort)...)

	messages := []action_kit_api.Message{} // make sure messages is not nil
	excludes, condensed := condenseExcludes(excludes)
	if condensed {
		messages = append(messages, action_kit_api.Message{
			Level: extutil.Ptr(action_kit_api.Warn),
			Message: "Some excludes (to protect agent and extensions) were aggregated to reduce the number of commands necessary." +
				"This may lead to less specific exclude rules, some traffic might not be affected, as expected. " +
				"You can avoid this by configuring a more specific attack (e.g. by specifying ports or CIDRs).",
		})
	}

	interfaceIndexes, err := networkInterfaceIndexes(extutil.ToStringArray(actionConfig["networkInterface"]))
	if err != nil {
		return network.Filter{}, nil, err
	}

	return network.Filter{
		Include:          includes,
		Exclude:          excludes,
		InterfaceIndexes: interfaceIndexes,
	}, messages, nil
}

func condenseExcludes(excludes []akn.NetWithPortRange) ([]akn.NetWithPortRange, bool) {
	l := len(excludes)
	excludes = utils.CondenseNetWithPortRange(excludes, 500)
	return excludes, l != len(excludes)
}

func toExcludes(restrictedEndpoints []action_kit_api.RestrictedEndpoint) ([]akn.NetWithPortRange, error) {
	var excludes []akn.NetWithPortRange

	for _, restrictedEndpoint := range restrictedEndpoints {
		_, cidr, err := net.ParseCIDR(restrictedEndpoint.Cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid cidr %s: %w", restrictedEndpoint.Cidr, err)
		}

		portRange := akn.PortRangeAny
		if restrictedEndpoint.PortMin != 0 || restrictedEndpoint.PortMax != 0 {
			portRange = akn.PortRange{From: uint16(restrictedEndpoint.PortMin), To: uint16(restrictedEndpoint.PortMax)}
		}

		nwps := akn.NewNetWithPortRanges([]net.IPNet{*cidr}, portRange)
		for i := range nwps {
			var sb strings.Builder
			if restrictedEndpoint.Name != "" {
				sb.WriteString(restrictedEndpoint.Name)
				sb.WriteString(" ")
			}
			if restrictedEndpoint.Url != "" {
				sb.WriteString(restrictedEndpoint.Url)
			}
			nwps[i].Comment = strings.TrimSpace(sb.String())
		}

		excludes = append(excludes, nwps...)
	}
	return excludes, nil
}

// LogTestSigningStatus records whether test signing is enabled, which helps to diagnose WinDivert driver load failures.
func LogTestSigningStatus() {
	if isTestSigningEnabled, err := utils.IsTestSigningEnabled(); err != nil {
		log.Warn().Err(err).Msg("failed retrieving testsigning flag from the bcdedit.")
	} else {
		log.Info().Bool("testsigning", isTestSigningEnabled).Msg("checked testsigning flag of the machine")
	}
}

// networkInterfaceIndexes fails on names that do not exist on this host: without a matching interface the filter
// would not restrict the interfaces at all, and the attack would hit every interface.
func networkInterfaceIndexes(names []string) ([]int, error) {
	if len(names) == 0 {
		return nil, nil
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}
	return resolveInterfaceIndexes(names, ifaces)
}

// resolveInterfaceIndexes maps interface names to their indexes. Windows interface names (aliases) commonly contain
// spaces and parentheses, e.g. "Ethernet 2" or "vEthernet (WSL)"; they are matched as a whole. Like Windows itself,
// the match ignores case, and surrounding whitespace is dropped.
func resolveInterfaceIndexes(names []string, ifaces []net.Interface) ([]int, error) {
	indexByName := make(map[string]int, len(ifaces))
	for _, iface := range ifaces {
		indexByName[strings.ToLower(iface.Name)] = iface.Index
	}

	var indexes []int
	var missing []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		index, ok := indexByName[strings.ToLower(name)]
		if !ok {
			missing = append(missing, strconv.Quote(name))
			continue
		}
		if !slices.Contains(indexes, index) {
			indexes = append(indexes, index)
		}
	}
	if len(missing) > 0 {
		available := make([]string, 0, len(ifaces))
		for _, iface := range ifaces {
			available = append(available, strconv.Quote(iface.Name))
		}
		return nil, fmt.Errorf("network interface %s not found, available: %s", strings.Join(missing, ", "), strings.Join(available, ", "))
	}
	return indexes, nil
}
