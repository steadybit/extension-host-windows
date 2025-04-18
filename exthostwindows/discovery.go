// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"context"
	"github.com/elastic/go-sysinfo"
	"github.com/rs/zerolog/log"
	networkutils "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-host-windows/config"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"os"
	"time"
)

type hostDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*hostDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*hostDiscovery)(nil)
)

func NewHostDiscovery() discovery_kit_sdk.TargetDiscovery {
	discovery := &hostDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 30*time.Second),
	)
}

func (d *hostDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: targetID,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("30s"),
		},
	}
}

func (d *hostDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:      targetID,
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),

		// Labels used in the UI
		Label: discovery_kit_api.PluralLabel{One: "Windows Host", Other: "Windows Hosts"},

		// Category for the targets to appear in
		Category: extutil.Ptr("basic"),

		// Specify attributes shown in table columns and to be used for sorting
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: hostNameAttribute},
				{Attribute: hostIp4Attribute},
				{Attribute: "aws.zone", FallbackAttributes: &[]string{"google.zone", "azure.zone"}},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: hostNameAttribute,
					Direction: "ASC",
				},
			},
		},
	}
}

func (d *hostDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: hostNameAttribute,
			Label: discovery_kit_api.PluralLabel{
				One:   "Hostname",
				Other: "Hostnames",
			},
		}, {
			Attribute: hostDomainnameAttribute,
			Label: discovery_kit_api.PluralLabel{
				One:   "Domainname",
				Other: "Domainnames",
			},
		}, {
			Attribute: hostIp4Attribute,
			Label: discovery_kit_api.PluralLabel{
				One:   "IPv4",
				Other: "IPv4s",
			},
		}, {
			Attribute: hostIpv6Attribute,
			Label: discovery_kit_api.PluralLabel{
				One:   "IPv6",
				Other: "IPv6s",
			},
		}, {
			Attribute: hostNic,
			Label: discovery_kit_api.PluralLabel{
				One:   "NIC",
				Other: "NICs",
			},
		}, {
			Attribute: hostOsFamilyAttribute,
			Label: discovery_kit_api.PluralLabel{
				One:   "OS Family",
				Other: "OS Families",
			},
		}, {
			Attribute: hostOsManufacturer,
			Label: discovery_kit_api.PluralLabel{
				One:   "OS Manufacturer",
				Other: "OS Manufacturers",
			},
		}, {
			Attribute: hostOsVersion,
			Label: discovery_kit_api.PluralLabel{
				One:   "OS Version",
				Other: "OS Versions",
			},
		},
	}
}

func (d *hostDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	hostname, _ := os.Hostname()
	target := discovery_kit_api.Target{
		Id:         hostname,
		TargetType: targetID,
		Label:      hostname,
		Attributes: map[string][]string{
			hostNameAttribute: {hostname},
			hostNic:           networkutils.GetOwnNetworkInterfaces(),
		},
	}

	var ownIpV4s, ownIpV6s []string
	for _, ip := range networkutils.GetOwnIPs() {
		if ipv4 := ip.To4(); ipv4 != nil {
			ownIpV4s = append(ownIpV4s, ipv4.String())
		} else if ipv6 := ip.To16(); ipv6 != nil {
			ownIpV6s = append(ownIpV6s, ipv6.String())
		}
	}
	if len(ownIpV4s) > 0 {
		target.Attributes[hostIp4Attribute] = ownIpV4s
	}
	if len(ownIpV6s) > 0 {
		target.Attributes[hostIpv6Attribute] = ownIpV6s
	}

	if host, err := sysinfo.Host(); err == nil {
		target.Attributes[hostOsFamilyAttribute] = []string{host.Info().OS.Family}
		target.Attributes[hostOsManufacturer] = []string{host.Info().OS.Name}
		target.Attributes[hostOsVersion] = []string{host.Info().OS.Version}

		if fqdn, err := host.FQDNWithContext(ctx); err == nil {
			target.Attributes[hostDomainnameAttribute] = []string{fqdn}
		} else {
			target.Attributes[hostDomainnameAttribute] = []string{host.Info().Hostname}
		}
	} else {
		log.Error().Err(err).Msg("Failed to get host info")
	}

	for key, value := range getEnvironmentVariables() {
		target.Attributes[hostEnv+key] = []string{value}
	}
	for key, value := range getLabels() {
		target.Attributes[hostLabel+key] = []string{value}
	}

	targets := []discovery_kit_api.Target{target}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesHost), nil
}
