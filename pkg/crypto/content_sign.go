// Package crypto: 内容签名原语。
// 条目内容签名契约与 model.KnowledgeEntry.ComputeContentHash 一致：
// SHA256(title + "\n" + content + "\n" + category)。这里签名的是该 SHA256 摘要。
package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
)

// contentDigest 计算条目内容摘要 = SHA256(title\ncontent\ncategory)，与 model 契约一致。
// 在 pkg/crypto 内本地实现以避免 import internal/storage/model（会循环）。
func contentDigest(title, content, category string) []byte {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s\n%s\n%s", title, content, category)))
	return h[:]
}

// ratingDigest 计算评分摘要 = SHA256(entryID\nraterPub\nscore)，score 用十进制浮点规范化。
func ratingDigest(entryID, raterPub string, score float64) []byte {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s\n%s\n%s", entryID, raterPub, strconv.FormatFloat(score, 'f', -1, 64))))
	return h[:]
}

// SignContent 用创建者私钥对条目内容摘要签名。
func SignContent(priv ed25519.PrivateKey, title, content, category string) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid ed25519 private key size")
	}
	return ed25519.Sign(priv, contentDigest(title, content, category)), nil
}

// VerifyContent 校验条目内容签名。任意输入非法（公钥/签名长度不对）均返回 false，不 panic。
func VerifyContent(pub ed25519.PublicKey, sig []byte, title, content, category string) bool {
	if len(pub) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(pub, contentDigest(title, content, category), sig)
}

// SignRating 用评分者私钥对评分内容摘要签名。
func SignRating(priv ed25519.PrivateKey, entryID, raterPub string, score float64) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid ed25519 private key size")
	}
	return ed25519.Sign(priv, ratingDigest(entryID, raterPub, score)), nil
}

// VerifyRating 校验评分签名。
func VerifyRating(pub ed25519.PublicKey, sig []byte, entryID, raterPub string, score float64) bool {
	if len(pub) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(pub, ratingDigest(entryID, raterPub, score), sig)
}
