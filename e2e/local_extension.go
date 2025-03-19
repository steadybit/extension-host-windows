// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package e2e

import (
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	aclient "github.com/steadybit/action-kit/go/action_kit_test/client"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	dclient "github.com/steadybit/discovery-kit/go/discovery_kit_test/client"
	"time"
)

type LocalExtension struct {
	client *resty.Client
}

func NewLocalExtension(port int) *LocalExtension {
	return &LocalExtension{
		client: resty.New().SetBaseURL(fmt.Sprintf("http://127.0.0.1:%d", port)),
	}
}

func (e *LocalExtension) Client() *resty.Client {
	return e.client
}

func (e *LocalExtension) DiscoverTargets(discoveryId string) ([]discovery_kit_api.Target, error) {
	return dclient.NewDiscoveryClient("/", e.client).DiscoverTargets(discoveryId)
}

func (e *LocalExtension) DiscoverEnrichmentData(discoveryId string) ([]discovery_kit_api.EnrichmentData, error) {
	return dclient.NewDiscoveryClient("/", e.client).DiscoverEnrichmentData(discoveryId)
}

func (e *LocalExtension) RunAction(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext) (aclient.ActionExecution, error) {
	return aclient.NewActionClient("/", e.client).RunAction(actionId, target, config, executionContext)
}

func (e *LocalExtension) RunActionWithFiles(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext, files []aclient.File) (aclient.ActionExecution, error) {
	return aclient.NewActionClient("/", e.client).RunActionWithFiles(actionId, target, config, executionContext, files)
}

func (e *LocalExtension) PollForTarget(ctx context.Context, targetId string, predicate func(target discovery_kit_api.Target) bool) (discovery_kit_api.Target, error) {
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return discovery_kit_api.Target{}, fmt.Errorf("timed out waiting for target. last error: %w", lastErr)
		case <-time.After(200 * time.Millisecond):
			result, err := e.DiscoverTargets(targetId)
			if err != nil {
				lastErr = err
				continue
			}
			for _, target := range result {
				if predicate(target) {
					return target, nil
				}
			}
		}
	}
}

func (e *LocalExtension) PollForEnrichmentData(ctx context.Context, targetId string, predicate func(target discovery_kit_api.EnrichmentData) bool) (discovery_kit_api.EnrichmentData, error) {
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return discovery_kit_api.EnrichmentData{}, fmt.Errorf("timed out waiting for target. last error: %w", lastErr)
		case <-time.After(200 * time.Millisecond):
			result, err := e.DiscoverEnrichmentData(targetId)
			if err != nil {
				lastErr = err
				continue
			}
			for _, enrichmentData := range result {
				if predicate(enrichmentData) {
					return enrichmentData, nil
				}
			}
		}
	}
}
