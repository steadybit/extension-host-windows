// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_normalizeBiosUuid(t *testing.T) {
	assert.Equal(t, "4203f2b8-1c6a-9e0d-2f41-7a3c55b90e12", normalizeBiosUuid("4203F2B8-1C6A-9E0D-2F41-7A3C55B90E12\r\n"))
	assert.Empty(t, normalizeBiosUuid("00000000-0000-0000-0000-000000000000"))
	assert.Empty(t, normalizeBiosUuid("FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF"))
	assert.Empty(t, normalizeBiosUuid(""))
}

func Test_cachedBiosUuid_retries_until_query_succeeds(t *testing.T) {
	calls := 0
	c := cachedBiosUuid{query: func(context.Context) (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("WMI not ready")
		}
		return "4203F2B8-1C6A-9E0D-2F41-7A3C55B90E12", nil
	}}

	assert.Empty(t, c.get(context.Background()))
	assert.Equal(t, "4203f2b8-1c6a-9e0d-2f41-7a3c55b90e12", c.get(context.Background()))
	assert.Equal(t, "4203f2b8-1c6a-9e0d-2f41-7a3c55b90e12", c.get(context.Background()))
	assert.Equal(t, 2, calls)
}
