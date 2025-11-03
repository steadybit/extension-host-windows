// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
)

const baseLocation = "Software\\Steadybit GmbH\\Extensions"

type ExtensionRegistry struct {
	name  string
	port  uint16
	types []string
}

func NewExtensionRegistry(name string, port uint16, types []string) *ExtensionRegistry {
	return &ExtensionRegistry{
		name:  name,
		port:  port,
		types: types,
	}
}

func (r *ExtensionRegistry) SetupLocalDiscovery() error {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, r.extensionRegistryKeyPath(), registry.WRITE)
	if err != nil {
		return fmt.Errorf("unable to create/open extensions registry key: %w", err)
	}

	defer func(key registry.Key) {
		_ = key.Close()
	}(key)

	err = key.SetStringValue("location", fmt.Sprintf("http://localhost:%d", r.port))
	if err != nil {
		return fmt.Errorf("unable to set location value in the registry: %w", err)
	}

	err = key.SetStringsValue("types", r.types)
	if err != nil {
		return fmt.Errorf("unable to set type value in the registry: %w", err)
	}

	return nil
}

func (r *ExtensionRegistry) RemoveLocalDiscovery() error {
	err := registry.DeleteKey(registry.LOCAL_MACHINE, r.extensionRegistryKeyPath())
	if err != nil {
		return fmt.Errorf("unable to delete extensions registry key: %w", err)
	}
	return nil
}

func (r *ExtensionRegistry) extensionRegistryKeyPath() string {
	return fmt.Sprintf("%s\\%s", baseLocation, r.name)
}

func (r *ExtensionRegistry) Port(port uint16) {
	r.port = port
}
