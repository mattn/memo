//go:build !windows

package main

import (
	"os/exec"
	"strings"
)

func shellquote(s string) string {
	return `'` + strings.Replace(s, `'`, `'\''`, -1) + `'`
}

func shellCommand(command string) *exec.Cmd {
	return exec.Command("sh", "-c", command)
}
