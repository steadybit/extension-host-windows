// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func getFamily(net net.IPNet) (Family, error) {
	switch {
	case net.IP.To4() != nil:
		return FamilyV4, nil
	case net.IP.To16() != nil:
		return FamilyV6, nil
	default:
		return "", fmt.Errorf("unknown family for %s", net)
	}
}

func getStartEndIP(ipNet net.IPNet) (net.IP, net.IP, error) {
	family, err := getFamily(ipNet)

	if err != nil {
		return nil, nil, err
	}

	if family == FamilyV4 {
		startIp := ipNet.IP.Mask(ipNet.Mask)

		invertedMask := make(net.IP, len(startIp.To4()))

		for i := range invertedMask {
			invertedMask[i] = ^ipNet.Mask[i]
		}

		endIp := make(net.IP, len(startIp.To4()))
		startIpTo4 := startIp.To4()

		for i := range endIp {
			endIp[i] = startIpTo4[i] | invertedMask[i]
		}

		return startIp, endIp, nil
	}

	if family == FamilyV6 {
		startIp := ipNet.IP.Mask(ipNet.Mask)

		invertedMask := make(net.IP, len(startIp.To16()))

		for i := range invertedMask {
			invertedMask[i] = ^ipNet.Mask[i]
		}

		endIp := make(net.IP, len(startIp.To16()))
		startIpTo16 := startIp.To16()

		for i := range endIp {
			endIp[i] = startIpTo16[i] | invertedMask[i]
		}

		return startIp, endIp, nil
	}

	return nil, nil, fmt.Errorf("not implemented")
}

// addressFields returns the WinDivert destination and source address field
// names for the given family (ip.* for IPv4, ipv6.* for IPv6).
func addressFields(family Family) (dst, src string) {
	if family == FamilyV6 {
		return "ipv6.DstAddr", "ipv6.SrcAddr"
	}
	return "ip.DstAddr", "ip.SrcAddr"
}

const openGroup string = " and ("
const closeGroup string = ")"

func buildWinDivertFilter(f Filter) (string, error) {
	var sb strings.Builder

	// Start from a protocol-agnostic base so portless protocols (e.g. ICMP) are
	// matched as well. The include/exclude clauses restrict to tcp/udp only where
	// a port is specified; when no port is given they match every protocol.
	sb.WriteString("true")

	if f.Direction != DirectionAll {
		writeDirectionFilter(&sb, f.Direction)
	}

	if len(f.InterfaceIndexes) > 0 {
		writeInterfaceFilter(&sb, f.InterfaceIndexes)
	}

	if len(f.Include) > 0 {
		err := writeIncludeFilter(&sb, f, f.Direction)
		if err != nil {
			return "", err
		}
	}

	if len(f.Exclude) > 0 {
		err := writeExcludeFilter(&sb, f)
		if err != nil {
			return "", err
		}
	}

	return sb.String(), nil
}

func writeDirectionFilter(sb *strings.Builder, direction Direction) {
	sb.WriteString(" and ")
	if direction == DirectionIncoming {
		sb.WriteString("inbound")
	} else {
		sb.WriteString("outbound")
	}
}

func writeInterfaceFilter(sb *strings.Builder, ifIdxs []int) {
	sb.WriteString(openGroup)
	ifIdxStatements := make([]string, len(ifIdxs))
	for i, ifIdx := range ifIdxs {
		ifIdxStatements[i] = fmt.Sprintf("ifIdx == %d", ifIdx)
	}
	sb.WriteString(strings.Join(ifIdxStatements, " or "))
	sb.WriteString(closeGroup)
}

func writeIncludeFilter(sb *strings.Builder, filter Filter, direction Direction) error {
	sb.WriteString(openGroup)
	for i, ran := range filter.Include {
		family, err := getFamily(ran.Net)
		if err != nil {
			return err
		}
		dstAddr, srcAddr := addressFields(family)

		startIp, endIp, err := getStartEndIP(ran.Net)
		if err != nil {
			return err
		}

		if direction != DirectionIncoming {
			sb.WriteString(includeClause(dstAddr, "tcp.DstPort", "udp.DstPort", ran.PortRange, startIp, endIp))
		}

		if direction == DirectionAll {
			sb.WriteString(" or ")
		}

		if direction != DirectionOutgoing {
			sb.WriteString(includeClause(srcAddr, "tcp.SrcPort", "udp.SrcPort", ran.PortRange, startIp, endIp))
		}

		if i < len(filter.Include)-1 {
			sb.WriteString(" or ")
		}
	}
	sb.WriteString(closeGroup)
	return nil
}

func writeExcludeFilter(sb *strings.Builder, filter Filter) error {
	sb.WriteString(openGroup)
	for i, ran := range filter.Exclude {
		family, err := getFamily(ran.Net)
		if err != nil {
			return err
		}
		dstAddr, srcAddr := addressFields(family)

		startIp, endIp, err := getStartEndIP(ran.Net)
		if err != nil {
			return err
		}

		sb.WriteString(excludeClause(dstAddr, "tcp.DstPort", "udp.DstPort", ran.PortRange, startIp, endIp))
		sb.WriteString(" and ")
		sb.WriteString(excludeClause(srcAddr, "tcp.SrcPort", "udp.SrcPort", ran.PortRange, startIp, endIp))

		if i < len(filter.Exclude)-1 {
			sb.WriteString(" and ")
		}
	}

	sb.WriteString(closeGroup)
	return nil
}

// addressMatch renders the `<addrField> ...` fragment matching a single address
// or an inclusive address range.
func addressMatch(addrField string, startIp, endIp net.IP) string {
	if startIp.String() == endIp.String() {
		return fmt.Sprintf("%s == %s", addrField, startIp.String())
	}
	return fmt.Sprintf("%s >= %s and %s <= %s", addrField, startIp.String(), addrField, endIp.String())
}

// includeClause builds a WinDivert sub-expression matching traffic to/from the
// given address range. For the any-port wildcard it matches every protocol
// (including portless ones such as ICMP); otherwise it restricts to the tcp/udp
// packets whose port falls in the range.
func includeClause(addrField, tcpPortField, udpPortField string, portRange akn.PortRange, startIp, endIp net.IP) string {
	addr := addressMatch(addrField, startIp, endIp)
	if portRange == akn.PortRangeAny {
		return fmt.Sprintf("( %s )", addr)
	}

	var portFilter string
	if portRange.From == portRange.To {
		portFilter = fmt.Sprintf("(( %s == %d ) or ( %s == %d ))", tcpPortField, portRange.From, udpPortField, portRange.From)
	} else {
		portFilter = fmt.Sprintf("(( %s >= %d and %s <= %d ) or ( %s >= %d and %s <= %d ))", tcpPortField, portRange.From, tcpPortField, portRange.To, udpPortField, portRange.From, udpPortField, portRange.To)
	}
	return fmt.Sprintf("( %s and %s)", addr, portFilter)
}

// excludeClause builds a WinDivert sub-expression that spares traffic to/from
// the given address range. For the any-port wildcard it excludes every protocol
// on that address; otherwise it only excludes the tcp/udp packets whose port
// falls in the range.
func excludeClause(addrField, tcpPortField, udpPortField string, portRange akn.PortRange, startIp, endIp net.IP) string {
	addr := addressMatch(addrField, startIp, endIp)
	spare := "false"
	if portRange != akn.PortRangeAny {
		// A port-scoped exclude must spare only the tcp/udp packets whose port is
		// in the range. Each port test is guarded with `not tcp`/`not udp` so that
		// portless protocols (e.g. ICMP) stay subject to the attack — a bare port
		// comparison is false for a packet that has no port, which would otherwise
		// spare all ICMP to/from the excluded address.
		if portRange.From == portRange.To {
			spare = fmt.Sprintf("(( not tcp or %s != %d ) and ( not udp or %s != %d ))", tcpPortField, portRange.From, udpPortField, portRange.From)
		} else {
			spare = fmt.Sprintf("(( not tcp or %s < %d or %s > %d ) and ( not udp or %s < %d or %s > %d ))", tcpPortField, portRange.From, tcpPortField, portRange.To, udpPortField, portRange.From, udpPortField, portRange.To)
		}
	}
	return fmt.Sprintf("(( %s )? %s: true)", addr, spare)
}

func buildWinDivertFilterFile(f Filter) (string, error) {
	filterContent, err := buildWinDivertFilter(f)
	if err != nil {
		return "", err
	}

	tempFile, err := os.CreateTemp("", "wdna-filter-*.txt")
	if err != nil {
		return "", err
	}
	defer func(tempFile *os.File) {
		_ = tempFile.Close()
	}(tempFile)

	_, err = tempFile.Write([]byte(filterContent))
	if err != nil {
		return "", err
	}
	return tempFile.Name(), nil
}

func awaitWinDivertServiceStatus(state svc.State, timeout time.Duration) error {
	// wait until the windivert service reports successful startup or an error occurred
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func(m *mgr.Mgr) {
		_ = m.Disconnect()
	}(m)

	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		s, err := m.OpenService("windivert")
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			log.Debug().Msgf("failed opening the windivert service with error (retrying in 500ms): %v", err)
			continue
		}

		log.Info().Msgf("successfully opened the windivert service.")
		// deferred function is only created once
		//goland:noinspection GoDeferInLoop
		defer func(s *mgr.Service) {
			_ = s.Close()
		}(s)

		for time.Now().Before(end) {
			status, err := s.Query()
			if err == nil && status.State == state {
				log.Debug().Int("state", int(status.State)).Msgf("windivert service reached state %d", state)
				return nil
			}
			//goland:noinspection GoDfaErrorMayBeNotNil
			log.Debug().Int("state", int(status.State)).Msgf("windivert service not yet in state %d", state)
			time.Sleep(100 * time.Millisecond)
		}
	}
	return fmt.Errorf("windivert service did not reach state %d in time", state)
}
