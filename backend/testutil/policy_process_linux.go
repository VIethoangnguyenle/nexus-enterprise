//go:build realpolicy && linux

package testutil

import (
	osexec "os/exec"
	"syscall"
)

// dieWithParent makes the kernel kill the policy process if the test binary is
// killed (a timeout, a panic, ^C) before StopPolicy runs, so a failed run leaves
// no orphan holding a port and a database connection.
func dieWithParent(cmd *osexec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
