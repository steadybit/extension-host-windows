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
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
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
			Attribute: hostNicAttribute,
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
			Attribute: hostOsManufacturerAttribute,
			Label: discovery_kit_api.PluralLabel{
				One:   "OS Manufacturer",
				Other: "OS Manufacturers",
			},
		}, {
			Attribute: hostOsVersionAttribute,
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
			hostNicAttribute:  networkutils.GetOwnNetworkInterfaces(),
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
		target.Attributes[hostOsManufacturerAttribute] = []string{host.Info().OS.Name}
		target.Attributes[hostOsVersionAttribute] = []string{host.Info().OS.Version}

		if fqdn, err := host.FQDNWithContext(ctx); err == nil {
			target.Attributes[hostDomainnameAttribute] = []string{fqdn}
		} else {
			target.Attributes[hostDomainnameAttribute] = []string{host.Info().Hostname}
		}
	} else {
		log.Error().Err(err).Msg("Failed to get host info")
	}

	for key, value := range getEnvironmentVariables() {
		target.Attributes[hostEnvAttributePrefix+key] = []string{value}
	}
	for key, value := range getLabels() {
		target.Attributes[hostLabelAttributePrefix+key] = []string{value}
	}

	if id := awsInstanceId(ctx); id != "" {
		target.Attributes[awsInstanceIdAttribute] = []string{id}
	} else if id := gcpInstanceId(ctx); id != "" {
		target.Attributes[gcpInstanceIdAttribute] = []string{id}
	} else if id := azureInstanceId(ctx); id != "" {
		target.Attributes[azureInstanceIdAttribute] = []string{id}
	}

	targets := []discovery_kit_api.Target{target}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesHost), nil
}

func awsInstanceId(ctx context.Context) string {
	if awsEnv := os.Getenv("AWS_EXECUTION_ENV"); awsEnv == "" {
		return ""
	}
	commands := []string{
		"$token = Invoke-RestMethod -Method PUT -Headers @{'X-aws-ec2-metadata-token-ttl-seconds' = '60'} http://169.254.169.254/latest/api/token",
		"Invoke-RestMethod -Method GET -Headers @{'X-aws-ec2-metadata-token' = $token} http://169.254.169.254/latest/meta-data/instance-id",
	}
	instanceId, err := utils.ExecutePowershellCommand(ctx, commands, utils.PSRun)
	if err != nil {
		log.Error().Err(err).Msg("failed to retrieve AWS EC2 instance id")
		return ""
	}
	return instanceId
}

func gcpInstanceId(ctx context.Context) string {
	command := "if ((Get-WmiObject Win32_BIOS).Manufacturer -eq 'Google') { Invoke-RestMethod -Headers @{'Metadata-Flavor' = 'Google'} -Uri 'http://metadata.google.internal/computeMetadata/v1/instance/id' }"
	instanceId, err := utils.ExecutePowershellCommand(ctx, []string{command}, utils.PSRun)
	if err != nil {
		log.Error().Err(err).Msg("failed to retrieve GCP instance id")
		return ""
	}
	return instanceId
}

func azureInstanceId(ctx context.Context) string {
	commands := []string{
		"$sys=Get-CimInstance Win32_ComputerSystem",
		"$bios=Get-CimInstance Win32_BIOS",
		"if($sys.Manufacturer -eq 'Microsoft Corporation' -and $sys.Model -eq 'Virtual Machine' -and $bios.Manufacturer -eq 'Microsoft Corporation') { " +
			"try{(Invoke-RestMethod -Headers @{Metadata='true'} -Uri 'http://169.254.169.254/metadata/instance?api-version=2021-02-01').compute.vmId} catch{''}} else {''}",
	}
	instanceId, err := utils.ExecutePowershellCommand(ctx, commands, utils.PSRun)
	if err != nil {
		log.Error().Err(err).Msg("failed to retrieve Azure instance id")
		return ""
	}
	return instanceId
}
