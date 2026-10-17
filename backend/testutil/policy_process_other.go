//go:build realpolicy && !linux

package testutil

import osexec "os/exec"

// dieWithParent has no portable equivalent off Linux; StopPolicy, run from
// TestMain, is the only cleanup there.
func dieWithParent(*osexec.Cmd) {}
