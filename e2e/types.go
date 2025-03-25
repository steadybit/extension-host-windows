// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package e2e

import (
	"context"
	"github.com/go-resty/resty/v2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	aclient "github.com/steadybit/action-kit/go/action_kit_test/client"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
)

type Environment interface {
	BuildTarget(ctx context.Context) *action_kit_api.Target
	FindProcessIds(ctx context.Context, command string) []int
	ExecuteProcess(ctx context.Context, command string, parameters ...string) (string, error)
	StartAndAwaitProcess(ctx context.Context, command string, awaitFn func(string) bool, parameters ...string) (func(), error)
	StopProcess(ctx context.Context, commandOrPid string) error
}

type ExtensionFactory interface {
	Create(ctx context.Context) error
	Start(ctx context.Context, environment *Environment) (Extension, error)
	Stop(ctx context.Context, environment *Environment, extension Extension) error
}

type Extension interface {
	Client() *resty.Client
	DiscoverTargets(discoveryId string) ([]discovery_kit_api.Target, error)
	DiscoverEnrichmentData(discoveryId string) ([]discovery_kit_api.EnrichmentData, error)
	RunAction(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext) (aclient.ActionExecution, error)
	RunActionWithFiles(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext, files []aclient.File) (aclient.ActionExecution, error)
	PollForTarget(ctx context.Context, targetId string, predicate func(target discovery_kit_api.Target) bool) (discovery_kit_api.Target, error)
	PollForEnrichmentData(ctx context.Context, targetId string, predicate func(target discovery_kit_api.EnrichmentData) bool) (discovery_kit_api.EnrichmentData, error)
}
