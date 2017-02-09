// +build !windows

package main

import "fmt"

func shellquote(s string) string {
	return `'` + strings.Replace(s, `'`, `'\''`, -1) + `'`
}
