//go:build !windows

package server

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func configureHookProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}

func killHookProcess(command *exec.Cmd) {
	if command.Process == nil {
		return
	}

	rootPID := command.Process.Pid
	descendants := descendantPIDs(rootPID)

	pgid, err := syscall.Getpgid(rootPID)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	} else {
		_ = syscall.Kill(-rootPID, syscall.SIGKILL)
	}

	for _, pid := range descendants {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}

	_ = syscall.Kill(rootPID, syscall.SIGKILL)
}

func descendantPIDs(rootPID int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	childrenByParent := map[int][]int{}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		ppid, err := readParentPID(pid)
		if err != nil {
			continue
		}

		childrenByParent[ppid] = append(childrenByParent[ppid], pid)
	}

	result := make([]int, 0)
	queue := []int{rootPID}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]

		for _, child := range childrenByParent[parent] {
			result = append(result, child)
			queue = append(queue, child)
		}
	}

	return result
}

func readParentPID(pid int) (int, error) {
	bytes, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, err
	}

	parts := strings.Fields(string(bytes))
	if len(parts) < 4 {
		return 0, syscall.EINVAL
	}

	ppid, err := strconv.Atoi(parts[3])
	if err != nil {
		return 0, err
	}

	return ppid, nil
}
