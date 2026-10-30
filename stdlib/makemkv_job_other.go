//go:build !windows

package stdlib

import "os/exec"

func makeMKVInitJob()                {}
func makeMKVAttachJob(cmd *exec.Cmd) {}
