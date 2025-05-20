// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_TrimShellOutput(t *testing.T) {
	command, err := ExecutePowershellCommand(t.Context(), []string{"echo 'hello world'"}, PSRun)
	require.NoError(t, err)
	require.Equal(t, "hello world", command)
}

func Test_sanitizePowerShellArg(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Plain string without special characters",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "String with single quotes",
			input:    "It's a test",
			expected: "It''s a test",
		},
		{
			name:     "String with double quotes",
			input:    "Say \"hello\"",
			expected: "Say `\"hello`\"",
		},
		{
			name:     "String with backticks",
			input:    "Escape`character",
			expected: "Escape``character",
		},
		{
			name:     "String with dollar sign",
			input:    "$variable",
			expected: "`$variable",
		},
		{
			name:     "String with parentheses",
			input:    "function(param)",
			expected: "function`(param`)",
		},
		{
			name:     "String with braces",
			input:    "scriptblock{code}",
			expected: "scriptblock`{code`}",
		},
		{
			name:     "String with semicolon",
			input:    "command1;command2",
			expected: "command1`;command2",
		},
		{
			name:     "String with pipe",
			input:    "cmd1|cmd2",
			expected: "cmd1`|cmd2",
		},
		{
			name:     "String with ampersand",
			input:    "cmd1&cmd2",
			expected: "cmd1`&cmd2",
		},
		{
			name:     "String with redirection operators",
			input:    "cmd > file.txt",
			expected: "cmd `> file.txt",
		},
		{
			name:     "String with input redirection",
			input:    "cmd < file.txt",
			expected: "cmd `< file.txt",
		},
		{
			name:     "String with nested escapes",
			input:    "`$already_escaped",
			expected: "```$already_escaped",
		},
		{
			name:     "Complex string with multiple special characters",
			input:    "function($var) { if ($var -eq 'test') { Write-Host \"Hello, World!\"; exit 0 } }",
			expected: "function`(`$var`) `{ if `(`$var -eq ''test''`) `{ Write-Host `\"Hello, World!`\"`; exit 0 `} `}",
		},
		{
			name:     "Command injection attempt",
			input:    "param'; Start-Process calc.exe; #",
			expected: "param''`; Start-Process calc.exe`; #",
		},
		{
			name:     "Complex injection attempt",
			input:    "param\"; $(Invoke-Expression \"calc.exe\"); $(",
			expected: "param`\"`; `$`(Invoke-Expression `\"calc.exe`\"`)`; `$`(",
		},
		{
			name:     "Multiple backticks",
			input:    "Test```with```backticks",
			expected: "Test``````with``````backticks",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizePowershellArg(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizePowershellArg() = %v, want %v", got, tt.expected)
			}
		})
	}
}
