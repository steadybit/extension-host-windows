// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package exthostwindows

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFillDiskOpts_Args(t *testing.T) {
	tests := []struct {
		name          string
		opts          FillDiskOpts
		want          []string
		wantBlockSize uint
	}{
		{
			name: "at once",
			opts: FillDiskOpts{Method: AtOnce, FilePath: `C:\temp\disk-fill`, ByteSize: 200_000_000, BlockSize: 5},
			want: []string{"file", "createNew", `C:\temp\disk-fill`, "200000000"},
			// block size is not relevant for fsutil
			wantBlockSize: 5,
		},
		{
			name:          "over time",
			opts:          FillDiskOpts{Method: OverTime, FilePath: `C:\temp\disk-fill`, ByteSize: 200_000_000, BlockSize: 5},
			want:          []string{"dd", `of=C:\temp\disk-fill`, "iflag=fullblock", "bs=5M", "count=40"},
			wantBlockSize: 5,
		},
		{
			name:          "over time with block size larger than allocation",
			opts:          FillDiskOpts{Method: OverTime, FilePath: `C:\temp\disk-fill`, ByteSize: 3_000_000, BlockSize: 5},
			want:          []string{"dd", `of=C:\temp\disk-fill`, "iflag=fullblock", "bs=3M", "count=1"},
			wantBlockSize: 3,
		},
		{
			name: "unknown method",
			opts: FillDiskOpts{Method: "UNKNOWN", FilePath: `C:\temp\disk-fill`, ByteSize: 3_000_000, BlockSize: 5},
			want: []string{},
			// block size is untouched
			wantBlockSize: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.opts.Args())
			assert.Equal(t, tt.wantBlockSize, tt.opts.BlockSize)
		})
	}
}

func TestBytesToMegabytes(t *testing.T) {
	assert.Equal(t, uint64(0), BytesToMegabytes(999_999))
	assert.Equal(t, uint64(1), BytesToMegabytes(1_000_000))
	assert.Equal(t, uint64(1024), BytesToMegabytes(1_024_000_000))
}

func TestIoStressOpts_Args(t *testing.T) {
	tests := []struct {
		name string
		opts IoStressOpts
		want []string
	}{
		{
			name: "file system",
			opts: IoStressOpts{StressLayer: FileSystem, StressLayerInput: `C:\temp\io.dat`, ThreadCount: 4, Duration: 30 * time.Second},
			want: []string{"-d30", "-F4", `C:\temp\io.dat`},
		},
		{
			name: "named partition without caching",
			opts: IoStressOpts{StressLayer: NamedPartition, StressLayerInput: "D", ThreadCount: 2, Duration: 10 * time.Second, DisableSwHwCaching: true},
			want: []string{"-d10", "-F2", "-Sh", "D:"},
		},
		{
			name: "physical disk",
			opts: IoStressOpts{StressLayer: PhysicalDisk, StressLayerInput: "1", ThreadCount: 1, Duration: time.Minute},
			want: []string{"-d60", "-F1", "#1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.opts.Args())
		})
	}
}

func TestIOStressLayer_IsValid(t *testing.T) {
	assert.True(t, FileSystem.IsValid())
	assert.True(t, NamedPartition.IsValid())
	assert.True(t, PhysicalDisk.IsValid())
	assert.False(t, IOStressLayer("Network Share").IsValid())
}

func TestFillMethod_IsValid(t *testing.T) {
	assert.True(t, AtOnce.IsValid())
	assert.True(t, OverTime.IsValid())
	assert.False(t, FillMethod("SOMETIMES").IsValid())
}
