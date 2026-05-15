//go:build windows

package driver

import "github.com/aymanbagabas/go-pty"

// terminateChild kills the child. Windows console processes have no
// graceful-termination signal, so there is no SIGTERM grace period.
func terminateChild(cmd *pty.Cmd) {
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}
