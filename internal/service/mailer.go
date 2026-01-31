package service

import (
	"fmt"
	"log"
	"net/smtp"

	"bytebattle/internal/config"
)

type Mailer interface {
	SendVerificationCode(to, code string) error
}

type smtpMailer struct {
	cfg *config.SMTPConfig
}

func NewSMTPMailer(cfg *config.SMTPConfig) Mailer {
	return &smtpMailer{cfg: cfg}
}

func (m *smtpMailer) SendVerificationCode(to, code string) error {
	subject := "Byte-Battle: Код подтверждения"
	body := fmt.Sprintf("Ваш код подтверждения: %s\n\nКод действителен 15 минут.", code)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		m.cfg.From, to, subject, body)

	auth := smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)

	return smtp.SendMail(addr, auth, m.cfg.From, []string{to}, []byte(msg))
}

type devMailer struct{}

func NewDevMailer() Mailer {
	return &devMailer{}
}

func (m *devMailer) SendVerificationCode(to, code string) error {
	log.Printf("[DEV MAILER] Email to: %s | Verification code: %s", to, code)
	return nil
}

func NewMailer(cfg *config.SMTPConfig) Mailer {
	if cfg.Enabled {
		log.Println("[MAILER] Using SMTP mailer")
		return NewSMTPMailer(cfg)
	}
	log.Println("[MAILER] SMTP not configured, using dev mailer (codes in stdout)")
	return NewDevMailer()
}