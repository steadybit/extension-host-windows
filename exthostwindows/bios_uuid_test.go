// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_cachedBiosUuid(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{name: "normalized", output: "4203F2B8-1C6A-9E0D-2F41-7A3C55B90E12\r\n", want: "4203f2b8-1c6a-9e0d-2f41-7a3c55b90e12"},
		{name: "all zeros", output: "00000000-0000-0000-0000-000000000000"},
		{name: "all F", output: "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			c := cachedBiosUuid{query: func(context.Context) (string, error) {
				calls++
				return tt.output, nil
			}}

			assert.Equal(t, tt.want, c.get(context.Background()))
			assert.Equal(t, tt.want, c.get(context.Background()))
			assert.Equal(t, 1, calls)
		})
	}
}

func Test_cachedBiosUuid_retries_until_query_succeeds(t *testing.T) {
	results := []struct {
		output string
		err    error
	}{
		{err: errors.New("WMI not ready")},
		{output: "WARNING: something unexpected\r\n"},
		{output: ""},
		{output: "4203F2B8-1C6A-9E0D-2F41-7A3C55B90E12"},
	}
	calls := 0
	c := cachedBiosUuid{query: func(context.Context) (string, error) {
		r := results[calls]
		calls++
		return r.output, r.err
	}}

	for range 3 {
		assert.Empty(t, c.get(context.Background()))
	}
	assert.Equal(t, "4203f2b8-1c6a-9e0d-2f41-7a3c55b90e12", c.get(context.Background()))
	assert.Equal(t, "4203f2b8-1c6a-9e0d-2f41-7a3c55b90e12", c.get(context.Background()))
	assert.Equal(t, 4, calls)
}
