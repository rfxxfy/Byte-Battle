package service

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"

	"github.com/resend/resend-go/v2"
)

//go:embed templates
var emailTemplates embed.FS

type Mailer interface {
	SendVerificationCode(ctx context.Context, to, code string) error
}

type resendMailer struct {
	client    *resend.Client
	fromEmail string
}

const verificationEmailSubject = "Ваш код входа в Byte Battle"

var verificationEmailTemplate = template.Must(
	template.ParseFS(emailTemplates, "templates/verification_email.html"),
)

func NewResendMailer(apiKey, fromEmail string) Mailer {
	return &resendMailer{
		client:    resend.NewClient(apiKey),
		fromEmail: fromEmail,
	}
}

func (m *resendMailer) SendVerificationCode(ctx context.Context, to, code string) error {
	htmlBody, err := renderVerificationEmailHTML(code)
	if err != nil {
		return fmt.Errorf("render email html: %w", err)
	}

	_, err = m.client.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
		From:    m.fromEmail,
		To:      []string{to},
		Subject: verificationEmailSubject,
		Html:    htmlBody,
		Text:    fmt.Sprintf("Ваш код подтверждения: %s\nКод действителен 15 минут.", code),
	})
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

type devMailer struct{}

func NewDevMailer() Mailer {
	return &devMailer{}
}

func (m *devMailer) SendVerificationCode(_ context.Context, to, code string) error {
	log.Printf("[DEV MAILER] to=%s | verification_code=%s", to, code)
	return nil
}

func NewMailer(apiKey, fromEmail string) Mailer {
	if apiKey != "" {
		log.Println("[MAILER] Resend mailer enabled")
		return NewResendMailer(apiKey, fromEmail)
	}
	log.Println("[MAILER] RESEND_API_KEY not set, using dev mailer (codes will be printed to logs)")
	return NewDevMailer()
}

func renderVerificationEmailHTML(code string) (string, error) {
	digits := make([]string, len(code))
	for i, c := range code {
		digits[i] = string(c)
	}
	var b bytes.Buffer
	if err := verificationEmailTemplate.Execute(&b, struct {
		Code   string
		Digits []string
	}{Code: code, Digits: digits}); err != nil {
		return "", err
	}
	return b.String(), nil
}
