package ialocal

import _ "embed"

// llama-win-x64.zip: llama-server do llama.cpp (MIT) compilado com MinGW-w64,
// com as bibliotecas do GCC junto. Só depende de DLLs que todo Windows tem
// (kernel32, msvcrt, advapi32, ws2_32). Refazer com scripts/motor-windows.sh.
//
//go:embed motor/llama-win-x64.zip
var motorZip []byte

func init() { motorEmbutido = motorZip }
