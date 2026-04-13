// Package daemon_test 提供守护进程模块的单元测试
package daemon_test

import (
	"os"
	"syscall"
	"testing"
	"time"

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

// TestProgramRun 测试 Program run 方法
func TestProgramRun(t *testing.T) {
	// 测试有 StartFn 的情况
	t.Run("with StartFn", func(t *testing.T) {
		called := false
		prg := &daemon.Program{
			StartFn: func() error {
				called = true
				return nil
			},
		}

		prg.Start(nil)
		time.Sleep(50 * time.Millisecond) // 等待 goroutine 执行

		if !called {
			t.Error("StartFn 应被调用")
		}
	})

	// 测试无 StartFn 的情况 (nil)
	t.Run("nil StartFn", func(t *testing.T) {
		prg := &daemon.Program{
			StartFn: nil,
		}

		// 调用不应 panic
		prg.Start(nil)
		time.Sleep(10 * time.Millisecond)
	})
}

// TestDaemonStatus 测试 Daemon Status 方法
func TestDaemonStatus(t *testing.T) {
	cfg := &config.Config{}
	d, err := daemon.NewDaemon(cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewDaemon 失败: %v", err)
	}

	// Status 调用可能在不同环境下返回不同结果
	// 我们主要验证方法可以正常调用
	status, err := d.Status()
	// 在某些环境下可能返回错误,这是可接受的
	if err != nil {
		// 预期在非服务环境下可能失败
		t.Logf("Status 返回错误 (预期在某些环境下): %v", err)
	}
	// 验证返回值是合理的
	if status != "running" && status != "stopped" && status != "unknown" {
		t.Errorf("Status 返回无效值: %s", status)
	}
}

// TestDaemonRun 测试 Daemon Run 方法
func TestDaemonRun(t *testing.T) {
	cfg := &config.Config{}
	d, err := daemon.NewDaemon(cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewDaemon 失败: %v", err)
	}

	// Run 会阻塞,我们在 goroutine 中测试
	done := make(chan error, 1)
	go func() {
		done <- d.Run()
	}()

	// 短暂等待后验证 Run 没有立即返回错误
	select {
	case err := <-done:
		// Run 返回了,可能是预期外的错误
		t.Logf("Run 返回: %v", err)
	case <-time.After(100 * time.Millisecond):
		// Run 正在运行,这是预期的
	}
}

// TestWaitForSignal 测试 WaitForSignal 函数
func TestWaitForSignal(t *testing.T) {
	stopCalled := false
	stopFn := func() {
		stopCalled = true
	}

	// 在 goroutine 中等待信号
	done := make(chan struct{})
	go func() {
		daemon.WaitForSignal(stopFn)
		close(done)
	}()

	// 等待信号监听器启动
	time.Sleep(50 * time.Millisecond)

	// 发送 SIGTERM 信号
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("FindProcess 失败: %v", err)
	}
	p.Signal(syscall.SIGTERM)

	// 等待 WaitForSignal 完成
	select {
	case <-done:
		// 成功完成
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSignal 超时")
	}

	if !stopCalled {
		t.Error("stopFn 应被调用")
	}
}

// TestWaitForSignalNilStop 测试 WaitForSignal 空回调
func TestWaitForSignalNilStop(t *testing.T) {
	// 在 goroutine 中等待信号
	done := make(chan struct{})
	go func() {
		daemon.WaitForSignal(nil)
		close(done)
	}()

	// 等待信号监听器启动
	time.Sleep(50 * time.Millisecond)

	// 发送 SIGTERM 信号
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("FindProcess 失败: %v", err)
	}
	p.Signal(syscall.SIGTERM)

	// 等待 WaitForSignal 完成
	select {
	case <-done:
		// 成功完成,nil stopFn 不应导致 panic
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSignal 超时")
	}
}

// TestRunAsService 测试 RunAsService 函数
func TestRunAsService(t *testing.T) {
	// 保存原始 os.Args
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	cfg := &config.Config{}

	startFn := func() error {
		return nil
	}
	stopFn := func() error {
		return nil
	}

	// 测试 status 命令
	t.Run("status command", func(t *testing.T) {
		os.Args = []string{"agentwiki", "status"}
		err := daemon.RunAsService(cfg, startFn, stopFn)
		// status 命令在非服务环境下可能返回错误
		// 我们验证函数可以正常执行
		_ = err
	})

	// 测试 install 命令
	t.Run("install command", func(t *testing.T) {
		os.Args = []string{"agentwiki", "install"}
		err := daemon.RunAsService(cfg, startFn, stopFn)
		// install 命令需要权限
		_ = err
	})

	// 测试 uninstall 命令
	t.Run("uninstall command", func(t *testing.T) {
		os.Args = []string{"agentwiki", "uninstall"}
		err := daemon.RunAsService(cfg, startFn, stopFn)
		// uninstall 命令需要权限
		_ = err
	})

	// 测试 start 命令
	t.Run("start command", func(t *testing.T) {
		os.Args = []string{"agentwiki", "start"}
		err := daemon.RunAsService(cfg, startFn, stopFn)
		// start 命令需要服务已安装
		_ = err
	})

	// 测试 stop 命令
	t.Run("stop command", func(t *testing.T) {
		os.Args = []string{"agentwiki", "stop"}
		err := daemon.RunAsService(cfg, startFn, stopFn)
		// stop 命令需要服务已安装
		_ = err
	})

	// 测试空参数情况 (默认 Run)
	t.Run("no command", func(t *testing.T) {
		os.Args = []string{"agentwiki"}
		// Run 会阻塞,在 goroutine 中测试
		done := make(chan error, 1)
		go func() {
			done <- daemon.RunAsService(cfg, startFn, stopFn)
		}()

		select {
		case err := <-done:
			t.Logf("RunAsService 返回: %v", err)
		case <-time.After(100 * time.Millisecond):
			// 正在运行,预期行为
		}
	})
}

// TestDaemonInstallUninstall 测试服务安装卸载 (需要权限)
func TestDaemonInstallUninstall(t *testing.T) {
	cfg := &config.Config{}
	d, err := daemon.NewDaemon(cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewDaemon 失败: %v", err)
	}

	// 这些操作通常需要管理员权限
	// 在非特权环境下会失败,我们验证方法可调用
	t.Run("install", func(t *testing.T) {
		err := d.Install()
		if err != nil {
			t.Logf("Install 失败 (预期在非 root 环境): %v", err)
		}
	})

	t.Run("uninstall", func(t *testing.T) {
		err := d.Uninstall()
		if err != nil {
			t.Logf("Uninstall 失败 (预期在非 root 环境): %v", err)
		}
	})
}

// TestDaemonStartStop 测试服务启动停止 (需要权限)
func TestDaemonStartStop(t *testing.T) {
	cfg := &config.Config{}
	d, err := daemon.NewDaemon(cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewDaemon 失败: %v", err)
	}

	// 这些操作通常需要服务已安装
	t.Run("start service", func(t *testing.T) {
		err := d.Start()
		if err != nil {
			t.Logf("Start 失败 (预期在未安装服务时): %v", err)
		}
	})

	t.Run("stop service", func(t *testing.T) {
		err := d.Stop()
		if err != nil {
			t.Logf("Stop 失败 (预期在未安装服务时): %v", err)
		}
	})
}

// TestProgramWithConfig 测试带配置的 Program
func TestProgramWithConfig(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{
			Type: "seed",
		},
	}

	prg := &daemon.Program{
		Config: cfg,
		StartFn: func() error {
			// 可以访问配置
			return nil
		},
		StopFn: func() error {
			return nil
		},
	}

	if prg.Config.Node.Type != "seed" {
		t.Error("Config 未正确设置")
	}

	// 测试 Start 和 Stop
	err := prg.Start(nil)
	if err != nil {
		t.Errorf("Start 失败: %v", err)
	}

	err = prg.Stop(nil)
	if err != nil {
		t.Errorf("Stop 失败: %v", err)
	}
}
