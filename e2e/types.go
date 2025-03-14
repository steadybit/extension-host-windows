// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package e2e

import (
	"github.com/go-resty/resty/v2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	aclient "github.com/steadybit/action-kit/go/action_kit_test/client"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
)

type Environment interface {
	BuildTarget() *action_kit_api.Target
	FindProcessIds(command string) []int
	StartProcess(command string, awaitFn func(string) bool, parameters ...string) (func(), error)
}

type ExtensionFactory interface {
	Create() error
	Start(environment *Environment) (Extension, error)
	Stop(environment *Environment, extension Extension) error
}

type Extension interface {
	Client() *resty.Client
	DiscoverTargets(discoveryId string) ([]discovery_kit_api.Target, error)
	DiscoverEnrichmentData(discoveryId string) ([]discovery_kit_api.EnrichmentData, error)
	RunAction(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext) (aclient.ActionExecution, error)
	RunActionWithFiles(actionId string, target *action_kit_api.Target, config interface{}, executionContext *action_kit_api.ExecutionContext, files []aclient.File) (aclient.ActionExecution, error)
}
