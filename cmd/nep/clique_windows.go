package main

import (
	"syscall"
	"unsafe"
)

// cliqueDuplo diz se o nep foi aberto com dois cliques no Explorer: nesse
// caso o console foi criado só para ele (nenhum outro processo nele).
func cliqueDuplo() bool {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList")
	if proc.Find() != nil {
		return false
	}
	var ids [2]uint32
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&ids[0])), 2)
	return n == 1
}
