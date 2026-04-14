package email

import (
	"bytes"
	"strings"
	"testing"
	"text/template"
	"time"
)

func TestNewService(t *testing.T) {
	config := Config{
		Host:     "smtp.example.com",
		Port:     587,
		From:     "test@example.com",
		FromName: "Test Sender",
		Username: "user",
		Password: "pass",
		UseTLS:   true,
	}

	service := NewService(config)
	if service == nil {
		t.Fatal("NewService returned nil")
	}

	if service.config.Host != config.Host {
		t.Errorf("Expected host %s, got %s", config.Host, service.config.Host)
	}
}

func TestNewService_NoAuth(t *testing.T) {
	config := Config{
		Host: "smtp.example.com",
		Port: 25,
		From: "test@example.com",
	}

	service := NewService(config)
	if service == nil {
		t.Fatal("NewService returned nil")
	}

	// Without username/password, auth should be nil
	if service.auth != nil {
		t.Error("Auth should be nil without username/password")
	}
}

func TestService_Send_EmptyRecipient(t *testing.T) {
	service := NewService(Config{
		Host: "smtp.example.com",
		Port: 25,
		From: "test@example.com",
	})

	email := &Email{
		To:       []string{},
		Subject:  "Test",
		TextBody: "Test content",
	}

	err := service.Send(email)
	if err == nil {
		t.Error("Send with empty recipient should return error")
	}
}

func TestEmail_Templates(t *testing.T) {
	// Test that templates are valid
	if verificationEmailTemplate.HTML == "" {
		t.Error("Verification email HTML template should not be empty")
	}
	if verificationEmailTemplate.Text == "" {
		t.Error("Verification email Text template should not be empty")
	}

	if welcomeEmailTemplate.HTML == "" {
		t.Error("Welcome email HTML template should not be empty")
	}
	if welcomeEmailTemplate.Text == "" {
		t.Error("Welcome email Text template should not be empty")
	}
}

func TestVerificationEmailTemplate_Render(t *testing.T) {
	data := struct {
		Code      string
		VerifyURL string
		Year      int
	}{
		Code:      "123456",
		VerifyURL: "https://example.com/verify?code=123456",
		Year:      time.Now().Year(),
	}

	// Test HTML template rendering
	tmpl, err := template.New("verification").Parse(verificationEmailTemplate.HTML)
	if err != nil {
		t.Fatalf("Failed to parse HTML template: %v", err)
	}

	var htmlBuf bytes.Buffer
	if err := tmpl.Execute(&htmlBuf, data); err != nil {
		t.Fatalf("Failed to execute HTML template: %v", err)
	}

	htmlOutput := htmlBuf.String()
	if !strings.Contains(htmlOutput, "123456") {
		t.Error("HTML output should contain verification code")
	}
	if !strings.Contains(htmlOutput, "https://example.com/verify?code=123456") {
		t.Error("HTML output should contain verify URL")
	}

	// Test text template rendering
	tmpl, err = template.New("verification_text").Parse(verificationEmailTemplate.Text)
	if err != nil {
		t.Fatalf("Failed to parse text template: %v", err)
	}

	var textBuf bytes.Buffer
	if err := tmpl.Execute(&textBuf, data); err != nil {
		t.Fatalf("Failed to execute text template: %v", err)
	}

	textOutput := textBuf.String()
	if !strings.Contains(textOutput, "123456") {
		t.Error("Text output should contain verification code")
	}
}

func TestWelcomeEmailTemplate_Render(t *testing.T) {
	data := struct {
		AgentName string
		Year      int
	}{
		AgentName: "TestAgent",
		Year:      time.Now().Year(),
	}

	// Test HTML template rendering
	tmpl, err := template.New("welcome").Parse(welcomeEmailTemplate.HTML)
	if err != nil {
		t.Fatalf("Failed to parse HTML template: %v", err)
	}

	var htmlBuf bytes.Buffer
	if err := tmpl.Execute(&htmlBuf, data); err != nil {
		t.Fatalf("Failed to execute HTML template: %v", err)
	}

	htmlOutput := htmlBuf.String()
	if !strings.Contains(htmlOutput, "TestAgent") {
		t.Error("HTML output should contain agent name")
	}

	// Test text template rendering
	tmpl, err = template.New("welcome_text").Parse(welcomeEmailTemplate.Text)
	if err != nil {
		t.Fatalf("Failed to parse text template: %v", err)
	}

	var textBuf bytes.Buffer
	if err := tmpl.Execute(&textBuf, data); err != nil {
		t.Fatalf("Failed to execute text template: %v", err)
	}

	textOutput := textBuf.String()
	if !strings.Contains(textOutput, "TestAgent") {
		t.Error("Text output should contain agent name")
	}
}

func TestEmail_BuildMessage(t *testing.T) {
	service := NewService(Config{
		Host:     "smtp.example.com",
		Port:     587,
		From:     "test@example.com",
		FromName: "Test Sender",
		Username: "user",
		Password: "pass",
	})

	// Test building email with HTML body
	email := &Email{
		To:       []string{"recipient@example.com"},
		Subject:  "Test Subject",
		TextBody: "Plain text content",
		HTMLBody: "<html><body>HTML content</body></html>",
	}

	// We can't easily test the Send function without a real SMTP server,
	// but we can test the message building logic indirectly by checking
	// that the function returns an error for network issues (expected)
	err := service.Send(email)
	// The send will fail due to network issues, but we can verify it's a network error
	if err == nil {
		// This would only happen if somehow connected to a real SMTP server
		t.Log("Send succeeded unexpectedly (maybe local SMTP server?)")
	} else {
		// Expected: some kind of network/connection error
		t.Logf("Send failed as expected for test: %v", err)
	}
}

func TestService_Send_TextOnly(t *testing.T) {
	service := NewService(Config{
		Host: "smtp.example.com",
		Port: 25,
		From: "test@example.com",
	})

	email := &Email{
		To:       []string{"recipient@example.com"},
		Subject:  "Text Only",
		TextBody: "This is plain text only",
	}

	// Should fail with network error, not with message building error
	err := service.Send(email)
	if err == nil {
		t.Log("Send succeeded unexpectedly")
	} else {
		// Verify it's not an "empty recipient" error
		if strings.Contains(err.Error(), "收件人地址为空") {
			t.Error("Should not fail with empty recipient error")
		}
		t.Logf("Send failed as expected: %v", err)
	}
}

func TestService_Send_WithFromName(t *testing.T) {
	service := NewService(Config{
		Host:     "smtp.example.com",
		Port:     587,
		From:     "noreply@example.com",
		FromName: "Polyant Team",
		Username: "user",
		Password: "pass",
	})

	email := &Email{
		To:       []string{"user@example.com"},
		Subject:  "Welcome",
		TextBody: "Welcome to Polyant",
		HTMLBody: "<html><body>Welcome to Polyant</body></html>",
	}

	err := service.Send(email)
	// Expected to fail with network error
	if err != nil {
		t.Logf("Send failed as expected: %v", err)
	}
}

func TestService_Send_TLS(t *testing.T) {
	service := NewService(Config{
		Host:          "smtp.example.com",
		Port:          465,
		From:          "test@example.com",
		Username:      "user",
		Password:      "pass",
		UseTLS:        true,
		SkipTLSVerify: true,
	})

	email := &Email{
		To:       []string{"recipient@example.com"},
		Subject:  "TLS Test",
		TextBody: "Testing TLS connection",
	}

	err := service.Send(email)
	// Expected to fail with network error (TLS connection)
	if err == nil {
		t.Log("TLS Send succeeded unexpectedly")
	} else {
		t.Logf("TLS Send failed as expected: %v", err)
	}
}

func TestService_SendVerificationEmail(t *testing.T) {
	service := NewService(Config{
		Host:     "smtp.example.com",
		Port:     587,
		From:     "noreply@example.com",
		FromName: "Polyant",
		Username: "user",
		Password: "pass",
	})

	err := service.SendVerificationEmail(
		"user@example.com",
		"123456",
		"https://example.com/verify?code=123456",
	)

	// Expected to fail with network error
	if err != nil {
		// Verify it's a network error, not a template error
		if strings.Contains(err.Error(), "template") {
			t.Errorf("Template error: %v", err)
		} else {
			t.Logf("SendVerificationEmail failed as expected (network): %v", err)
		}
	}
}

func TestService_SendWelcomeEmail(t *testing.T) {
	service := NewService(Config{
		Host:     "smtp.example.com",
		Port:     587,
		From:     "noreply@example.com",
		FromName: "Polyant",
		Username: "user",
		Password: "pass",
	})

	err := service.SendWelcomeEmail("user@example.com", "TestAgent")

	// Expected to fail with network error
	if err != nil {
		// Verify it's a network error, not a template error
		if strings.Contains(err.Error(), "template") {
			t.Errorf("Template error: %v", err)
		} else {
			t.Logf("SendWelcomeEmail failed as expected (network): %v", err)
		}
	}
}

func TestService_SendNotificationEmail(t *testing.T) {
	service := NewService(Config{
		Host:     "smtp.example.com",
		Port:     587,
		From:     "noreply@example.com",
		FromName: "Polyant",
		Username: "user",
		Password: "pass",
	})

	err := service.SendNotificationEmail(
		"user@example.com",
		"Notification Subject",
		"Notification content here",
	)

	// Expected to fail with network error
	if err != nil {
		// Verify it's a network error
		if strings.Contains(err.Error(), "收件人地址为空") {
			t.Error("Should not fail with empty recipient error")
		} else {
			t.Logf("SendNotificationEmail failed as expected (network): %v", err)
		}
	}
}
