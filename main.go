// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build windows

package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-host/config"
	"github.com/steadybit/extension-host/exthost"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthealth"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/steadybit/extension-kit/extruntime"
	"github.com/steadybit/extension-kit/extsignals"
	_ "go.uber.org/automaxprocs" // Importing automaxprocs automatically adjusts GOMAXPROCS.
)

func main() {
	extlogging.InitZeroLog()

	extbuild.PrintBuildInformation()
	extruntime.LogRuntimeInformation(zerolog.InfoLevel)

	config.ParseConfiguration()
	config.ValidateConfiguration()

	exthealth.SetReady(false)
	exthealth.StartProbes(int(config.Config.HealthPort))

	exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(getExtensionList))
	action_kit_sdk.RegisterAction(exthost.NewShutdownAction())
	action_kit_sdk.RegisterAction(exthost.NewStopProcessAction())
	action_kit_sdk.RegisterAction(exthost.NewNetworkBlockDnsContainerAction())
	action_kit_sdk.RegisterAction(exthost.NewNetworkBlackholeContainerAction())
	action_kit_sdk.RegisterAction(exthost.NewNetworkLimitBandwidthContainerAction())
	action_kit_sdk.RegisterAction(exthost.NewNetworkDelayContainerAction())
	action_kit_sdk.RegisterAction(exthost.NewNetworkCorruptPackagesContainerAction())
	action_kit_sdk.RegisterAction(exthost.NewNetworkPackageLossContainerAction())

	log.Info().Interface("cfg", runc.ConfigFromEnvironment())

	discovery_kit_sdk.Register(exthost.NewHostDiscovery())

	extsignals.ActivateSignalHandlers()

	action_kit_sdk.RegisterCoverageEndpoints()

	exthealth.SetReady(true)

	exthttp.Listen(exthttp.ListenOpts{
		Port: int(config.Config.Port),
	})
}

type ExtensionListResponse struct {
	action_kit_api.ActionList       `json:",inline"`
	discovery_kit_api.DiscoveryList `json:",inline"`
}

func getExtensionList() ExtensionListResponse {
	return ExtensionListResponse{
		ActionList:    action_kit_sdk.GetActionList(),
		DiscoveryList: discovery_kit_sdk.GetDiscoveryList(),
	}
}
