// Package mail 提供邮件发送能力。
// 通过 Mailer 接口抽象发送通道：
// - DebugMailer：仅把邮件内容打印到控制台，用于本地联调（email.debug=true）；
// - SMTPMailer：标准 SMTP + TLS 直发，适合对接 QQ/163/企业邮箱等。
package mail

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"sync"

	"feed/config"
)

// Mailer 邮件发送契约。
type Mailer interface {
	Send(to, subject, body string) error
}

var (
	mailerOnce sync.Once
	mailer     Mailer
)

// Default 返回进程级单例 Mailer，按配置选择实现。
func Default() Mailer {
	mailerOnce.Do(func() {
		if config.AppConfig.Email.Debug {
			mailer = &DebugMailer{}
			log.Println("[Mail] debug mode enabled, emails will be printed to console")
			return
		}
		mailer = &SMTPMailer{}
	})
	return mailer
}

// DebugMailer 调试模式：不真正发信，只打印。
type DebugMailer struct{}

func (d *DebugMailer) Send(to, subject, body string) error {
	log.Printf("[Mail][Debug] to=%s subject=%s\n---\n%s\n---", to, subject, body)
	return nil
}

// SMTPMailer 标准 SMTP 发送实现。
type SMTPMailer struct{}

// Send 通过 SMTP 发送纯文本邮件。
// 使用隐式 TLS（通常为 465 端口）。
func (s *SMTPMailer) Send(to, subject, body string) error {
	cfg := config.AppConfig.Email
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	host := cfg.Host
	fromAddr := extractAddress(cfg.From)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, host)

	headers := map[string]string{
		"From":         cfg.From,
		"To":           to,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": `text/plain; charset="UTF-8"`,
	}
	var sb strings.Builder
	for k, v := range headers {
		fmt.Fprintf(&sb, "%s: %s\r\n", k, v)
	}
	sb.WriteString("\r\n")
	sb.WriteString(body)

	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return fmt.Errorf("dial smtp %s failed: %w", addr, err)
	}
	defer func() { _ = conn.Close() }()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("create smtp client failed: %w", err)
	}
	defer func() { _ = client.Quit() }()

	if ok, _ := client.Extension("AUTH"); ok {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth failed: %w", err)
		}
	}
	if err = client.Mail(fromAddr); err != nil {
		return fmt.Errorf("smtp MAIL failed: %w", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT failed: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA failed: %w", err)
	}
	if _, err = w.Write([]byte(sb.String())); err != nil {
		return fmt.Errorf("smtp write failed: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("smtp close DATA failed: %w", err)
	}
	return nil
}

// extractAddress 从 "Name <addr@x.com>" 形式中提取纯地址。
func extractAddress(from string) string {
	if i := strings.LastIndex(from, "<"); i >= 0 {
		if j := strings.LastIndex(from, ">"); j > i {
			return from[i+1 : j]
		}
	}
	return from
}
