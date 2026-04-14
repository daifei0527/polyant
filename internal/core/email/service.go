// Package email 提供邮件发送服务
package email

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
	"time"
)

// Config 邮件服务配置
type Config struct {
	// SMTP服务器地址
	Host string `yaml:"host"`
	// SMTP服务器端口
	Port int `yaml:"port"`
	// 发件人邮箱
	From string `yaml:"from"`
	// 发件人名称
	FromName string `yaml:"from_name"`
	// SMTP用户名
	Username string `yaml:"username"`
	// SMTP密码
	Password string `yaml:"password"`
	// 是否使用TLS
	UseTLS bool `yaml:"use_tls"`
	// 是否跳过TLS证书验证（仅用于测试）
	SkipTLSVerify bool `yaml:"skip_tls_verify"`
}

// Service 邮件服务
type Service struct {
	config Config
	auth   smtp.Auth
}

// NewService 创建邮件服务
func NewService(config Config) *Service {
	var auth smtp.Auth
	if config.Username != "" && config.Password != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.Host)
	}
	
	return &Service{
		config: config,
		auth:   auth,
	}
}

// Email 邮件内容
type Email struct {
	To       []string
	Subject  string
	TextBody string
	HTMLBody string
}

// Send 发送邮件
func (s *Service) Send(email *Email) error {
	if len(email.To) == 0 {
		return fmt.Errorf("收件人地址为空")
	}
	
	// 构建邮件头
	var msg bytes.Buffer
	
	// 发件人
	fromAddr := s.config.From
	if s.config.FromName != "" {
		fromAddr = fmt.Sprintf("%s <%s>", s.config.FromName, s.config.From)
	}
	msg.WriteString(fmt.Sprintf("From: %s\r\n", fromAddr))
	
	// 收件人
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(email.To, ", ")))
	
	// 主题
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", email.Subject))
	
	// 日期
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	
	// MIME版本
	msg.WriteString("MIME-Version: 1.0\r\n")
	
	// 内容类型
	if email.HTMLBody != "" {
		// 混合内容（HTML + 纯文本）
		boundary := fmt.Sprintf("boundary_%d", time.Now().UnixNano())
		msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
		msg.WriteString("\r\n")
		
		// 纯文本部分
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		msg.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		msg.WriteString(email.TextBody)
		msg.WriteString("\r\n\r\n")
		
		// HTML部分
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		msg.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		msg.WriteString(email.HTMLBody)
		msg.WriteString("\r\n\r\n")
		
		// 结束边界
		msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		// 仅纯文本
		msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		msg.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		msg.WriteString(email.TextBody)
	}
	
	// 发送邮件
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	
	if s.config.UseTLS {
		return s.sendWithTLS(addr, email.To, msg.Bytes())
	}
	
	return smtp.SendMail(addr, s.auth, s.config.From, email.To, msg.Bytes())
}

// sendWithTLS 使用TLS发送邮件
func (s *Service) sendWithTLS(addr string, to []string, msg []byte) error {
	// TLS配置
	tlsConfig := &tls.Config{
		InsecureSkipVerify: s.config.SkipTLSVerify,
		ServerName:         s.config.Host,
	}
	
	// 连接服务器
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS连接失败: %w", err)
	}
	
	// 创建SMTP客户端
	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %w", err)
	}
	defer client.Close()
	
	// 认证
	if s.auth != nil {
		if err := client.Auth(s.auth); err != nil {
			return fmt.Errorf("SMTP认证失败: %w", err)
		}
	}
	
	// 设置发件人
	if err := client.Mail(s.config.From); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}
	
	// 设置收件人
	for _, t := range to {
		if err := client.Rcpt(t); err != nil {
			return fmt.Errorf("设置收件人失败: %w", err)
		}
	}
	
	// 发送内容
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("准备邮件内容失败: %w", err)
	}
	
	_, err = writer.Write(msg)
	if err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}
	
	if err := writer.Close(); err != nil {
		return fmt.Errorf("发送邮件失败: %w", err)
	}
	
	return client.Quit()
}

// SendVerificationEmail 发送验证邮件
func (s *Service) SendVerificationEmail(to, code, verifyURL string) error {
	tmpl := verificationEmailTemplate
	
	data := struct {
		Code      string
		VerifyURL string
		Year      int
	}{
		Code:      code,
		VerifyURL: verifyURL,
		Year:      time.Now().Year(),
	}
	
	var htmlBuf, textBuf bytes.Buffer
	
	t, err := template.New("verification").Parse(tmpl.HTML)
	if err != nil {
		return err
	}
	if err := t.Execute(&htmlBuf, data); err != nil {
		return err
	}
	
	t, err = template.New("verification_text").Parse(tmpl.Text)
	if err != nil {
		return err
	}
	if err := t.Execute(&textBuf, data); err != nil {
		return err
	}
	
	return s.Send(&Email{
		To:       []string{to},
		Subject:  "Polyant - 邮箱验证",
		TextBody: textBuf.String(),
		HTMLBody: htmlBuf.String(),
	})
}

// SendWelcomeEmail 发送欢迎邮件
func (s *Service) SendWelcomeEmail(to, agentName string) error {
	data := struct {
		AgentName string
		Year      int
	}{
		AgentName: agentName,
		Year:      time.Now().Year(),
	}
	
	var htmlBuf, textBuf bytes.Buffer
	
	t, err := template.New("welcome").Parse(welcomeEmailTemplate.HTML)
	if err != nil {
		return err
	}
	if err := t.Execute(&htmlBuf, data); err != nil {
		return err
	}
	
	t, err = template.New("welcome_text").Parse(welcomeEmailTemplate.Text)
	if err != nil {
		return err
	}
	if err := t.Execute(&textBuf, data); err != nil {
		return err
	}
	
	return s.Send(&Email{
		To:       []string{to},
		Subject:  "欢迎加入 Polyant",
		TextBody: textBuf.String(),
		HTMLBody: htmlBuf.String(),
	})
}

// SendNotificationEmail 发送通知邮件
func (s *Service) SendNotificationEmail(to, subject, content string) error {
	return s.Send(&Email{
		To:       []string{to},
		Subject:  subject,
		TextBody: content,
	})
}

// ==================== 邮件模板 ====================

type emailTemplate struct {
	HTML string
	Text string
}

var verificationEmailTemplate = emailTemplate{
	HTML: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .code { font-size: 32px; font-weight: bold; color: #667eea; letter-spacing: 8px; padding: 20px; background: white; border-radius: 8px; text-align: center; margin: 20px 0; }
        .button { display: inline-block; padding: 12px 30px; background: #667eea; color: white; text-decoration: none; border-radius: 5px; }
        .footer { text-align: center; color: #999; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔐 邮箱验证</h1>
        </div>
        <div class="content">
            <p>您好！</p>
            <p>感谢您注册 Polyant。请使用以下验证码完成邮箱验证：</p>
            <div class="code">{{.Code}}</div>
            <p>或者点击下方按钮直接验证：</p>
            <p style="text-align: center;">
                <a href="{{.VerifyURL}}" class="button">立即验证</a>
            </p>
            <p>验证码有效期为 30 分钟，请尽快完成验证。</p>
            <p>如果您没有注册 Polyant 账户，请忽略此邮件。</p>
        </div>
        <div class="footer">
            <p>© {{.Year}} Polyant. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`,
	Text: `Polyant 邮箱验证

您好！

感谢您注册 Polyant。请使用以下验证码完成邮箱验证：

验证码: {{.Code}}

或访问以下链接验证：
{{.VerifyURL}}

验证码有效期为 30 分钟，请尽快完成验证。

如果您没有注册 Polyant 账户，请忽略此邮件。

© {{.Year}} Polyant. All rights reserved.`,
}

var welcomeEmailTemplate = emailTemplate{
	HTML: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #11998e 0%, #38ef7d 100%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .footer { text-align: center; color: #999; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🎉 欢迎加入 Polyant</h1>
        </div>
        <div class="content">
            <p>亲爱的 {{.AgentName}}，</p>
            <p>欢迎加入 Polyant 社区！</p>
            <p>Polyant 是一个去中心化的知识库网络，您可以：</p>
            <ul>
                <li>📝 贡献您的知识，获得社区认可</li>
                <li>🔍 搜索海量知识，获取精准答案</li>
                <li>⭐ 对知识进行评分，帮助提升质量</li>
                <li>🌐 节点同步，参与去中心化网络</li>
            </ul>
            <p>开始您的知识之旅吧！</p>
        </div>
        <div class="footer">
            <p>© {{.Year}} Polyant. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`,
	Text: `欢迎加入 Polyant

亲爱的 {{.AgentName}}，

欢迎加入 Polyant 社区！

Polyant 是一个去中心化的知识库网络，您可以：
- 贡献您的知识，获得社区认可
- 搜索海量知识，获取精准答案
- 对知识进行评分，帮助提升质量
- 节点同步，参与去中心化网络

开始您的知识之旅吧！

© {{.Year}} Polyant. All rights reserved.`,
}
