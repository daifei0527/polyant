// Package ed25519_test 提供密钥管理功能的单元测试
package ed25519_test

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"

	authed25519 "github.com/daifei0527/agentwiki/internal/auth/ed25519"
)

// TestGenerateKeyPair 测试密钥对生成
func TestGenerateKeyPair(t *testing.T) {
	pub, priv, err := authed25519.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	// 验证密钥长度
	if len(pub) != ed25519.PublicKeySize {
		t.Errorf("公钥长度错误: 期望 %d, 实际 %d", ed25519.PublicKeySize, len(pub))
	}

	if len(priv) != ed25519.PrivateKeySize {
		t.Errorf("私钥长度错误: 期望 %d, 实际 %d", ed25519.PrivateKeySize, len(priv))
	}

	// 多次生成应产生不同的密钥
	pub2, priv2, err := authed25519.GenerateKeyPair()
	if err != nil {
		t.Fatalf("第二次生成密钥对失败: %v", err)
	}

	if string(pub) == string(pub2) {
		t.Error("两次生成的公钥不应相同")
	}
	if string(priv) == string(priv2) {
		t.Error("两次生成的私钥不应相同")
	}
}

// TestSignAndVerify 测试签名和验证
func TestSignAndVerify(t *testing.T) {
	pub, priv, err := authed25519.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	message := []byte("Hello, AgentWiki!")

	// 签名
	signature, err := authed25519.Sign(priv, message)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	if len(signature) != ed25519.SignatureSize {
		t.Errorf("签名长度错误: 期望 %d, 实际 %d", ed25519.SignatureSize, len(signature))
	}

	// 验证正确签名
	if !authed25519.Verify(pub, message, signature) {
		t.Error("有效签名验证失败")
	}

	// 验证错误消息
	if authed25519.Verify(pub, []byte("Wrong message"), signature) {
		t.Error("错误消息不应验证通过")
	}

	// 验证错误签名
	wrongSig := make([]byte, len(signature))
	copy(wrongSig, signature)
	wrongSig[0] ^= 0xFF
	if authed25519.Verify(pub, message, wrongSig) {
		t.Error("错误签名不应验证通过")
	}
}

// TestSignWithInvalidKey 测试使用无效私钥签名
func TestSignWithInvalidKey(t *testing.T) {
	invalidPriv := make([]byte, 16) // 长度不对
	message := []byte("test")

	_, err := authed25519.Sign(invalidPriv, message)
	if err == nil {
		t.Error("使用无效私钥签名应返回错误")
	}
}

// TestVerifyWithInvalidKey 测试使用无效公钥验证
func TestVerifyWithInvalidKey(t *testing.T) {
	invalidPub := make([]byte, 16) // 长度不对
	message := []byte("test")
	signature := make([]byte, ed25519.SignatureSize)

	if authed25519.Verify(invalidPub, message, signature) {
		t.Error("使用无效公钥验证应返回 false")
	}
}

// TestPublicKeyStringConversion 测试公钥字符串转换
func TestPublicKeyStringConversion(t *testing.T) {
	pub, _, err := authed25519.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	// 编码
	pubStr := authed25519.PublicKeyToString(pub)
	if pubStr == "" {
		t.Fatal("公钥编码结果为空")
	}

	// 解码
	decoded, err := authed25519.StringToPublicKey(pubStr)
	if err != nil {
		t.Fatalf("公钥解码失败: %v", err)
	}

	// 验证一致性
	if string(pub) != string(decoded) {
		t.Error("编码解码后的公钥不一致")
	}
}

// TestPrivateKeyStringConversion 测试私钥字符串转换
func TestPrivateKeyStringConversion(t *testing.T) {
	_, priv, err := authed25519.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	// 编码
	privStr := authed25519.PrivateKeyToString(priv)
	if privStr == "" {
		t.Fatal("私钥编码结果为空")
	}

	// 解码
	decoded, err := authed25519.StringToPrivateKey(privStr)
	if err != nil {
		t.Fatalf("私钥解码失败: %v", err)
	}

	// 验证一致性
	if string(priv) != string(decoded) {
		t.Error("编码解码后的私钥不一致")
	}
}

// TestStringToPublicKeyInvalid 测试无效公钥字符串解码
func TestStringToPublicKeyInvalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "无效 Base64",
			input:   "not-valid-base64!!!",
			wantErr: true,
		},
		{
			name:    "长度不足",
			input:   "YQ==", // "a" 的 base64
			wantErr: true,
		},
		{
			name:    "空字符串",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := authed25519.StringToPublicKey(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("StringToPublicKey(%q) 错误 = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

// TestStringToPrivateKeyInvalid 测试无效私钥字符串解码
func TestStringToPrivateKeyInvalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "无效 Base64",
			input:   "not-valid-base64!!!",
			wantErr: true,
		},
		{
			name:    "长度不足",
			input:   "YQ==",
			wantErr: true,
		},
		{
			name:    "空字符串",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := authed25519.StringToPrivateKey(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("StringToPrivateKey(%q) 错误 = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

// TestSaveAndLoadKeyPair 测试密钥对保存和加载
func TestSaveAndLoadKeyPair(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "agentwiki-keys-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 生成密钥对
	pub, priv, err := authed25519.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	// 保存密钥对
	if err := authed25519.SaveKeyPair(priv, pub, tmpDir); err != nil {
		t.Fatalf("保存密钥对失败: %v", err)
	}

	// 验证文件存在
	files := []string{"keypair.json", "private_key.json", "public_key.json"}
	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("文件 %s 未创建", f)
		}
	}

	// 加载密钥对
	loadedPriv, loadedPub, err := authed25519.LoadKeyPair(tmpDir)
	if err != nil {
		t.Fatalf("加载密钥对失败: %v", err)
	}

	// 验证一致性
	if string(priv) != string(loadedPriv) {
		t.Error("加载的私钥不一致")
	}
	if string(pub) != string(loadedPub) {
		t.Error("加载的公钥不一致")
	}
}

// TestLoadKeyPairFromSeparateFiles 测试从单独文件加载密钥对
func TestLoadKeyPairFromSeparateFiles(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "agentwiki-keys-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 生成密钥对
	pub, priv, err := authed25519.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	// 只保存单独文件（不保存 keypair.json）
	pubPath := filepath.Join(tmpDir, "public_key.json")
	privPath := filepath.Join(tmpDir, "private_key.json")

	pubData := `{"public_key": "` + authed25519.PublicKeyToString(pub) + `"}`
	privData := `{"private_key": "` + authed25519.PrivateKeyToString(priv) + `"}`

	if err := os.WriteFile(pubPath, []byte(pubData), 0644); err != nil {
		t.Fatalf("写入公钥文件失败: %v", err)
	}
	if err := os.WriteFile(privPath, []byte(privData), 0600); err != nil {
		t.Fatalf("写入私钥文件失败: %v", err)
	}

	// 加载密钥对
	loadedPriv, loadedPub, err := authed25519.LoadKeyPair(tmpDir)
	if err != nil {
		t.Fatalf("加载密钥对失败: %v", err)
	}

	// 验证一致性
	if string(priv) != string(loadedPriv) {
		t.Error("加载的私钥不一致")
	}
	if string(pub) != string(loadedPub) {
		t.Error("加载的公钥不一致")
	}
}

// TestLoadKeyPairNotFound 测试加载不存在的密钥对
func TestLoadKeyPairNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agentwiki-keys-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, _, err = authed25519.LoadKeyPair(tmpDir)
	if err == nil {
		t.Error("加载不存在的密钥对应返回错误")
	}
}

// TestSignVerifyRoundTrip 测试完整的签名验证流程
func TestSignVerifyRoundTrip(t *testing.T) {
	// 生成密钥对
	pub, priv, err := authed25519.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	// 编码为字符串
	pubStr := authed25519.PublicKeyToString(pub)
	privStr := authed25519.PrivateKeyToString(priv)

	// 解码
	decodedPub, err := authed25519.StringToPublicKey(pubStr)
	if err != nil {
		t.Fatalf("解码公钥失败: %v", err)
	}
	decodedPriv, err := authed25519.StringToPrivateKey(privStr)
	if err != nil {
		t.Fatalf("解码私钥失败: %v", err)
	}

	// 使用解码后的密钥签名验证
	message := []byte("Test message for round trip")
	signature, err := authed25519.Sign(decodedPriv, message)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	if !authed25519.Verify(decodedPub, message, signature) {
		t.Error("使用编解码后的密钥签名验证失败")
	}
}
