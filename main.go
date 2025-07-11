// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build windows

package main

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-host-windows/config"
	"github.com/steadybit/extension-host-windows/exthostwindows"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthealth"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/steadybit/extension-kit/extruntime"
	"github.com/steadybit/extension-kit/extsignals"
	_ "go.uber.org/automaxprocs" // Importing automaxprocs automatically adjusts GOMAXPROCS.
	"golang.org/x/sys/windows/registry"
)

func main() {
	extlogging.InitZeroLog()

	// Register a QoS policy cleanup routine as additional safeguard.
	stopQosCleanup := exthostwindows.RegisterQosPolicyCleanup()

	// Register Windows Service early during startup to log messages as Windows application events
	exthostwindows.ActivateWindowsServiceHandler(func() {
		stopQosCleanup()
		exthttp.StopListen()
	})

	extbuild.PrintBuildInformation()
	extruntime.LogRuntimeInformation(zerolog.InfoLevel)

	config.ParseConfiguration()
	config.ValidateConfiguration()

	exthealth.SetReady(false)
	exthealth.StartProbes(int(config.Config.HealthPort))

	exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(getExtensionList))
	action_kit_sdk.RegisterAction(exthostwindows.NewShutdownAction())
	action_kit_sdk.RegisterAction(exthostwindows.NewStopProcessAction())
	action_kit_sdk.RegisterAction(exthostwindows.NewNetworkBlockDnsContainerAction())
	action_kit_sdk.RegisterAction(exthostwindows.NewNetworkBlackholeContainerAction())
	action_kit_sdk.RegisterAction(exthostwindows.NewNetworkLimitBandwidthContainerAction())
	action_kit_sdk.RegisterAction(exthostwindows.NewNetworkDelayContainerAction())
	action_kit_sdk.RegisterAction(exthostwindows.NewNetworkCorruptPackagesContainerAction())
	action_kit_sdk.RegisterAction(exthostwindows.NewNetworkPackageLossContainerAction())
	action_kit_sdk.RegisterAction(exthostwindows.NewTimetravelAction())
	action_kit_sdk.RegisterAction(exthostwindows.NewStressCpuAction())
	action_kit_sdk.RegisterAction(exthostwindows.NewStressIoAction())
	action_kit_sdk.RegisterAction(exthostwindows.NewFillMemAction())
	action_kit_sdk.RegisterAction(exthostwindows.NewFillDiskAction())

	discovery_kit_sdk.Register(exthostwindows.NewHostDiscovery())

	extsignals.ActivateSignalHandlers()

	action_kit_sdk.RegisterCoverageEndpoints()

	setupLocalDiscovery(config.Config.Port)

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

func setupLocalDiscovery(port uint16) {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `Software\Steadybit GmbH\Extensions\Extension Host Windows`, registry.WRITE)

	if err != nil {
		log.Error().Err(err).Msg("unable to create/open extensions registry key, automatic discovery in the local network will not work.")
	}

	defer key.Close()

	err = key.SetStringValue("location", fmt.Sprintf("http://localhost:%d", port))

	if err != nil {
		log.Error().Err(err).Msg("unable to set location value in the registry, automatic discovery in the local network will not work.")
	}

	err = key.SetStringsValue("types", []string{"ACTION", "DISCOVERY"})

	if err != nil {
		log.Error().Err(err).Msg("unable to set type value in the registry, automatic discovery in the local network will not work.")
	}
}
