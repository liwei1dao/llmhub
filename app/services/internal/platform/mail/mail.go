// Package mail provides a tiny SMTP client for outbound transactional
// emails — currently only email-verification codes used during account
// signup. It is deliberately minimal: one Send call, plaintext body,
// STARTTLS. Heavier needs (templates, attachments, queues) belong in a
// dedicated worker, not here.
package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/llmhub/llmhub/internal/platform/config"
)

// Mailer is the abstraction used by callers; one concrete implementation
// (SMTP) plus a dev no-op are provided here.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
	// Enabled reports whether outbound email is actually wired. Callers
	// (e.g. send-code handler) can refuse to issue codes when the dev
	// mailer is the only one present in a production-like env.
	Enabled() bool
}

// New picks an implementation from config. Returns a dev mailer (logs
// the email to stdout instead of sending) when SMTP is unconfigured —
// this keeps local dev workable without real credentials.
func New(cfg config.SMTPConfig, logger *slog.Logger) Mailer {
	if cfg.Host == "" || cfg.From == "" {
		logger.Warn("SMTP not configured; falling back to dev mailer (emails will be logged, not sent)")
		return &devMailer{logger: logger}
	}
	port := cfg.Port
	if port == 0 {
		port = 587
	}
	return &smtpMailer{
		host:     cfg.Host,
		port:     port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
		fromName: cfg.FromName,
		logger:   logger,
	}
}

// -------- dev fallback --------

type devMailer struct{ logger *slog.Logger }

func (d *devMailer) Enabled() bool { return false }

func (d *devMailer) Send(ctx context.Context, to, subject, body string) error {
	// Keep this single-line so it greps cleanly in `docker logs`.
	d.logger.InfoContext(ctx, "[dev-mailer] would-send email", "to", to, "subject", subject, "body", body)
	return nil
}

// -------- SMTP (STARTTLS, e.g. Gmail :587) --------

type smtpMailer struct {
	host, username, password, from, fromName string
	port                                     int
	logger                                   *slog.Logger
}

func (m *smtpMailer) Enabled() bool { return true }

// Send dials the SMTP server, runs STARTTLS, AUTH PLAIN if creds are
// present, then ships a minimal RFC 5322 message.
//
// We talk to SMTP directly instead of pulling in gomail to keep deps
// lean — the message we send has no attachments, no HTML alternative,
// no inline images, so the standard library is enough.
func (m *smtpMailer) Send(ctx context.Context, to, subject, body string) error {
	if to == "" {
		return errors.New("mail: empty recipient")
	}
	addr := net.JoinHostPort(m.host, strconv.Itoa(m.port))

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", addr, err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, m.host)
	if err != nil {
		return fmt.Errorf("smtp handshake: %w", err)
	}
	defer c.Quit()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if m.username != "" {
		if err := c.Auth(smtp.PlainAuth("", m.username, m.password, m.host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := c.Mail(m.from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}

	fromHeader := m.from
	if m.fromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", m.fromName, m.from)
	}
	msg := buildMessage(fromHeader, to, subject, body)
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close DATA: %w", err)
	}
	return nil
}

func buildMessage(from, to, subject, body string) string {
	var b strings.Builder
	b.WriteString("From: ")
	b.WriteString(from)
	b.WriteString("\r\n")
	b.WriteString("To: ")
	b.WriteString(to)
	b.WriteString("\r\n")
	b.WriteString("Subject: ")
	b.WriteString(subject)
	b.WriteString("\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("Date: ")
	b.WriteString(time.Now().UTC().Format(time.RFC1123Z))
	b.WriteString("\r\n\r\n")
	b.WriteString(body)
	return b.String()
}
