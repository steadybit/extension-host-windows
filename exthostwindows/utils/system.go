// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package utils

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// SystemExecutable returns the absolute path of a tool in the Windows system directory, so that no executable with
// the same name found earlier in PATH is started instead.
func SystemExecutable(name string) string {
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		return name
	}
	return filepath.Join(dir, name)
}

// IsWindowsHomeEdition reports whether Windows runs as a Home edition, which does not apply policy-based QoS.
func IsWindowsHomeEdition() bool {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer func() { _ = key.Close() }()

	edition, _, err := key.GetStringValue("EditionID")
	if err != nil {
		return false
	}
	return isHomeEdition(edition)
}

// isHomeEdition matches the EditionID of Home editions: Core, CoreN, CoreSingleLanguage and CoreCountrySpecific.
// Server Core installations report ServerStandard or ServerDatacenter.
func isHomeEdition(editionID string) bool {
	return strings.HasPrefix(editionID, "Core")
}
