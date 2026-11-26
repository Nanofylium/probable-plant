package ialocal

import (
	"os/exec"
	"syscall"
)

// esconderJanela: o llama-server roda sem abrir janela preta.
func esconderJanela(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
