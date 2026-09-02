//go:build darwin

package system

import (
	"os/exec"
	"strconv"
	"strings"
)

// probe fills in Darwin-specific fields: architecture-based Apple Silicon
// detection and CPU model via sysctl.
func probe(info *Info) {
	info.AppleSilicon = info.Architecture == "arm64"

	if model, err := sysctlString("machdep.cpu.brand_string"); err == nil {
		info.CPUModel = model
	} else if info.AppleSilicon {
		info.CPUModel = "Apple Silicon"
	}
}

// memoryStats returns total and available physical memory in bytes on macOS.
func memoryStats() (total uint64, available uint64, err error) {
	total, err = sysctlUint64("hw.memsize")
	if err != nil {
		return 0, 0, err
	}

	available = availableMemoryFromVMStat(total)
	return total, available, nil
}

func sysctlString(name string) (string, error) {
	out, err := exec.Command("sysctl", "-n", name).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func sysctlUint64(name string) (uint64, error) {
	s, err := sysctlString(name)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(s, 10, 64)
}

// availableMemoryFromVMStat estimates available memory using `vm_stat`,
// which reports page counts for free/inactive/speculative pages. If parsing
// fails for any reason, it falls back to the total memory value so callers
// still get a usable (if conservative) number.
func availableMemoryFromVMStat(total uint64) uint64 {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return total
	}

	pageSize := uint64(4096)
	var free, inactive, speculative uint64

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "Mach Virtual Memory Statistics"):
			if n := extractPageSize(line); n > 0 {
				pageSize = n
			}
		case strings.HasPrefix(line, "Pages free:"):
			free = extractPageCount(line)
		case strings.HasPrefix(line, "Pages inactive:"):
			inactive = extractPageCount(line)
		case strings.HasPrefix(line, "Pages speculative:"):
			speculative = extractPageCount(line)
		}
	}

	available := (free + inactive + speculative) * pageSize
	if available == 0 || available > total {
		return total
	}
	return available
}

func extractPageCount(line string) uint64 {
	parts := strings.Split(line, ":")
	if len(parts) != 2 {
		return 0
	}
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[1]), "."))
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// extractPageSize parses "page size of 16384 bytes" from the vm_stat header.
func extractPageSize(line string) uint64 {
	const marker = "page size of "
	idx := strings.Index(line, marker)
	if idx == -1 {
		return 0
	}
	rest := line[idx+len(marker):]
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return 0
	}
	n, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0
	}
	return n
}
