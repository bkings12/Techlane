// Package mailer provides a minimal SMTP client (stdlib net/smtp, no external
// deps) so the platform has an email channel for things like password resets.
// If SMTP isn't configured, Send() logs instead of erroring so the surrounding
// flow (e.g. forgot-password) still completes in dev/self-hosted setups.
package mailer

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"

	"github.com/techlane/techlane/packages/pkg/config"
)

type Sender interface {
	Send(to, subject, body string) error
}

type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func ConfigFromEnv() Config {
	return Config{
		Host:     config.Env("SMTP_HOST", ""),
		Port:     config.Env("SMTP_PORT", "587"),
		Username: config.Env("SMTP_USERNAME", ""),
		Password: config.Env("SMTP_PASSWORD", ""),
		From:     config.Env("SMTP_FROM", "TechLane <no-reply@techlane.local>"),
	}
}

func (c Config) Configured() bool {
	return c.Host != "" && c.Username != ""
}

// New returns an SMTP-backed sender, or a no-op sender (that just logs) when
// SMTP isn't configured.
func New(cfg Config) Sender {
	if !cfg.Configured() {
		return noopSender{}
	}
	return smtpSender{cfg: cfg}
}

type smtpSender struct{ cfg Config }

func (s smtpSender) Send(to, subject, body string) error {
	addr := net.JoinHostPort(s.cfg.Host, s.cfg.Port)
	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}
	msg := buildMessage(s.cfg.From, to, subject, body)

	if s.cfg.Port == "465" {
		return sendImplicitTLS(addr, s.cfg.Host, auth, s.cfg.From, to, msg)
	}
	return smtp.SendMail(addr, auth, senderAddress(s.cfg.From), []string{to}, msg)
}

// sendImplicitTLS handles port 465 (SMTPS), which smtp.SendMail doesn't
// support directly since it always dials plaintext then optionally STARTTLS.
func sendImplicitTLS(addr, host string, auth smtp.Auth, from, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	defer conn.Close()
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(senderAddress(from)); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// senderAddress strips a "Display Name <addr>" wrapper down to the bare
// address, since SMTP envelope commands (MAIL FROM) need just the address.
func senderAddress(from string) string {
	if i := strings.LastIndex(from, "<"); i != -1 {
		if j := strings.LastIndex(from, ">"); j > i {
			return from[i+1 : j]
		}
	}
	return from
}

func buildMessage(from, to, subject, body string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
	b.WriteString(body)
	b.WriteString("\r\n")
	return []byte(b.String())
}

type noopSender struct{}

func (noopSender) Send(to, subject, _ string) error {
	slog.Warn("email not sent: SMTP not configured (set SMTP_HOST/SMTP_USERNAME/SMTP_PASSWORD)", "to", to, "subject", subject)
	return nil
}
