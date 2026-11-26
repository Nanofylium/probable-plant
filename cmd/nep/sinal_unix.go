//go:build !windows

package main

import "syscall"

var sigTerm = syscall.SIGTERM
