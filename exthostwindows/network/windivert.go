// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"fmt"
	"net"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/network"
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

	if family == network.FamilyV6 {
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

func setCorrectReplacements(replacements *map[string]string, family Family) {
	if family == FamilyV4 {
		(*replacements)["ipDstAddr"] = "ip.DstAddr"
		(*replacements)["ipSrcAddr"] = "ip.SrcAddr"
	} else {
		(*replacements)["ipDstAddr"] = "ipv6.DstAddr"
		(*replacements)["ipSrcAddr"] = "ipv6.SrcAddr"
	}
}

const openGroup string = " and ("
const closeGroup string = ")"

func buildWinDivertFilter(f Filter) (string, error) {
	var sb strings.Builder

	sb.WriteString("(tcp or udp)")

	if f.Direction != DirectionAll {
		writeDirectionFilter(&sb, f.Direction)
	}

	if len(f.InterfaceIndexes) > 0 {
		writeInterfaceFilter(&sb, f.InterfaceIndexes)
	}

	if len(f.Filter.Include) > 0 {
		err := writeIncludeFilter(&sb, f.Filter, f.Direction)
		if err != nil {
			return "", err
		}
	}

	if len(f.Filter.Exclude) > 0 {
		err := writeExcludeFilter(&sb, f.Filter)
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

func writeIncludeFilter(sb *strings.Builder, filter network.Filter, direction Direction) error {
	replaceMap := map[string]string{
		"tcpDstPort": "tcp.DstPort",
		"udpDstPort": "udp.DstPort",
		"tcpSrcPort": "tcp.SrcPort",
		"udpSrcPort": "udp.SrcPort",
	}

	sb.WriteString(openGroup)
	for i, ran := range filter.Include {
		family, err := getFamily(ran.Net)
		if err != nil {
			return err
		}

		setCorrectReplacements(&replaceMap, family)
		if direction != DirectionIncoming {
			var portFilter string

			if ran.PortRange.From == ran.PortRange.To {
				portFilter = fmt.Sprintf("(( {{.tcpDstPort}} == %d ) or ( {{.udpDstPort}} == %d ))",
					ran.PortRange.From, ran.PortRange.From)
			} else {
				portFilter = fmt.Sprintf("(( {{.tcpDstPort}} >= %d and {{.tcpDstPort}} <= %d ) or ( {{.udpDstPort}} >= %d and {{.udpDstPort}} <= %d ))",
					ran.PortRange.From, ran.PortRange.To, ran.PortRange.From, ran.PortRange.To)
			}

			startIp, endIp, err := getStartEndIP(ran.Net)
			if err != nil {
				return err
			}

			var config string

			if startIp.String() == endIp.String() {
				config = fmt.Sprintf("( {{.ipDstAddr}} == %s and %s)",
					startIp.String(), portFilter)
			} else {
				config = fmt.Sprintf("( {{.ipDstAddr}} >= %s and {{.ipDstAddr}} <= %s and %s)",
					startIp.String(), endIp.String(), portFilter)
			}

			tmpl, err := template.New("filter").Parse(config)
			if err != nil {
				return err
			}

			err = tmpl.Execute(sb, replaceMap)
			if err != nil {
				return err
			}
		}

		if direction == DirectionAll {
			sb.WriteString(" or ")
		}

		if direction != DirectionOutgoing {
			var portFilter string

			if ran.PortRange.From == ran.PortRange.To {
				portFilter = fmt.Sprintf("(( {{.tcpSrcPort}} == %d ) or ( {{.udpSrcPort}} == %d ))",
					ran.PortRange.From, ran.PortRange.From)
			} else {
				portFilter = fmt.Sprintf("(( {{.tcpSrcPort}} >= %d and {{.tcpSrcPort}} <= %d ) or ( {{.udpSrcPort}} >= %d and {{.udpSrcPort}} <= %d ))",
					ran.PortRange.From, ran.PortRange.To, ran.PortRange.From, ran.PortRange.To)
			}

			startIp, endIp, err := getStartEndIP(ran.Net)
			if err != nil {
				return err
			}

			var config string

			if startIp.String() == endIp.String() {
				config = fmt.Sprintf("( {{.ipSrcAddr}} == %s and %s)",
					startIp.String(), portFilter)
			} else {
				config = fmt.Sprintf("( {{.ipSrcAddr}} >= %s and {{.ipSrcAddr}} <= %s and %s)",
					startIp.String(), endIp.String(), portFilter)
			}

			tmpl, err := template.New("filter").Parse(config)
			if err != nil {
				return err
			}

			err = tmpl.Execute(sb, replaceMap)
			if err != nil {
				return err
			}
		}

		if i < len(filter.Include)-1 {
			sb.WriteString(" or ")
		}
	}
	sb.WriteString(closeGroup)
	return nil
}

func writeExcludeFilter(sb *strings.Builder, filter network.Filter) error {
	replaceMap := map[string]string{
		"tcpDstPort": "tcp.DstPort",
		"udpDstPort": "udp.DstPort",
		"tcpSrcPort": "tcp.SrcPort",
		"udpSrcPort": "udp.SrcPort",
	}

	sb.WriteString(openGroup)
	for i, ran := range filter.Exclude {
		family, err := getFamily(ran.Net)
		if err != nil {
			return err
		}

		setCorrectReplacements(&replaceMap, family)

		var portFilter string

		if ran.PortRange.From == ran.PortRange.To {
			portFilter = fmt.Sprintf("(( {{.tcpDstPort}} != %d ) or ( {{.udpDstPort}} != %d ))",
				ran.PortRange.From, ran.PortRange.To)
		} else {
			portFilter = fmt.Sprintf("(( {{.tcpDstPort}} < %d or {{.tcpDstPort}} > %d ) or ( {{.udpDstPort}} < %d or {{.udpDstPort}} > %d ))",
				ran.PortRange.From, ran.PortRange.To, ran.PortRange.From, ran.PortRange.To)
		}

		startIp, endIp, err := getStartEndIP(ran.Net)
		if err != nil {
			return err
		}

		var config string

		if startIp.String() == endIp.String() {
			config = fmt.Sprintf("(( {{.ipDstAddr}} == %s )? %s: true)",
				startIp.String(), portFilter)
		} else {
			config = fmt.Sprintf("(( {{.ipDstAddr}} >= %s and {{.ipDstAddr}} <= %s )? %s: true)",
				startIp.String(), endIp.String(), portFilter)
		}

		tmpl, err := template.New("filter").Parse(config)
		if err != nil {
			return err
		}

		err = tmpl.Execute(sb, replaceMap)
		if err != nil {
			return err
		}

		sb.WriteString(" and ")

		if ran.PortRange.From == ran.PortRange.To {
			portFilter = fmt.Sprintf("(( {{.tcpSrcPort}} != %d ) or ( {{.udpSrcPort}} != %d ))",
				ran.PortRange.From, ran.PortRange.To)
		} else {
			portFilter = fmt.Sprintf("(( {{.tcpSrcPort}} < %d or {{.tcpSrcPort}} > %d ) or ( {{.udpSrcPort}} < %d or {{.udpSrcPort}} > %d ))",
				ran.PortRange.From, ran.PortRange.To, ran.PortRange.From, ran.PortRange.To)
		}

		startIp, endIp, err = getStartEndIP(ran.Net)
		if err != nil {
			return err
		}

		if startIp.String() == endIp.String() {
			config = fmt.Sprintf("(( {{.ipSrcAddr}} == %s )? %s: true)",
				startIp.String(), portFilter)
		} else {
			config = fmt.Sprintf("(( {{.ipSrcAddr}} >= %s and {{.ipSrcAddr}} <= %s )? %s: true)",
				startIp.String(), endIp.String(), portFilter)
		}

		tmpl, err = template.New("filter").Parse(config)
		if err != nil {
			return err
		}

		err = tmpl.Execute(sb, replaceMap)
		if err != nil {
			return err
		}

		if i < len(filter.Exclude)-1 {
			sb.WriteString(" and ")
		}
	}

	sb.WriteString(closeGroup)
	return nil
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
