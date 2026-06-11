// Package email provides SMTP-based transactional email sending.
//
// In development, it sends to MailDev (no auth, no TLS).
// In production, configure a real SMTP server via SMTP_* env vars.
package email

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"github.com/pixelvide/aegis/server/internal/config"
)

// Service sends transactional emails via SMTP.
type Service struct {
	host     string
	port     int
	username string
	password string
	from     string
	useTLS   bool
}

// New creates an email Service from SMTP configuration.
func New(cfg config.SMTPConfig) *Service {
	s := &Service{
		host:     cfg.Host,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
		useTLS:   cfg.TLS,
	}

	if s.username == "" {
		slog.Info("SMTP auth disabled (MailDev mode)", "web_ui", "http://localhost:1080", "component", "email")
	} else {
		slog.Info("SMTP configured", "host", s.host, "port", s.port, "tls", s.useTLS, "component", "email")
	}

	return s
}

// Send sends an email with the given subject and HTML body.
func (s *Service) Send(to, subject, htmlBody string) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	msg := buildMessage(s.from, to, subject, htmlBody)

	// Use SMTP auth only if credentials are provided
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	if s.useTLS {
		if err := s.sendWithTLS(addr, auth, msg, to); err != nil {
			slog.Error("email send failed", "to", to, "subject", subject, "error", err, "tls", true, "component", "email")
			return err
		}
		slog.Debug("email sent", "to", to, "subject", subject, "tls", true, "component", "email")
		return nil
	}

	if err := smtp.SendMail(addr, auth, s.from, []string{to}, msg); err != nil {
		slog.Error("email send failed", "to", to, "subject", subject, "error", err, "tls", false, "component", "email")
		return err
	}
	slog.Debug("email sent", "to", to, "subject", subject, "tls", false, "component", "email")
	return nil
}

// sendWithTLS connects with STARTTLS for production SMTP providers.
func (s *Service) sendWithTLS(addr string, auth smtp.Auth, msg []byte, to string) error {
	conn, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer conn.Close()

	// STARTTLS
	tlsConfig := &tls.Config{
		ServerName: s.host,
		MinVersion: tls.VersionTLS12,
	}
	if err := conn.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("smtp starttls: %w", err)
	}

	if auth != nil {
		if err := conn.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := conn.Mail(s.from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := conn.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}

	wc, err := conn.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := wc.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}

	return conn.Quit()
}

// buildMessage constructs a MIME email message.
func buildMessage(from, to, subject, htmlBody string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String())
}
