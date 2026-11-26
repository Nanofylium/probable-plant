//go:build !windows

package ialocal

import "os/exec"

func esconderJanela(c *exec.Cmd) {}
