//go:build !windows

package main

// cliqueDuplo: fora do Windows, o nep sem argumentos abre o modo interativo.
func cliqueDuplo() bool { return false }
