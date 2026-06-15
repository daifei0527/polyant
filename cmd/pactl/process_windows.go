//go:build windows
// +build windows

package main

import (
	"os"
	"syscall"
)

// processExists 检查进程是否存在（Windows 版本，非破坏性探测）
//
// 用 OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION) + GetExitCodeProcess 判断，
// 不向目标进程发送任何信号。之前的实现误用 proc.Kill() 来"探测"存在性——
// 这会真的终止进程（破坏性副作用）。PROCESS_QUERY_LIMITED_INFORMATION (0x1000)
// 允许查询包括受保护进程在内的大多数进程的退出码而无需更高权限。
func processExists(pid int) bool {
	const (
		processQueryLimitedInformation = 0x1000
		// stillActive 是 GetExitCodeProcess 对仍在运行的进程返回的退出码
		// (STATUS_PENDING)。stdlib syscall 未导出该常量，故在此定义。
		stillActive = 259
	)
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	var code uint32
	if err := syscall.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}
	return code == stillActive
}

// terminateProcess 终止进程（Windows 版本）
func terminateProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
