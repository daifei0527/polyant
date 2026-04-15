package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// keyCmd 密钥管理命令
var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "密钥管理",
	Long:  "管理 Ed25519 密钥对，用于 API 认证",
}

// keyGenerateCmd 生成密钥对
var keyGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "生成新的密钥对",
	Long: `生成新的 Ed25519 密钥对。

密钥将保存在 ~/.polyant/keys/ 目录下。
如果密钥已存在，将显示警告并提示覆盖选项。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		keyDir := GetDefaultKeyDir()
		force, _ := cmd.Flags().GetBool("force")

		// 检查是否已有密钥
		if client.HasKeys() && !force {
			return fmt.Errorf("密钥已存在，使用 --force 覆盖现有密钥")
		}

		// 确保目录存在
		if err := EnsureKeyDirExists(); err != nil {
			return fmt.Errorf("创建密钥目录失败: %w", err)
		}

		// 重新生成密钥
		if err := client.LoadOrGenerateKeys(keyDir); err != nil {
			return fmt.Errorf("生成密钥失败: %w", err)
		}

		pubKey := client.GetPublicKey()
		fmt.Println("密钥对已生成并保存到:", keyDir)
		fmt.Println()
		fmt.Println("公钥 (Base64):")
		fmt.Println("  " + pubKey)
		fmt.Println()
		fmt.Println("现在可以使用以下命令注册用户:")
		fmt.Printf("  pactl user register --name \"your-agent-name\"\n")

		return nil
	},
}

// keyShowCmd 显示当前公钥
var keyShowCmd = &cobra.Command{
	Use:   "show",
	Short: "显示当前公钥",
	Long: `显示当前加载的公钥信息。

如果密钥尚未加载，将从默认目录加载密钥。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 尝试加载密钥
		if !client.HasKeys() {
			keyDir := GetDefaultKeyDir()
			if err := client.LoadOrGenerateKeys(keyDir); err != nil {
				return fmt.Errorf("加载密钥失败: %w", err)
			}
		}

		pubKey := client.GetPublicKey()
		format, _ := cmd.Flags().GetString("format")

		switch format {
		case "hex":
			// 先解码 base64，再转为 hex
			decoded, err := hex.DecodeString(pubKey)
			if err != nil {
				return fmt.Errorf("解码公钥失败: %w", err)
			}
			fmt.Println(hex.EncodeToString(decoded))
		case "base64":
			fmt.Println(pubKey)
		default:
			fmt.Println("公钥信息:")
			fmt.Printf("  Base64: %s\n", pubKey)
			decoded, err := hex.DecodeString(pubKey)
			if err == nil {
				fmt.Printf("  Hex:     %s\n", hex.EncodeToString(decoded))
			}
			fmt.Printf("  长度:    %d 字节\n", 32) // Ed25519 公钥固定 32 字节
		}

		return nil
	},
}

// keyExportCmd 导出公钥
var keyExportCmd = &cobra.Command{
	Use:   "export",
	Short: "导出公钥到文件",
	Long: `将公钥导出到指定文件。

默认格式为 Base64，可通过 --format 指定为 hex。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 确保密钥已加载
		if !client.HasKeys() {
			keyDir := GetDefaultKeyDir()
			if err := client.LoadOrGenerateKeys(keyDir); err != nil {
				return fmt.Errorf("加载密钥失败: %w", err)
			}
		}

		output, _ := cmd.Flags().GetString("output")
		format, _ := cmd.Flags().GetString("format")

		pubKey := client.GetPublicKey()
		var content string

		switch format {
		case "hex":
			decoded, err := hex.DecodeString(pubKey)
			if err != nil {
				return fmt.Errorf("解码公钥失败: %w", err)
			}
			content = hex.EncodeToString(decoded)
		default:
			content = pubKey
		}

		if output == "" {
			fmt.Println(content)
			return nil
		}

		// 写入文件
		if err := os.WriteFile(output, []byte(content+"\n"), 0644); err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}

		fmt.Printf("公钥已导出到: %s\n", output)
		return nil
	},
}

// keyStatusCmd 显示密钥状态
var keyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "显示密钥状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		keyDir := GetDefaultKeyDir()

		fmt.Println("密钥状态:")
		fmt.Printf("  密钥目录: %s\n", keyDir)

		if client.HasKeys() {
			fmt.Println("  状态:     已加载")
			fmt.Printf("  公钥:     %s...\n", client.GetPublicKey()[:16])
		} else {
			// 尝试加载
			err := client.LoadOrGenerateKeys(keyDir)
			if err != nil {
				fmt.Println("  状态:     未找到密钥")
				fmt.Println()
				fmt.Println("提示: 运行 'pactl key generate' 生成密钥")
			} else {
				fmt.Println("  状态:     已加载")
				fmt.Printf("  公钥:     %s...\n", client.GetPublicKey()[:16])
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(keyCmd)

	// key generate
	keyCmd.AddCommand(keyGenerateCmd)
	keyGenerateCmd.Flags().BoolP("force", "f", false, "覆盖现有密钥")

	// key show
	keyCmd.AddCommand(keyShowCmd)
	keyShowCmd.Flags().String("format", "", "输出格式 (base64|hex)")

	// key export
	keyCmd.AddCommand(keyExportCmd)
	keyExportCmd.Flags().StringP("output", "o", "", "输出文件路径")
	keyExportCmd.Flags().String("format", "base64", "输出格式 (base64|hex)")

	// key status
	keyCmd.AddCommand(keyStatusCmd)
}
