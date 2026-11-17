// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows"
)

type HostnameResolver struct {
}

type HostnameInput struct {
	Records  []string
	Hostname string
}

type HostnameOutput struct {
	IPAddresses []net.IP
}

var defaultHostnameResolver = &HostnameResolver{}

func Resolve(ctx context.Context, hostnames ...string) ([]net.IP, error) {
	return defaultHostnameResolver.Resolve(ctx, hostnames...)
}

func (h *HostnameResolver) Resolve(ctx context.Context, hostnames ...string) ([]net.IP, error) {
	if len(hostnames) == 0 {
		return nil, nil
	}

	unresolved := make([]string, 0, len(hostnames))
	resolved := make([]net.IP, 0)
	var toResolve []HostnameInput
	var invalid []string

	for _, hostname := range hostnames {
		if len(strings.TrimSpace(hostname)) == 0 {
			invalid = append(invalid, hostname)
			continue
		}
		toResolve = append(toResolve, HostnameInput{
			Hostname: hostname,
			Records:  []string{"A", "AAAA"},
		})
		unresolved = append(unresolved, hostname)
	}

	if len(invalid) > 0 {
		return nil, fmt.Errorf("could not resolve hostnames: '%s'", strings.Join(invalid, "', '"))
	}

	for _, hostnameInput := range toResolve {
		output, err := hostnameInput.Resolve(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not resolve hostname '%s': %w", hostnameInput.Hostname, err)
		}

		if len(output.IPAddresses) == 0 {
			// unknown names yield no addresses; keep it in unresolved to report it by name
			continue
		}

		unresolved = slices.DeleteFunc(unresolved, func(hostname string) bool {
			return hostname == hostnameInput.Hostname
		})

		resolved = append(resolved, output.IPAddresses...)
	}

	if len(unresolved) > 0 {
		return nil, fmt.Errorf("could not resolve hostnames: '%s'", strings.Join(unresolved, "', '"))
	}

	log.Trace().Interface("resolved", resolved).Strs("hostnames", hostnames).Msg("resolved resolved")
	return resolved, nil
}

// dnsFreeRecordList is DnsFreeRecordList from windns.h, the free type for record lists returned by DnsQuery.
const dnsFreeRecordList = 1

var dnsRecordTypes = map[string]uint16{
	"A":    windows.DNS_TYPE_A,
	"AAAA": windows.DNS_TYPE_AAAA,
}

// Resolve queries the Windows DNS client like Resolve-DnsName does, including the hosts file, the DNS cache and
// LLMNR/NetBIOS for local names.
func (i *HostnameInput) Resolve(_ context.Context) (*HostnameOutput, error) {
	hostnameOutput := HostnameOutput{
		IPAddresses: make([]net.IP, 0),
	}

	for _, record := range i.Records {
		qtype, ok := dnsRecordTypes[record]
		if !ok {
			return nil, fmt.Errorf("unsupported DNS record type %s", record)
		}
		hostnameOutput.IPAddresses = append(hostnameOutput.IPAddresses, queryDns(i.Hostname, qtype)...)
	}

	return &hostnameOutput, nil
}

// queryDns returns the A and AAAA addresses of the answer, ignoring CNAME and other records. A failed query, e.g. for
// an unknown name or a name without records of the type, yields no addresses.
func queryDns(hostname string, qtype uint16) []net.IP {
	var records *windows.DNSRecord
	if err := windows.DnsQuery(hostname, qtype, 0, nil, &records, nil); err != nil {
		log.Debug().Err(err).Str("hostname", hostname).Uint16("type", qtype).Msg("DNS query returned no records")
		return nil
	}
	defer windows.DnsRecordListFree(records, dnsFreeRecordList)

	var ips []net.IP
	for r := records; r != nil; r = r.Next {
		switch r.Type {
		case windows.DNS_TYPE_A:
			ips = append(ips, slices.Clone(net.IP(r.Data[:net.IPv4len])))
		case windows.DNS_TYPE_AAAA:
			ips = append(ips, slices.Clone(net.IP(r.Data[:net.IPv6len])))
		}
	}
	return ips
}
