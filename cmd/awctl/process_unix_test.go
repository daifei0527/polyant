//go:build !windows
// +build !windows

package main

import (
	"os/exec"
	"testing"
	"time"
)

func TestTerminateProcess(t *testing.T) {
	// Start a sleep process
	cmd := exec.Command("sleep", "10")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	pid := cmd.Process.Pid

	// Verify process exists
	if !processExists(pid) {
		t.Fatalf("Process (PID %d) should exist", pid)
	}

	// Terminate the process
	err = terminateProcess(pid)
	if err != nil {
		t.Errorf("terminateProcess failed: %v", err)
	}

	// Wait for process to terminate
	cmd.Wait()

	// Give it a moment for the process to fully terminate
	time.Sleep(100 * time.Millisecond)

	// Verify process no longer exists
	if processExists(pid) {
		t.Error("Process should not exist after termination")
	}
}

func TestTerminateProcess_NonExistent(t *testing.T) {
	// Try to terminate a non-existent process
	err := terminateProcess(9999999)
	if err == nil {
		t.Error("terminateProcess should return error for non-existent process")
	}
}
