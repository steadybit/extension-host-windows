// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package e2e

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/steadybit/extension-host-windows/exthostwindows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// awaitProcesses waits until exactly count processes whose executable name contains name run, and returns their pids.
func awaitProcesses(t *testing.T, l Environment, name string, count int) []int {
	t.Helper()
	var pids []int
	require.Eventuallyf(t, func() bool {
		pids = l.FindProcessIds(t.Context(), name)
		return len(pids) == count
	}, 20*time.Second, 100*time.Millisecond, "expected %d %s processes, last seen %v", count, name, pids)
	return pids
}

func testStressCpu(t *testing.T, l Environment, e Extension) {
	const process = "steadybit-stress-cpu"
	config := map[string]any{"cpuLoad": 20, "cores": 1, "priority": "Normal", "duration": 60000}

	t.Run("stopping one attack leaves another one running", func(t *testing.T) {
		first, err := e.RunAction(exthostwindows.BaseActionID+".stress-cpu", l.BuildTarget(t.Context()), config, nil)
		require.NoError(t, err)
		defer func() { _ = first.Cancel() }()
		firstPids := awaitProcesses(t, l, process, 1)

		second, err := e.RunAction(exthostwindows.BaseActionID+".stress-cpu", l.BuildTarget(t.Context()), config, nil)
		require.NoError(t, err)
		defer func() { _ = second.Cancel() }()
		pids := awaitProcesses(t, l, process, 2)
		secondPids := slices.DeleteFunc(pids, func(pid int) bool { return slices.Contains(firstPids, pid) })

		require.NoError(t, first.Cancel())
		assert.Equal(t, secondPids, l.FindProcessIds(t.Context(), process), "only the process of the stopped attack must end")

		require.NoError(t, second.Cancel())
		assert.Empty(t, l.FindProcessIds(t.Context(), process))
	})

	t.Run("the attack ends after its duration", func(t *testing.T) {
		short := map[string]any{"cpuLoad": 20, "cores": 1, "priority": "Normal", "duration": 3000}
		action, err := e.RunAction(exthostwindows.BaseActionID+".stress-cpu", l.BuildTarget(t.Context()), short, nil)
		require.NoError(t, err)
		awaitProcesses(t, l, process, 1)

		require.NoError(t, action.Wait())
		assert.Empty(t, l.FindProcessIds(t.Context(), process))
	})
}

func testStressIo(t *testing.T, l Environment, e Extension) {
	// diskspd only works on an existing file.
	file := filepath.Join(t.TempDir(), "steadybit-stress-io.dat")
	f, err := os.Create(file)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(64<<20))
	require.NoError(t, f.Close())

	config := map[string]any{
		"stressLayer":        "File System",
		"stressLayerInput":   file,
		"threadCount":        1,
		"duration":           60000,
		"disableSwHwCaching": false,
	}
	action, err := e.RunAction(exthostwindows.BaseActionID+".stress-io", l.BuildTarget(t.Context()), config, nil)
	require.NoError(t, err)
	defer func() { _ = action.Cancel() }()
	awaitProcesses(t, l, "diskspd", 1)

	require.NoError(t, action.Cancel())
	assert.Empty(t, l.FindProcessIds(t.Context(), "diskspd"))
}

func testFillMem(t *testing.T, l Environment, e Extension) {
	config := map[string]any{"duration": 60000, "mode": "absolute", "size": 100, "unit": "MiB"}
	action, err := e.RunAction(exthostwindows.BaseActionID+".fill_mem", l.BuildTarget(t.Context()), config, nil)
	require.NoError(t, err)
	defer func() { _ = action.Cancel() }()
	require.Eventually(t, func() bool { return len(l.FindProcessIds(t.Context(), "memfill")) > 0 }, 20*time.Second, 100*time.Millisecond)

	require.NoError(t, action.Cancel())
	assert.Empty(t, l.FindProcessIds(t.Context(), "memfill"), "memfill and the workers it starts must end")
}

func testFillDisk(t *testing.T, l Environment, e Extension) {
	for _, method := range []string{"OVER_TIME", "AT_ONCE"} {
		t.Run(method, func(t *testing.T) {
			dir := t.TempDir()
			// Large enough that dd still writes when the test looks for it.
			config := map[string]any{"duration": 60000, "mode": "MB_TO_FILL", "size": 1000, "path": dir, "method": method, "blocksize": 5}
			action, err := e.RunAction(exthostwindows.BaseActionID+".fill_disk", l.BuildTarget(t.Context()), config, nil)
			require.NoError(t, err)
			defer func() { _ = action.Cancel() }()

			if method == "OVER_TIME" {
				awaitProcesses(t, l, "devzero", 1)
				awaitProcesses(t, l, "coreutils", 1)
			}
			require.Eventually(t, func() bool { return len(fillFiles(dir)) == 1 }, 20*time.Second, 100*time.Millisecond, "the fill file must be created")

			require.NoError(t, action.Cancel())
			assert.Empty(t, fillFiles(dir), "the fill file must be removed")
			if method == "OVER_TIME" {
				assert.Empty(t, l.FindProcessIds(t.Context(), "devzero"))
				assert.Empty(t, l.FindProcessIds(t.Context(), "coreutils"))
			}
		})
	}
}

// fillFiles lists dir; a directory that cannot be read counts as empty.
func fillFiles(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
