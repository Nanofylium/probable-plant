package lang

import "runtime"

// Version é a versão da linguagem e do executável nep.
const Version = "0.1.0"

func goos() string { return runtime.GOOS }
