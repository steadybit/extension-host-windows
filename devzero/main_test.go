// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsBrokenPipe(t *testing.T) {
	assert.True(t, isBrokenPipe(syscall.EPIPE))
	assert.True(t, isBrokenPipe(&fs.PathError{Op: "write", Path: "/dev/stdout", Err: syscall.EPIPE}))
	assert.True(t, isBrokenPipe(fmt.Errorf("wrapped: %w", &fs.PathError{Op: "write", Err: syscall.EPIPE})))

	assert.False(t, isBrokenPipe(syscall.EACCES))
	assert.False(t, isBrokenPipe(&fs.PathError{Op: "write", Err: syscall.EACCES}))
	assert.False(t, isBrokenPipe(errors.New("some error")))
}

func TestUnwrap(t *testing.T) {
	inner := errors.New("inner")
	assert.Equal(t, inner, unwrap(fmt.Errorf("outer: %w", inner)))
	assert.Nil(t, unwrap(inner))
}
