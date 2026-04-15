// Package crypto_test 提供加密工具包的单元测试
package crypto_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daifei0527/polyant/pkg/crypto"
)

// ==================== SHA256 测试 ====================

// TestComputeSHA256 测试 SHA256 计算
func TestComputeSHA256(t *testing.T) {
	tests := []struct {
		input    string
		expected string // 预计算的 SHA256 值
	}{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"hello", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
		{"Hello, Polyant!", "a1b2c3d4e5f6..."}, // 只检查长度和格式
	}

	for _, tc := range tests {
		result := crypto.ComputeSHA256([]byte(tc.input))

		// 验证长度 (SHA256 = 32 字节 = 64 十六进制字符)
		if len(result) != 64 {
			t.Errorf("ComputeSHA256(%q) 长度 = %d, want 64", tc.input, len(result))
		}

		// 验证十六进制格式
		for _, c := range result {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("ComputeSHA256(%q) 包含非十六进制字符: %c", tc.input, c)
				break
			}
		}
	}

	// 特定测试用例
	if crypto.ComputeSHA256([]byte("")) != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Error("空字符串 SHA256 计算错误")
	}

	if crypto.ComputeSHA256([]byte("hello")) != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Error("'hello' SHA256 计算错误")
	}
}

// TestComputeSHA256Deterministic 测试 SHA256 确定性
func TestComputeSHA256Deterministic(t *testing.T) {
	input := []byte("test data for determinism")

	hash1 := crypto.ComputeSHA256(input)
	hash2 := crypto.ComputeSHA256(input)

	if hash1 != hash2 {
		t.Error("相同输入应产生相同哈希值")
	}
}

// TestComputeSHA256Unique 测试不同输入产生不同哈希
func TestComputeSHA256Unique(t *testing.T) {
	inputs := []string{"a", "b", "c", "1", "2", "3"}
	hashes := make(map[string]bool)

	for _, input := range inputs {
		hash := crypto.ComputeSHA256([]byte(input))
		if hashes[hash] {
			t.Errorf("不同输入 '%s' 产生了重复哈希", input)
		}
		hashes[hash] = true
	}
}

// ==================== VerifySHA256 测试 ====================

// TestVerifySHA256 测试 SHA256 验证
func TestVerifySHA256(t *testing.T) {
	data := []byte("test data")
	hash := crypto.ComputeSHA256(data)

	if !crypto.VerifySHA256(data, hash) {
		t.Error("验证正确的哈希应返回 true")
	}

	// 错误的哈希
	if crypto.VerifySHA256(data, "wronghash") {
		t.Error("验证错误的哈希应返回 false")
	}

	// 不同的数据
	if crypto.VerifySHA256([]byte("different data"), hash) {
		t.Error("验证不同数据应返回 false")
	}
}

// TestVerifySHA256Empty 测试空数据验证
func TestVerifySHA256Empty(t *testing.T) {
	emptyHash := crypto.ComputeSHA256([]byte{})

	if !crypto.VerifySHA256([]byte{}, emptyHash) {
		t.Error("空数据验证应成功")
	}
}

// ==================== GenerateUUID 测试 ====================

// TestGenerateUUID 测试 UUID 生成
func TestGenerateUUID(t *testing.T) {
	uuid := crypto.GenerateUUID()

	// 验证格式: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	if len(uuid) != 36 {
		t.Errorf("UUID 长度 = %d, want 36", len(uuid))
	}

	// 验证连字符位置
	if uuid[8] != '-' || uuid[13] != '-' || uuid[18] != '-' || uuid[23] != '-' {
		t.Errorf("UUID 格式错误: %s", uuid)
	}

	// 验证十六进制字符
	for i, c := range uuid {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("UUID 包含无效字符: %c at position %d", c, i)
			break
		}
	}
}

// TestGenerateUUIDUnique 测试 UUID 唯一性
func TestGenerateUUIDUnique(t *testing.T) {
	uuids := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		uuid := crypto.GenerateUUID()
		if uuids[uuid] {
			t.Errorf("生成重复 UUID: %s", uuid)
		}
		uuids[uuid] = true
	}

	if len(uuids) != count {
		t.Errorf("生成了 %d 个 UUID, 期望 %d", len(uuids), count)
	}
}

// TestGenerateUUIDVersion4 测试 UUID v4 版本号
func TestGenerateUUIDVersion4(t *testing.T) {
	for i := 0; i < 100; i++ {
		uuid := crypto.GenerateUUID()

		// UUID v4 的第 14 个字符 (索引 14) 应该是 '4'
		// 格式: xxxxxxxx-xxxx-4xxx-...
		if uuid[14] != '4' {
			t.Errorf("UUID 不是 v4 版本: %s (版本字符: %c)", uuid, uuid[14])
		}

		// 变体位: 第 19 个字符应该是 8, 9, a, 或 b
		variantChar := uuid[19]
		if variantChar != '8' && variantChar != '9' && variantChar != 'a' && variantChar != 'b' {
			t.Errorf("UUID 变体位错误: %s (变体字符: %c)", uuid, variantChar)
		}
	}
}

// ==================== ComputeFileSHA256 测试 ====================

// TestComputeFileSHA256 测试文件哈希计算
func TestComputeFileSHA256(t *testing.T) {
	// 创建临时文件
	tmpDir, err := os.MkdirTemp("", "crypto-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("test file content for hashing")

	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	// 计算文件哈希
	fileHash, err := crypto.ComputeFileSHA256(testFile)
	if err != nil {
		t.Fatalf("ComputeFileSHA256 失败: %v", err)
	}

	// 验证哈希长度
	if len(fileHash) != 64 {
		t.Errorf("文件哈希长度 = %d, want 64", len(fileHash))
	}

	// 验证与内容哈希一致
	contentHash := crypto.ComputeSHA256(testContent)
	if fileHash != contentHash {
		t.Errorf("文件哈希 %q 与内容哈希 %q 不一致", fileHash, contentHash)
	}
}

// TestComputeFileSHA256NotFound 测试不存在的文件
func TestComputeFileSHA256NotFound(t *testing.T) {
	_, err := crypto.ComputeFileSHA256("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("计算不存在文件的哈希应返回错误")
	}
}

// TestComputeFileSHA256Empty 测试空文件
func TestComputeFileSHA256Empty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "crypto-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("创建空文件失败: %v", err)
	}

	hash, err := crypto.ComputeFileSHA256(emptyFile)
	if err != nil {
		t.Fatalf("ComputeFileSHA256 失败: %v", err)
	}

	// 空文件的 SHA256
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != expected {
		t.Errorf("空文件哈希 = %q, want %q", hash, expected)
	}
}

// ==================== 边界条件测试 ====================

// TestComputeSHA256LargeInput 测试大输入
func TestComputeSHA256LargeInput(t *testing.T) {
	// 1MB 数据
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	hash := crypto.ComputeSHA256(largeData)
	if len(hash) != 64 {
		t.Errorf("大输入哈希长度错误: %d", len(hash))
	}
}

// TestComputeSHA256Binary 测试二进制数据
func TestComputeSHA256Binary(t *testing.T) {
	// 包含所有字节值的数据
	binaryData := make([]byte, 256)
	for i := 0; i < 256; i++ {
		binaryData[i] = byte(i)
	}

	hash := crypto.ComputeSHA256(binaryData)
	if len(hash) != 64 {
		t.Errorf("二进制数据哈希长度错误: %d", len(hash))
	}
}

// TestComputeSHA256Unicode 测试 Unicode 数据
func TestComputeSHA256Unicode(t *testing.T) {
	unicodeData := []byte("中文测试 日本語 한국어 Emoji: 🎉🚀💻")

	hash := crypto.ComputeSHA256(unicodeData)
	if len(hash) != 64 {
		t.Errorf("Unicode 数据哈希长度错误: %d", len(hash))
	}

	// 验证一致性
	hash2 := crypto.ComputeSHA256(unicodeData)
	if hash != hash2 {
		t.Error("Unicode 数据哈希不一致")
	}
}

// TestVerifySHA256CaseInsensitive 测试验证的大小写处理
func TestVerifySHA256CaseInsensitive(t *testing.T) {
	data := []byte("test")
	lowerHash := crypto.ComputeSHA256(data)
	upperHash := strings.ToUpper(lowerHash)

	// 库返回小写哈希，验证应使用小写
	if !crypto.VerifySHA256(data, lowerHash) {
		t.Error("小写哈希验证应成功")
	}

	// 大写哈希应失败（因为 ComputeSHA256 返回小写）
	if crypto.VerifySHA256(data, upperHash) {
		t.Error("大写哈希验证不应成功")
	}
}

// TestVerifySHA256WrongLength 测试错误长度的哈希
func TestVerifySHA256WrongLength(t *testing.T) {
	data := []byte("test")

	// 太短的哈希
	if crypto.VerifySHA256(data, "abc") {
		t.Error("短哈希验证不应成功")
	}

	// 太长的哈希
	longHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855extra"
	if crypto.VerifySHA256(data, longHash) {
		t.Error("长哈希验证不应成功")
	}
}

// TestComputeFileSHA256ReadError 测试文件读取错误
func TestComputeFileSHA256ReadError(t *testing.T) {
	// 创建一个目录而不是文件
	tmpDir, err := os.MkdirTemp("", "crypto-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 尝试读取目录
	_, err = crypto.ComputeFileSHA256(tmpDir)
	if err == nil {
		t.Error("读取目录应返回错误")
	}
}

// TestGenerateUUIDConsistency 测试多次生成的一致性
func TestGenerateUUIDConsistency(t *testing.T) {
	uuid1 := crypto.GenerateUUID()
	uuid2 := crypto.GenerateUUID()

	// 两个 UUID 应该不同
	if uuid1 == uuid2 {
		t.Error("连续生成的 UUID 应该不同")
	}

	// 但格式应该一致
	if len(uuid1) != len(uuid2) {
		t.Error("UUID 长度应该一致")
	}

	// 版本号应该相同
	if uuid1[14] != uuid2[14] {
		t.Error("UUID 版本号应该一致")
	}
}

// TestComputeSHA256NilInput 测试 nil 输入
func TestComputeSHA256NilInput(t *testing.T) {
	hash := crypto.ComputeSHA256(nil)
	if hash != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("nil 输入应等于空输入的哈希, got %s", hash)
	}
}

// TestVerifySHA256EmptyHash 测试空哈希验证
func TestVerifySHA256EmptyHash(t *testing.T) {
	data := []byte("test")

	// 空哈希字符串
	if crypto.VerifySHA256(data, "") {
		t.Error("空哈希验证不应成功")
	}
}
