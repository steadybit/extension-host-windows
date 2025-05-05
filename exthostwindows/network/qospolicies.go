// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"context"
	"errors"
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
	return listQosPolicies(ctx, "Get-NetQosPolicy | Where-Object { $_.Name -like \""+qosPolicyPrefix+"*\" }", "\r\n\r\n")
}

func listSteadybitQosPolicyNames(ctx context.Context) ([]string, error) {
	return listQosPolicies(ctx, "Get-NetQosPolicy | Where-Object { $_.Name -like \""+qosPolicyPrefix+"*\" } | Select-Object -ExpandProperty Name", "\r\n")
}

func listQosPolicies(ctx context.Context, command string, separator string) ([]string, error) {
	result, err := utils.ExecutePowershellCommand(ctx, []string{command}, utils.PSRun)

	log.Info().Msgf("result: %s", result)

	if err != nil {
		return nil, fmt.Errorf("failed to list QoS policies: %w", err)
	}
	var policies []string
	blocks := strings.Split(result, separator)
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
	var errs error
	for _, policy := range policies {
		removeCommand := fmt.Sprintf("Remove-NetQosPolicy -Name %s -Confirm:`$false", policy)
		if _, err := utils.ExecutePowershellCommand(ctx, utils.BuildSystemCommandFor(removeCommand), utils.PSRun); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	if errs != nil {
		return fmt.Errorf("failed to remove QoS policies: %w", errs)
	}
	return nil
}
