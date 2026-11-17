// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package utils

import (
	"path/filepath"

	"golang.org/x/sys/windows"
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
