//go:build linux

package system

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// probe fills in Linux-specific fields. Apple Silicon does not apply on
// Linux, so AppleSilicon is always false here.
func probe(info *Info) {
	info.AppleSilicon = false
	info.CPUModel = cpuModelFromProcCPUInfo()
}

func cpuModelFromProcCPUInfo() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// memoryStats returns total and available memory in bytes, parsed from
// /proc/meminfo (values there are reported in kB).
func memoryStats() (total uint64, available uint64, err error) {
	f, ferr := os.Open("/proc/meminfo")
	if ferr != nil {
		return 0, 0, ferr
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = parseMemInfoKB(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			available = parseMemInfoKB(line)
		}
	}

	if available == 0 {
		available = total
	}

	return total, available, nil
}

func parseMemInfoKB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	n, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return n * 1024
}
