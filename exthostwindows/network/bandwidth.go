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
	Bandwidth   string
	IncludeCidr *net.IPNet
	Port        int
}

func (o *LimitBandwidthOpts) WinDivertCommands(_ Mode) ([]string, error) {
	return nil, nil
}

func (o *LimitBandwidthOpts) QoSCommands(mode Mode) ([]string, error) {
	var cmds []string

	expression, err := regexp.Compile("^[0-7]$")
	if err != nil {
		return nil, err
	}
	if expression.MatchString(o.Bandwidth) {
		return nil, fmt.Errorf("windows qos policy does not support rate settings below 8bit/s. (%s)", o.Bandwidth)
	}
	bandwidth := utils.SanitizePowershellArg(o.Bandwidth)

	if mode == ModeAdd {
		additionalParameters := ""
		if o.IncludeCidr != nil {
			additionalParameters = fmt.Sprintf("%s -IPDstPrefixMatchCondition '%s'", additionalParameters, o.IncludeCidr.String())
		}
		if o.Port != 0 {
			additionalParameters = fmt.Sprintf("%s -IPDstPortMatchCondition %d", additionalParameters, o.Port)
		}
		cmds = append(cmds, fmt.Sprintf("New-NetQosPolicy -Name STEADYBIT_QOS_%s -Precedence 255 -PolicyStore ActiveStore -Confirm:`$false -ThrottleRateActionBitsPerSecond %s %s", bandwidth, bandwidth, additionalParameters))
	} else {
		cmds = append(cmds, fmt.Sprintf("Remove-NetQosPolicy -Name STEADYBIT_QOS_%s -PolicyStore ActiveStore -Confirm:`$false", bandwidth))
	}

	return cmds, nil
}

func (o *LimitBandwidthOpts) String() string {
	var sb strings.Builder
	sb.WriteString("limit bandwidth to ")
	sb.WriteString(o.Bandwidth)
	if o.IncludeCidr != nil || o.Port != 0 {
		sb.WriteString(" for ")
		if o.IncludeCidr != nil {
			sb.WriteString(o.IncludeCidr.String())
		}
		if o.Port != 0 {
			sb.WriteString(":")
			sb.WriteString(strconv.Itoa(o.Port))
		}
	}
	return sb.String()
}
