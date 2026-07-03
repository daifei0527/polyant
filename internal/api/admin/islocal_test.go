package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsLocalRequest(t *testing.T) {
	cases := []struct {
		name      string
		remote    string
		host      string
		localHost string
		want      bool
	}{
		// 连接级（唯一信任源，难以伪造）。Host 头永不信任（可由客户端伪造）。
		{"loopback v4 with port", "127.0.0.1:54321", "anything", "127.0.0.1:18531", true},
		{"loopback v6", "[::1]:54321", "anything", "127.0.0.1:18531", true},
		{"loopback with custom configured port", "127.0.0.1:12345", "anything", "127.0.0.1:9999", true},
		{"external remote rejected", "10.0.0.5:12345", "evil.com", "127.0.0.1:18531", false},
		// Host 头不再授予本地访问权（安全修复：此前可被远程攻击者伪造绕过）
		{"external remote + host matches configured local (Host NOT trusted)", "10.0.0.5:12345", "127.0.0.1:9999", "127.0.0.1:9999", false},
		{"external remote + localhost alias on configured port (Host NOT trusted)", "10.0.0.5:12345", "localhost:9999", "127.0.0.1:9999", false},
		{"external remote + host on wrong port", "10.0.0.5:12345", "127.0.0.1:18531", "127.0.0.1:9999", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = c.host
			req.RemoteAddr = c.remote
			if got := isLocalRequest(req, c.localHost); got != c.want {
				t.Errorf("isLocalRequest(remote=%q, host=%q, localHost=%q) = %v, want %v",
					c.remote, c.host, c.localHost, got, c.want)
			}
		})
	}
}
