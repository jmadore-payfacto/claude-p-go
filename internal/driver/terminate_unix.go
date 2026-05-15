//go:build !windows

package driver

import (
	"syscall"
	"time"

	"github.com/aymanbagabas/go-pty"
)

// terminateChild asks the child to exit with SIGTERM, then escalates to
// SIGKILL if it doesn't stop within terminateGrace.
func terminateChild(cmd *pty.Cmd) {
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(terminateGrace):
		_ = cmd.Process.Kill()
		<-done
	}
}
