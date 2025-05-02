// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"fmt"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"net"
	"regexp"
	"strconv"
	"strings"
)

type LimitBandwidthOpts struct {
	Bandwidth    string
	IncludeCidrs []net.IPNet
	Port         int
}

func (o *LimitBandwidthOpts) WinDivertCommands(_ Mode) ([]string, error) {
	return nil, nil
}

func (o *LimitBandwidthOpts) QoSCommands(mode Mode) ([]string, error) {
	bandwidth, err := o.parseBandwidth()
	if err != nil {
		return nil, err
	}

	var cmds []string
	for i, includeCidr := range o.IncludeCidrs {
		if mode == ModeAdd {
			additionalParameters := ""
			if o.Port != 0 {
				additionalParameters = fmt.Sprintf("%s -IPDstPortMatchCondition %d", additionalParameters, o.Port)
			}
			netQosPolicyCommand := fmt.Sprintf("New-NetQosPolicy -Name %s%s_%d -Precedence 255 -Confirm:$false -ThrottleRateActionBitsPerSecond %s -IPDstPrefixMatchCondition '%s' %s",
				qosPolicyPrefix, bandwidth, i, bandwidth, includeCidr.String(), additionalParameters)
			cmds = append(cmds, netQosPolicyCommand)
		} else {
			netQosPolicyCommand := fmt.Sprintf("Remove-NetQosPolicy -Name %s%s_%d -Confirm:$false", qosPolicyPrefix, bandwidth, i)
			cmds = append(cmds, netQosPolicyCommand)
		}
	}
	return cmds, nil
}

func (o *LimitBandwidthOpts) parseBandwidth() (string, error) {
	expression, err := regexp.Compile("^[0-7]$")
	if err != nil {
		return "", err
	}
	if expression.MatchString(o.Bandwidth) {
		return "", fmt.Errorf("windows qos policy does not support rate settings below 8bit/s. (%s)", o.Bandwidth)
	}
	bandwidth := utils.SanitizePowershellArg(o.Bandwidth)
	return bandwidth, nil
}

func (o *LimitBandwidthOpts) String() string {
	var sb strings.Builder
	sb.WriteString("limit bandwidth to ")
	sb.WriteString(o.Bandwidth)
	if len(o.IncludeCidrs) > 0 {
		sb.WriteString(" for:\n")
		for _, includeCidr := range o.IncludeCidrs {
			sb.WriteString(" ")
			sb.WriteString(includeCidr.String())
			if o.Port != 0 {
				sb.WriteString(":")
				sb.WriteString(strconv.Itoa(o.Port))
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
