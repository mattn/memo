// +build windows

package main

import "syscall"

func shellquote(s string) string {
	return syscall.EscapeArg(s)
}
