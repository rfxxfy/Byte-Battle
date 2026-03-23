package service

import (
	"context"
	"fmt"
	"log"

	"github.com/resend/resend-go/v2"
)

type Mailer interface {
	SendVerificationCode(ctx context.Context, to, code string) error
}

type resendMailer struct {
	client    *resend.Client
	fromEmail string
}

func NewResendMailer(apiKey, fromEmail string) Mailer {
	return &resendMailer{
		client:    resend.NewClient(apiKey),
		fromEmail: fromEmail,
	}
}

func (m *resendMailer) SendVerificationCode(ctx context.Context, to, code string) error {
	_, err := m.client.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
		From:    m.fromEmail,
		To:      []string{to},
		Subject: "Ваш код входа в Byte Battle",
		Html: fmt.Sprintf(
			`<p>Ваш код подтверждения: <strong>%s</strong></p><p>Код действителен 15 минут.</p>`,
			code,
		),
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
	log.Printf("[DEV MAILER] to=%s | code=%s", to, code)
	return nil
}

func NewMailer(apiKey, fromEmail string) Mailer {
	if apiKey != "" {
		return NewResendMailer(apiKey, fromEmail)
	}
	log.Println("[MAILER] RESEND_API_KEY not set, codes will be printed to stdout")
	return NewDevMailer()
}
