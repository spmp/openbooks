//go:build windows

package server

import "os/exec"

func configureHookProcess(_ *exec.Cmd) {}

func killHookProcess(command *exec.Cmd) {
	if command.Process == nil {
		return
	}

	_ = command.Process.Kill()
}
