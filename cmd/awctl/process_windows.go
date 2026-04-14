// +build windows

package main

import (
	"os"
)

// processExists 检查进程是否存在（Windows 版本）
func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// 在 Windows 上，Signal(nil) 不支持检查进程存在
	// 我们尝试发送一个终止信号，如果失败说明进程不存在
	err = proc.Kill()
	if err != nil {
		return false
	}
	return true
}

// terminateProcess 终止进程（Windows 版本）
func terminateProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
