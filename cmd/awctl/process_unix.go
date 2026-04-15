//go:build !windows
// +build !windows

package main

import (
	"os"
	"syscall"
)

// processExists 检查进程是否存在（Unix 版本）
func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// terminateProcess 终止进程（Unix 版本）
func terminateProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}
