package email

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// TestBuildMessage_NoHeaderInjection: 收件人/主题含 CRLF 时不得注入额外邮件头（如 Bcc）。
func TestBuildMessage_NoHeaderInjection(t *testing.T) {
	service := NewService(Config{
		Host: "smtp.example.com", Port: 25, From: "from@example.com",
		FromName: "Team\r\nBcc: evil@inject.com", // 发件人名也尝试注入
	})

	email := &Email{
		To:       []string{"victim@example.com\r\nBcc: evil@inject.com"},
		Subject:  "Hi\r\nBcc: leaked@inject.com",
		TextBody: "body",
	}
	msg, err := service.buildMessage(email)
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}

	// 注入签名：CRLF 后接 "Bcc:" 才是真正的新头注入；仅出现 "Bcc:" 子串（被消毒压扁到同一行）不算注入。
	if bytes.Contains(msg, []byte("\r\nBcc:")) {
		t.Errorf("header injection succeeded — message contains a injected Bcc header line\n%s", msg)
	}
	// To 与 Subject 头各自必须只占一行（无嵌入 CRLF 制造新头）
	for _, line := range strings.Split(string(msg), "\r\n") {
		if strings.HasPrefix(line, "To: ") && (strings.Contains(line, "\n") || strings.Contains(strings.TrimPrefix(line, "To: "), "\r")) {
			t.Errorf("To header contains embedded CR/LF: %q", line)
		}
	}
}

// TestBuildMessage_Base64Body: 声明 base64 编码的正文必须真正 base64 编码，不得明文泄露。
func TestBuildMessage_Base64Body(t *testing.T) {
	service := NewService(Config{Host: "smtp.example.com", Port: 25, From: "from@example.com"})

	const plainMarker = "PLAIN_BODY_MARKER_42"
	const htmlMarker = "<html>HTML_MARKER_42</html>"
	email := &Email{
		To:       []string{"to@example.com"},
		Subject:  "S",
		TextBody: plainMarker,
		HTMLBody: htmlMarker,
	}
	msg, err := service.buildMessage(email)
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}

	// 明文标记不得直接出现在编码后的正文中
	if bytes.Contains(msg, []byte(plainMarker)) {
		t.Errorf("text body leaked as plaintext (not base64-encoded)\n%s", msg)
	}
	if bytes.Contains(msg, []byte("HTML_MARKER_42")) {
		t.Errorf("html body leaked as plaintext (not base64-encoded)\n%s", msg)
	}
	// 编码后的标记必须存在
	wantText := base64.StdEncoding.EncodeToString([]byte(plainMarker))
	if !bytes.Contains(msg, []byte(wantText)) {
		t.Errorf("text body not base64-encoded correctly; missing %s\n%s", wantText, msg)
	}
}

// TestBuildMessage_FromNameEncoded: 含非 ASCII 的发件人名用 mime 编码，且不破坏头结构。
func TestBuildMessage_FromNameEncoded(t *testing.T) {
	service := NewService(Config{Host: "smtp.example.com", Port: 25, From: "from@example.com", FromName: "Polyant 团队"})
	email := &Email{To: []string{"to@example.com"}, Subject: "S", TextBody: "b"}
	msg, err := service.buildMessage(email)
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}
	// From 头应存在且只占一行（无注入）
	fromLine := ""
	for _, line := range strings.Split(string(msg), "\r\n") {
		if strings.HasPrefix(line, "From: ") {
			fromLine = line
			break
		}
	}
	if fromLine == "" {
		t.Fatalf("From header missing\n%s", msg)
	}
	if strings.Contains(fromLine, "\n") || strings.Contains(fromLine, "\r") {
		t.Errorf("From header spans multiple lines (injection): %q", fromLine)
	}
}
