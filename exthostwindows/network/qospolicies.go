// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"strings"
)

const qosPolicyPrefix = "STEADYBIT_QOS_"

func logCurrentQoSRules(ctx context.Context, when string) {
	if !log.Trace().Enabled() {
		return
	}
	policies, err := listSteadybitQosPolicies(ctx)
	if err != nil {
		log.Trace().Err(err).Msg("failed to get current QoS rules")
	} else {
		log.Trace().Str("when", when).Strs("policies", policies).Msg("current QoS policies")
	}
}

func listSteadybitQosPolicies(ctx context.Context) ([]string, error) {
	listCommand := "Get-NetQosPolicy | Where-Object { $_.Name -like \"" + qosPolicyPrefix + "*\" }"
	result, err := utils.ExecutePowershellCommand(ctx, []string{listCommand}, utils.PSRun)
	if err != nil {
		return nil, fmt.Errorf("failed to list QoS policies: %w", err)
	}
	var policies []string
	blocks := strings.Split(result, "\r\n\r\n")
	for _, block := range blocks {
		if block != "" {
			policies = append(policies, block)
		}
	}
	return policies, nil
}

func listSteadybitQosPolicyNames(ctx context.Context) ([]string, error) {
	listCommand := "Get-NetQosPolicy | Where-Object { $_.Name -like \"" + qosPolicyPrefix + "*\" } | Select-Object -ExpandProperty Name"
	result, err := utils.ExecutePowershellCommand(ctx, []string{listCommand}, utils.PSRun)
	if err != nil {
		return nil, fmt.Errorf("failed to list QoS policies: %w", err)
	}
	var policies []string
	blocks := strings.Split(result, "\r\n")
	for _, block := range blocks {
		if block != "" {
			policies = append(policies, block)
		}
	}
	return policies, nil
}

func removeSteadybitQosPolicies(ctx context.Context) error {
	policies, err := listSteadybitQosPolicyNames(ctx)
	if err != nil {
		return err
	}
	if len(policies) == 0 {
		return nil
	}

	log.Error().Strs("policies", policies).Msg("Found leftover QoS policies, removing them")
	return removeQoSPolicies(ctx, policies)
}

func removeQoSPolicies(ctx context.Context, policies []string) error {
	var errors []string
	for _, policy := range policies {
		removeCommand := fmt.Sprintf("Remove-NetQosPolicy -Name %s -Confirm:$false", policy)
		if _, err := utils.ExecutePowershellCommand(ctx, []string{removeCommand}, utils.PSRun); err != nil {
			errors = append(errors, err.Error())
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("failed to remove QoS policies: %s", strings.Join(errors, "; "))
	}
	return nil
}
