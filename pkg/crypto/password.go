package crypto

import "golang.org/x/crypto/bcrypt"

// bcryptCost 是 bcrypt 的计算成本因子。12 在 2026 年的硬件上约 ~100ms/次，
// 对低频的管理员登录足够安全且不致明显延迟。
const bcryptCost = 12

// HashPassword 用 bcrypt 哈希明文密码（带随机盐）。
// 用于 Web admin 登录的密码存储。
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword 校验明文与 bcrypt 哈希是否匹配。
// hashed 为空（用户未设密码）一律返回 false——未设密码的账户不能通过密码登录。
// 使用 bcrypt 的恒定时间比较，抗时序侧信道。
func CheckPassword(hashed, plain string) bool {
	if hashed == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
