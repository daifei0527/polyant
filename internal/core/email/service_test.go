package email

import (
	"testing"
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
