//go:build windows

package runtime

import "os"

// terminate has no graceful-shutdown equivalent to SIGTERM on Windows via
// os.Process, so it kills the process directly; Process.Stop's escalation
// to Kill after ctx is done becomes a no-op immediately following this.
func terminate(proc *os.Process) error {
	return proc.Kill()
}
