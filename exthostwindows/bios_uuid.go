// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
)

const hostBiosUuidAttribute = "host.bios.uuid"

// biosUuid starts a PowerShell process, so a successful result is kept for the lifetime of the extension.
var biosUuid = cachedBiosUuid{query: queryBiosUuid}

type cachedBiosUuid struct {
	mu    sync.Mutex
	query func(ctx context.Context) (string, error)
	value string
	done  bool
}

func (c *cachedBiosUuid) get(ctx context.Context) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done {
		return c.value
	}
	value, err := c.query(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to retrieve BIOS UUID")
		return ""
	}
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		log.Warn().Str("output", value).Msg("unexpected BIOS UUID output")
		return ""
	}
	c.value = withoutPlaceholder(id)
	c.done = true
	return c.value
}

func queryBiosUuid(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return utils.ExecutePowershellCommand(ctx, []string{"(Get-CimInstance -ClassName Win32_ComputerSystemProduct).UUID"})
}

// withoutPlaceholder drops the placeholders firmware reports when no UUID is set (SMBIOS spec 7.2.1).
func withoutPlaceholder(id uuid.UUID) string {
	if id == uuid.Nil || id == uuid.Max {
		return ""
	}
	return id.String()
}
