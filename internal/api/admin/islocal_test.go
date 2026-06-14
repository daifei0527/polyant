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
		// 连接级（主判断，难以伪造）
		{"loopback v4 with port", "127.0.0.1:54321", "anything", "127.0.0.1:18531", true},
		{"loopback v6", "[::1]:54321", "anything", "127.0.0.1:18531", true},
		{"loopback with custom configured port", "127.0.0.1:12345", "anything", "127.0.0.1:9999", true},
		{"external remote rejected", "10.0.0.5:12345", "evil.com", "127.0.0.1:18531", false},
		// Host 头辅助（端口取自配置，不再硬编码 18531）
		{"external remote + host matches configured local", "10.0.0.5:12345", "127.0.0.1:9999", "127.0.0.1:9999", true},
		{"external remote + localhost alias on configured port", "10.0.0.5:12345", "localhost:9999", "127.0.0.1:9999", true},
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
