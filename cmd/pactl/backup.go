package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
	"github.com/spf13/cobra"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/pkg/config"
)

// backupCmd 备份管理命令组（离线操作，不需要节点在线）
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "KV 备份管理（离线；restore 需先停节点）",
	Long: `KV 备份管理工具。

离线操作，直接读写磁盘上的备份目录和 KV 数据。
restore 操作要求节点已停止，否则 KV 锁文件冲突。

子命令:
  list     列出本地备份
  restore  从备份恢复 KV`,
}

// backupListCmd 列出本地备份
var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出本地备份（读 <DataDir>/backups）",
	Long:  "扫描 <DataDir>/backups/ 下所有备份目录，读取 manifest.json 并显示。",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, dataDir, err := resolveKVMeta()
		if err != nil {
			return err
		}
		backupDir := filepath.Join(dataDir, "backups")
		entries, err := os.ReadDir(backupDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("(无备份)")
				return nil
			}
			return err
		}

		count := 0
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			b, err := os.ReadFile(filepath.Join(backupDir, e.Name(), "manifest.json"))
			if err != nil {
				continue
			}
			var m struct {
				Engine    string `json:"engine"`
				CreatedAt int64  `json:"created_at"`
				SizeBytes int64  `json:"size_bytes"`
				KeyCount  int64  `json:"key_count"`
			}
			if json.Unmarshal(b, &m) != nil {
				continue
			}
			fmt.Printf("%s  engine=%s  size=%d  keys=%d\n", e.Name(), m.Engine, m.SizeBytes, m.KeyCount)
			count++
		}
		if count == 0 {
			fmt.Println("(无备份)")
		}
		return nil
	},
}

// backupRestoreCmd 从备份目录恢复 KV
var backupRestoreCmd = &cobra.Command{
	Use:   "restore <backup-dir>",
	Short: "从备份目录离线恢复 KV（必须先停节点）",
	Long: `从指定的备份目录恢复 KV 数据。

警告：此操作会替换当前 KV 数据！执行前请确保：
  1. 节点已停止（否则 KV 锁文件冲突）
  2. 已备份当前数据

Pebble：备份目录（Checkpoint 快照）是一个有效的 Pebble DB，直接交换目录。
Badger：使用 badger.Load 将 backup.bak 合并到现有 KV。

示例:
  pactl backup restore ./data/backups/1719876543210`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		srcDir := args[0]
		kvType, dataDir, err := resolveKVMeta()
		if err != nil {
			return err
		}
		kvPath := filepath.Join(dataDir, "kv")
		if err := assertNodeStopped(kvPath, kvType); err != nil {
			return err
		}

		switch kvType {
		case "badger":
			return restoreBadger(kvPath, srcDir)
		default: // pebble
			return restorePebble(kvPath, srcDir)
		}
	},
}

// resolveKVMeta 从配置文件解析 KVType 和 DataDir。
// configPath 是 main.go 中 --config 绑定的全局变量。
func resolveKVMeta() (kvType, dataDir string, err error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return "", "", fmt.Errorf("load config: %w", err)
	}
	if cfg.Node.DataDir == "" {
		cfg.Node.DataDir = "./data"
	}
	return cfg.Storage.KVType, cfg.Node.DataDir, nil
}

// assertNodeStopped 尝试以只读模式打开 KV；如果锁文件冲突则节点仍在运行。
func assertNodeStopped(kvPath, kvType string) error {
	switch kvType {
	case "badger":
		db, err := badger.Open(badger.DefaultOptions(kvPath).WithReadOnly(true))
		if err != nil {
			return fmt.Errorf("无法打开 KV（节点可能正在运行，请先停节点）: %w", err)
		}
		db.Close()
	default:
		s, err := kv.NewPebbleStore(kvPath)
		if err != nil {
			return fmt.Errorf("无法打开 KV（节点可能正在运行，请先停节点）: %w", err)
		}
		s.Close()
	}
	return nil
}

// restorePebble 通过交换目录恢复 Pebble KV。
// Pebble Checkpoint 产生的备份目录是一个有效的 Pebble DB 目录，可直接替换。
func restorePebble(kvPath, srcDir string) error {
	tmp := kvPath + ".old"
	if err := os.Rename(kvPath, tmp); err != nil {
		return fmt.Errorf("移开旧 KV: %w", err)
	}
	if err := os.Rename(srcDir, kvPath); err != nil {
		// 尝试回滚
		_ = os.Rename(tmp, kvPath)
		return fmt.Errorf("移入备份: %w", err)
	}
	_ = os.RemoveAll(tmp)
	fmt.Printf("Pebble 恢复完成: %s -> %s\n请重启节点。\n", srcDir, kvPath)
	return nil
}

// restoreBadger 使用 badger.Load 将备份文件合并到现有 KV。
// badger.Load 将 backup.bak 中的数据合并到已打开的 DB 中。
func restoreBadger(kvPath, srcDir string) error {
	db, err := badger.Open(badger.DefaultOptions(kvPath))
	if err != nil {
		return fmt.Errorf("open KV for Load: %w", err)
	}
	defer db.Close()

	f, err := os.Open(filepath.Join(srcDir, "backup.bak"))
	if err != nil {
		return fmt.Errorf("open backup.bak: %w", err)
	}
	defer f.Close()

	if err := db.Load(f, 256); err != nil {
		return fmt.Errorf("badger Load: %w", err)
	}
	fmt.Printf("Badger 恢复完成: 从 %s 载入 %s\n请重启节点。\n", srcDir, kvPath)
	return nil
}

func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupRestoreCmd)
}
