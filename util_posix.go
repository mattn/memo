// +build !windows

package main

import "fmt"

func shellquote(s string) string {
	return fmt.Sprintf("%q", s)
}
