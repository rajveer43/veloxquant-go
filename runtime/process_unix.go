//go:build !windows

package runtime

import (
	"os"
	"syscall"
)

// terminate sends a graceful shutdown signal (SIGTERM) to proc. The
// process is expected to catch it and exit cleanly; Process.Stop escalates
// to Kill itself if the process doesn't exit before its context is done.
func terminate(proc *os.Process) error {
	return proc.Signal(syscall.SIGTERM)
}
