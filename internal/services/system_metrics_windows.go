//go:build windows

package services

import (
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes        = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx  = kernel32.NewProc("GlobalMemoryStatusEx")
	lastSystemCPUSample       windowsCPUSample
	lastSystemCPUSampleLocker sync.Mutex
)

type windowsCPUSample struct {
	idle  uint64
	total uint64
	at    time.Time
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func systemCPUPercent() (float64, error) {
	lastSystemCPUSampleLocker.Lock()
	defer lastSystemCPUSampleLocker.Unlock()
	sample, err := readWindowsCPUSample()
	if err != nil {
		return 0, err
	}
	if lastSystemCPUSample.total == 0 {
		lastSystemCPUSample = sample
		return 0, nil
	}
	totalDelta := sample.total - lastSystemCPUSample.total
	idleDelta := sample.idle - lastSystemCPUSample.idle
	lastSystemCPUSample = sample
	if totalDelta == 0 {
		return 0, nil
	}
	return (1 - float64(idleDelta)/float64(totalDelta)) * 100, nil
}

func readWindowsCPUSample() (windowsCPUSample, error) {
	var idle, kernel, user windows.Filetime
	r1, _, err := procGetSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if r1 == 0 {
		return windowsCPUSample{}, err
	}
	idleTicks := filetimeTicks(idle)
	kernelTicks := filetimeTicks(kernel)
	userTicks := filetimeTicks(user)
	return windowsCPUSample{idle: idleTicks, total: kernelTicks + userTicks, at: time.Now()}, nil
}

func filetimeTicks(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

func systemMemory() (memorySnapshot, error) {
	var status memoryStatusEx
	status.Length = uint32(unsafe.Sizeof(status))
	r1, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if r1 == 0 {
		return memorySnapshot{}, err
	}
	return memorySnapshot{Total: status.TotalPhys, Used: status.TotalPhys - status.AvailPhys, Available: status.AvailPhys}, nil
}

func diskUsage(path string) (diskSnapshot, error) {
	if path == "" {
		path = "."
	}
	abs, err := os.Getwd()
	if err == nil {
		path = abs
	}
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return diskSnapshot{}, err
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &free, &total, &totalFree); err != nil {
		return diskSnapshot{}, err
	}
	return diskSnapshot{Total: total, Used: total - totalFree}, nil
}
