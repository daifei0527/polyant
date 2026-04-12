package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/daifei0527/agentwiki/internal/service/daemon"
	"github.com/daifei0527/agentwiki/pkg/config"
	"github.com/spf13/cobra"
)

// serviceCmd 服务管理命令
var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "服务管理",
	Long:  "管理 AgentWiki 系统服务（安装、启动、停止、卸载）",
}

// serviceInstallCmd 安装服务
var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "安装系统服务",
	Long: `安装 AgentWiki 为系统服务。

Linux (systemd): 安装到 /etc/systemd/system/
macOS (launchd): 安装到 ~/Library/LaunchAgents/
Windows: 注册为 Windows 服务`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		user, _ := cmd.Flags().GetString("user")
		configFile, _ := cmd.Flags().GetString("config")

		// 加载配置
		cfg, err := loadConfig(configFile)
		if err != nil {
			return fmt.Errorf("加载配置失败: %w", err)
		}

		// 创建服务守护进程
		d, err := daemon.NewDaemon(cfg, nil, nil)
		if err != nil {
			return fmt.Errorf("创建服务失败: %w", err)
		}

		// 安装服务
		if err := d.Install(); err != nil {
			return fmt.Errorf("安装服务失败: %w", err)
		}

		fmt.Printf("✓ 服务 %s 已安装\n", name)
		if configFile != "" {
			fmt.Printf("  配置文件: %s\n", configFile)
		}
		fmt.Printf("  运行用户: %s\n", user)
		fmt.Println()
		fmt.Println("启动服务:")
		fmt.Printf("  awctl service start --name %s\n", name)

		return nil
	},
}

// serviceUninstallCmd 卸载服务
var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "卸载系统服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		configFile, _ := cmd.Flags().GetString("config")

		// 加载配置
		cfg, err := loadConfig(configFile)
		if err != nil {
			return fmt.Errorf("加载配置失败: %w", err)
		}

		// 创建服务守护进程
		d, err := daemon.NewDaemon(cfg, nil, nil)
		if err != nil {
			return fmt.Errorf("创建服务失败: %w", err)
		}

		// 先停止服务
		_ = d.Stop()

		// 卸载服务
		if err := d.Uninstall(); err != nil {
			return fmt.Errorf("卸载服务失败: %w", err)
		}

		fmt.Printf("✓ 服务 %s 已卸载\n", name)
		return nil
	},
}

// serviceStartCmd 启动服务
var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "启动服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		configFile, _ := cmd.Flags().GetString("config")

		// 尝试通过守护进程启动
		cfg, err := loadConfig(configFile)
		if err == nil {
			d, err := daemon.NewDaemon(cfg, nil, nil)
			if err == nil {
				if err := d.Start(); err != nil {
					// 回退到 systemctl
					return startViaSystemd(name)
				}
				fmt.Printf("✓ 服务 %s 已启动\n", name)
				return nil
			}
		}

		// 回退到 systemctl
		return startViaSystemd(name)
	},
}

// serviceStopCmd 停止服务
var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		configFile, _ := cmd.Flags().GetString("config")

		// 尝试通过守护进程停止
		cfg, err := loadConfig(configFile)
		if err == nil {
			d, err := daemon.NewDaemon(cfg, nil, nil)
			if err == nil {
				if err := d.Stop(); err != nil {
					// 回退到 systemctl
					return stopViaSystemd(name)
				}
				fmt.Printf("✓ 服务 %s 已停止\n", name)
				return nil
			}
		}

		// 回退到 systemctl
		return stopViaSystemd(name)
	},
}

// serviceRestartCmd 重启服务
var serviceRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "重启服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")

		// 停止
		if err := stopViaSystemd(name); err != nil {
			// 忽略停止错误
		}

		// 启动
		return startViaSystemd(name)
	},
}

// serviceStatusCmd 查看服务状态
var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看服务状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		configFile, _ := cmd.Flags().GetString("config")

		// 尝试通过守护进程获取状态
		cfg, err := loadConfig(configFile)
		if err == nil {
			d, err := daemon.NewDaemon(cfg, nil, nil)
			if err == nil {
				status, err := d.Status()
				if err == nil {
					fmt.Printf("服务状态: %s\n", status)
					return nil
				}
			}
		}

		// 回退到 systemctl
		if runtime.GOOS == "linux" {
			systemctlPath, err := exec.LookPath("systemctl")
			if err == nil {
				out, err := exec.Command(systemctlPath, "status", name).CombinedOutput()
				if err != nil {
					fmt.Printf("服务状态: 未知\n")
					return nil
				}
				fmt.Println(string(out))
				return nil
			}
		}

		// 检查 PID 文件
		pidFile := fmt.Sprintf("/var/run/%s.pid", name)
		if data, err := os.ReadFile(pidFile); err == nil {
			var pid int
			fmt.Sscanf(string(data), "%d", &pid)
			if pid > 0 {
				// 检查进程是否存在
				if err := syscall.Kill(pid, 0); err == nil {
					fmt.Printf("服务状态: 运行中 (PID: %d)\n", pid)
					return nil
				}
			}
		}

		fmt.Println("服务状态: 未运行")
		return nil
	},
}

// serviceLogsCmd 查看服务日志
var serviceLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "查看服务日志",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetInt("tail")

		// 尝试 journalctl
		if runtime.GOOS == "linux" {
			journalctlPath, err := exec.LookPath("journalctl")
			if err == nil {
				args := []string{"-u", name}
				if follow {
					args = append(args, "-f")
				}
				if tail > 0 {
					args = append(args, "-n", fmt.Sprintf("%d", tail))
				}

				exec.Command(journalctlPath, args...).Run()
				return nil
			}
		}

		// 尝试日志文件
		logFile := fmt.Sprintf("/var/log/%s/%s.log", name, name)
		if _, err := os.Stat(logFile); err == nil {
			args := []string{logFile}
			if follow {
				args = []string{"-f", logFile}
			}
			exec.Command("tail", args...).Run()
			return nil
		}

		return fmt.Errorf("未找到日志文件")
	},
}

func loadConfig(configFile string) (*config.Config, error) {
	if configFile == "" {
		// 尝试默认配置文件
		defaultPaths := []string{
			"/etc/agentwiki/config.yaml",
			"/opt/agentwiki/config.yaml",
			"./config.yaml",
		}
		for _, p := range defaultPaths {
			if _, err := os.Stat(p); err == nil {
				configFile = p
				break
			}
		}
	}

	if configFile == "" {
		return config.DefaultConfig(), nil
	}

	return config.Load(configFile)
}

func startViaSystemd(name string) error {
	if runtime.GOOS == "linux" {
		systemctlPath, err := exec.LookPath("systemctl")
		if err == nil {
			if err := exec.Command(systemctlPath, "start", name).Run(); err != nil {
				return fmt.Errorf("启动服务失败: %w", err)
			}
			fmt.Printf("✓ 服务 %s 已启动\n", name)
			return nil
		}
	}

	fmt.Printf("启动服务: %s\n", name)
	fmt.Println("请手动启动服务或使用系统服务管理器")
	return nil
}

func stopViaSystemd(name string) error {
	if runtime.GOOS == "linux" {
		systemctlPath, err := exec.LookPath("systemctl")
		if err == nil {
			if err := exec.Command(systemctlPath, "stop", name).Run(); err != nil {
				return fmt.Errorf("停止服务失败: %w", err)
			}
			fmt.Printf("✓ 服务 %s 已停止\n", name)
			return nil
		}
	}

	// 尝试通过 PID 文件停止
	pidFile := fmt.Sprintf("/var/run/%s.pid", name)
	if data, err := os.ReadFile(pidFile); err == nil {
		var pid int
		fmt.Sscanf(string(data), "%d", &pid)
		if pid > 0 {
			if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
				return fmt.Errorf("停止进程失败: %w", err)
			}
			fmt.Printf("✓ 服务 %s 已停止 (PID: %d)\n", name, pid)
			return nil
		}
	}

	fmt.Printf("停止服务: %s\n", name)
	return nil
}

func init() {
	rootCmd.AddCommand(serviceCmd)

	serviceCmd.AddCommand(serviceInstallCmd)
	serviceInstallCmd.Flags().String("name", "agentwiki", "服务名称")
	serviceInstallCmd.Flags().String("user", "agentwiki", "运行用户")
	serviceInstallCmd.Flags().String("config", "", "配置文件路径")

	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceUninstallCmd.Flags().String("name", "agentwiki", "服务名称")
	serviceUninstallCmd.Flags().String("config", "", "配置文件路径")

	serviceCmd.AddCommand(serviceStartCmd)
	serviceStartCmd.Flags().String("name", "agentwiki", "服务名称")
	serviceStartCmd.Flags().String("config", "", "配置文件路径")

	serviceCmd.AddCommand(serviceStopCmd)
	serviceStopCmd.Flags().String("name", "agentwiki", "服务名称")
	serviceStopCmd.Flags().String("config", "", "配置文件路径")

	serviceCmd.AddCommand(serviceRestartCmd)
	serviceRestartCmd.Flags().String("name", "agentwiki", "服务名称")

	serviceCmd.AddCommand(serviceStatusCmd)
	serviceStatusCmd.Flags().String("name", "agentwiki", "服务名称")
	serviceStatusCmd.Flags().String("config", "", "配置文件路径")

	serviceCmd.AddCommand(serviceLogsCmd)
	serviceLogsCmd.Flags().String("name", "agentwiki", "服务名称")
	serviceLogsCmd.Flags().BoolP("follow", "f", false, "持续输出日志")
	serviceLogsCmd.Flags().Int("tail", 100, "显示最后 N 行")
}
