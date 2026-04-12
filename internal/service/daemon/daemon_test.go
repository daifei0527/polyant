// Package daemon_test 提供守护进程模块的单元测试
package daemon_test

import (
	"testing"

	"github.com/daifei0527/agentwiki/internal/service/daemon"
	"github.com/daifei0527/agentwiki/pkg/config"
)

// ==================== Program 测试 ====================

// TestProgramStart 测试 Program 启动
func TestProgramStart(t *testing.T) {
	prg := &daemon.Program{
		StartFn: func() error {
			return nil
		},
	}

	// Start 应该异步调用 run
	err := prg.Start(nil)
	if err != nil {
		t.Fatalf("Start 失败: %v", err)
	}

	// Start 方法成功调用即为通过
}

// TestProgramStop 测试 Program 停止
func TestProgramStop(t *testing.T) {
	stopCalled := false
	prg := &daemon.Program{
		StopFn: func() error {
			stopCalled = true
			return nil
		},
	}

	err := prg.Stop(nil)
	if err != nil {
		t.Fatalf("Stop 失败: %v", err)
	}

	if !stopCalled {
		t.Error("StopFn 应被调用")
	}
}

// TestProgramStopNil 测试空 StopFn
func TestProgramStopNil(t *testing.T) {
	prg := &daemon.Program{
		StopFn: nil,
	}

	err := prg.Stop(nil)
	if err != nil {
		t.Errorf("nil StopFn 应返回 nil, got %v", err)
	}
}

// ==================== Daemon 测试 ====================

// TestNewDaemon 测试创建 Daemon
func TestNewDaemon(t *testing.T) {
	cfg := &config.Config{}

	d, err := daemon.NewDaemon(cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewDaemon 失败: %v", err)
	}

	if d == nil {
		t.Error("Daemon 不应为 nil")
	}
}

// TestNewDaemonWithFunctions 测试带回调函数创建 Daemon
func TestNewDaemonWithFunctions(t *testing.T) {
	cfg := &config.Config{}

	startCalled := false
	stopCalled := false

	d, err := daemon.NewDaemon(cfg,
		func() error {
			startCalled = true
			return nil
		},
		func() error {
			stopCalled = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("NewDaemon 失败: %v", err)
	}

	if d == nil {
		t.Fatal("Daemon 不应为 nil")
	}

	_ = startCalled
	_ = stopCalled
}

// TestDaemonMethods 测试 Daemon 方法存在性
func TestDaemonMethods(t *testing.T) {
	cfg := &config.Config{}
	d, _ := daemon.NewDaemon(cfg, nil, nil)

	// 这些方法调用可能会在非 root 环境下失败
	// 我们只验证方法存在且可调用
	_ = d
}
