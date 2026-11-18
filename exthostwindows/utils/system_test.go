// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isHomeEdition(t *testing.T) {
	for edition, want := range map[string]bool{
		"Core":                 true,
		"CoreN":                true,
		"CoreSingleLanguage":   true,
		"CoreCountrySpecific":  true,
		"Professional":         false,
		"Enterprise":           false,
		"ServerStandard":       false,
		"ServerDatacenter":     false,
		"ServerDatacenterEval": false,
	} {
		assert.Equal(t, want, isHomeEdition(edition), edition)
	}
}
