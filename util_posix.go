// +build !windows

package main

import (
	"strings"
)

func shellquote(s string) string {
	return `'` + strings.Replace(s, `'`, `'\''`, -1) + `'`
}
