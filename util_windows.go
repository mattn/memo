//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func shellquote(s string) string {
	return syscall.EscapeArg(s)
}

// shellCommand builds a *exec.Cmd that runs the given command line through
// cmd.exe. SysProcAttr.CmdLine is set explicitly so that Go's os/exec does not
// re-escape the already quoted command line; otherwise file paths containing
// spaces would be split into multiple arguments. The /s switch tells cmd.exe to
// simply strip the outermost quotes and use the rest verbatim.
func shellCommand(command string) *exec.Cmd {
	cmd := exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd /s /c "` + command + `"`}
	return cmd
}
