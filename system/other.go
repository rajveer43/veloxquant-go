//go:build !darwin && !linux && !windows

package system

// probe is a no-op on unsupported platforms; detection degrades gracefully
// rather than failing.
func probe(info *Info) {
	info.AppleSilicon = false
}

// memoryStats returns zero values on unsupported platforms. Callers should
// treat a zero TotalMemory as "unknown", not "no memory".
func memoryStats() (total uint64, available uint64, err error) {
	return 0, 0, nil
}
