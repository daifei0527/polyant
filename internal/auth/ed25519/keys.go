// Package ed25519 提供 Ed25519 密钥管理功能
// 包含密钥生成、签名、验证和持久化存储
package ed25519

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// KeyPair 密钥对结构体，用于 JSON 序列化
type KeyPair struct {
	PrivateKey string `json:"private_key"` // Base64 编码的私钥
	PublicKey  string `json:"public_key"`  // Base64 编码的公钥
}

// GenerateKeyPair 生成新的 Ed25519 密钥对
// 返回公钥和私钥的字节切片
func GenerateKeyPair() (publicKey, privateKey []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("生成密钥对失败: %w", err)
	}
	return pub, priv, nil
}

// Sign 使用私钥对消息进行签名
// 返回签名字节切片
func Sign(privateKey, message []byte) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("私钥长度无效，期望 %d 字节", ed25519.PrivateKeySize)
	}
	signature := ed25519.Sign(privateKey, message)
	return signature, nil
}

// Verify 使用公钥验证消息签名
// 返回签名是否有效
func Verify(publicKey, message, signature []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(publicKey, message, signature)
}

// PublicKeyToString 将公钥字节切片转换为 Base64 编码字符串
func PublicKeyToString(pubKey []byte) string {
	return base64.StdEncoding.EncodeToString(pubKey)
}

// StringToPublicKey 将 Base64 编码的字符串转换为公钥字节切片
func StringToPublicKey(s string) ([]byte, error) {
	pubKey, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("解码公钥失败: %w", err)
	}
	if len(pubKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("公钥长度无效，期望 %d 字节，实际 %d 字节", ed25519.PublicKeySize, len(pubKey))
	}
	return pubKey, nil
}

// PrivateKeyToString 将私钥字节切片转换为 Base64 编码字符串
func PrivateKeyToString(privKey []byte) string {
	return base64.StdEncoding.EncodeToString(privKey)
}

// StringToPrivateKey 将 Base64 编码的字符串转换为私钥字节切片
func StringToPrivateKey(s string) ([]byte, error) {
	privKey, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("解码私钥失败: %w", err)
	}
	if len(privKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("私钥长度无效，期望 %d 字节，实际 %d 字节", ed25519.PrivateKeySize, len(privKey))
	}
	return privKey, nil
}

// SaveKeyPair 将密钥对保存到指定目录
// 会在目录下创建 private_key.json 和 public_key.json 两个文件
func SaveKeyPair(privateKey, publicKey []byte, dir string) error {
	// 确保目录存在
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建密钥目录失败: %w", err)
	}

	// 保存完整的密钥对到 keypair.json
	keyPair := KeyPair{
		PrivateKey: PrivateKeyToString(privateKey),
		PublicKey:  PublicKeyToString(publicKey),
	}

	data, err := json.MarshalIndent(keyPair, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化密钥对失败: %w", err)
	}

	keyPairPath := filepath.Join(dir, "keypair.json")
	if err := ioutil.WriteFile(keyPairPath, data, 0600); err != nil {
		return fmt.Errorf("写入密钥对文件失败: %w", err)
	}

	// 单独保存私钥
	privData, err := json.MarshalIndent(map[string]string{
		"private_key": PrivateKeyToString(privateKey),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化私钥失败: %w", err)
	}

	privPath := filepath.Join(dir, "private_key.json")
	if err := ioutil.WriteFile(privPath, privData, 0600); err != nil {
		return fmt.Errorf("写入私钥文件失败: %w", err)
	}

	// 单独保存公钥
	pubData, err := json.MarshalIndent(map[string]string{
		"public_key": PublicKeyToString(publicKey),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化公钥失败: %w", err)
	}

	pubPath := filepath.Join(dir, "public_key.json")
	if err := ioutil.WriteFile(pubPath, pubData, 0644); err != nil {
		return fmt.Errorf("写入公钥文件失败: %w", err)
	}

	return nil
}

// LoadKeyPair 从指定目录加载密钥对
// 优先从 keypair.json 加载，如果不存在则分别加载私钥和公钥文件
func LoadKeyPair(dir string) (privateKey, publicKey []byte, err error) {
	// 优先尝试从 keypair.json 加载
	keyPairPath := filepath.Join(dir, "keypair.json")
	data, err := ioutil.ReadFile(keyPairPath)
	if err == nil {
		var keyPair KeyPair
		if jsonErr := json.Unmarshal(data, &keyPair); jsonErr == nil {
			priv, err := StringToPrivateKey(keyPair.PrivateKey)
			if err != nil {
				return nil, nil, fmt.Errorf("解析私钥失败: %w", err)
			}
			pub, err := StringToPublicKey(keyPair.PublicKey)
			if err != nil {
				return nil, nil, fmt.Errorf("解析公钥失败: %w", err)
			}
			return priv, pub, nil
		}
	}

	// 回退：分别加载私钥和公钥
	privPath := filepath.Join(dir, "private_key.json")
	privData, err := ioutil.ReadFile(privPath)
	if err != nil {
		return nil, nil, fmt.Errorf("读取私钥文件失败: %w", err)
	}

	var privMap map[string]string
	if err := json.Unmarshal(privData, &privMap); err != nil {
		return nil, nil, fmt.Errorf("解析私钥文件失败: %w", err)
	}

	privateKey, err = StringToPrivateKey(privMap["private_key"])
	if err != nil {
		return nil, nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	pubPath := filepath.Join(dir, "public_key.json")
	pubData, err := ioutil.ReadFile(pubPath)
	if err != nil {
		return nil, nil, fmt.Errorf("读取公钥文件失败: %w", err)
	}

	var pubMap map[string]string
	if err := json.Unmarshal(pubData, &pubMap); err != nil {
		return nil, nil, fmt.Errorf("解析公钥文件失败: %w", err)
	}

	publicKey, err = StringToPublicKey(pubMap["public_key"])
	if err != nil {
		return nil, nil, fmt.Errorf("解析公钥失败: %w", err)
	}

	return privateKey, publicKey, nil
}
