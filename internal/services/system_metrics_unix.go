//go:build !windows

package services

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

var (
	lastUnixCPUSample       unixCPUSample
	lastUnixCPUSampleLocker sync.Mutex
)

type unixCPUSample struct {
	idle  uint64
	total uint64
}

func systemCPUPercent() (float64, error) {
	lastUnixCPUSampleLocker.Lock()
	defer lastUnixCPUSampleLocker.Unlock()
	sample, err := readUnixCPUSample()
	if err != nil {
		return 0, err
	}
	if lastUnixCPUSample.total == 0 {
		lastUnixCPUSample = sample
		return 0, nil
	}
	totalDelta := sample.total - lastUnixCPUSample.total
	idleDelta := sample.idle - lastUnixCPUSample.idle
	lastUnixCPUSample = sample
	if totalDelta == 0 {
		return 0, nil
	}
	return (1 - float64(idleDelta)/float64(totalDelta)) * 100, nil
}

func readUnixCPUSample() (unixCPUSample, error) {
	bytes, err := os.ReadFile("/proc/stat")
	if err != nil {
		return unixCPUSample{}, err
	}
	line := strings.SplitN(string(bytes), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return unixCPUSample{}, fmt.Errorf("无法读取 CPU 统计")
	}
	var total uint64
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return unixCPUSample{}, err
		}
		values = append(values, value)
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return unixCPUSample{idle: idle, total: total}, nil
}

func systemMemory() (memorySnapshot, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return memorySnapshot{}, err
	}
	unit := uint64(info.Unit)
	if unit == 0 {
		unit = 1
	}
	total := info.Totalram * unit
	free := info.Freeram * unit
	return memorySnapshot{Total: total, Used: total - free}, nil
}

func diskUsage(path string) (diskSnapshot, error) {
	var stat unix.Statfs_t
	if path == "" {
		path = "."
	}
	if err := unix.Statfs(path, &stat); err != nil {
		return diskSnapshot{}, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	return diskSnapshot{Total: total, Used: total - free}, nil
}
