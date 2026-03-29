package service

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
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

const verificationEmailSubject = "Ваш код входа в Byte Battle"

var verificationEmailTemplate = template.Must(template.New("verification_email").Parse(`<!doctype html>
<html lang="ru">
  <head>
	<meta charset="UTF-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1.0" />
	<title>Код подтверждения</title>
  </head>
  <body style="margin:0;padding:0;background:#0b1020;font-family:Inter,Arial,sans-serif;color:#e8ecff;">
	<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="padding:24px 12px;">
	  <tr>
		<td align="center">
		  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:560px;background:#131a33;border:1px solid #283463;border-radius:14px;overflow:hidden;">
			<tr>
			  <td style="padding:20px 24px;background:#1b2447;color:#8ea0ff;font-size:13px;font-weight:600;letter-spacing:0.08em;text-transform:uppercase;">Byte Battle</td>
			</tr>
			<tr>
			  <td style="padding:28px 24px 8px 24px;font-size:24px;line-height:1.3;font-weight:700;color:#ffffff;">Ваш код подтверждения</td>
			</tr>
			<tr>
			  <td style="padding:0 24px 16px 24px;font-size:15px;line-height:1.6;color:#c9d2ff;">Введите этот код на странице входа. Код действует 15 минут.</td>
			</tr>
			<tr>
			  <td style="padding:0 24px 24px 24px;">
				<div style="display:inline-block;padding:12px 20px;border-radius:10px;border:1px solid #3a4a87;background:#0d1430;color:#ffffff;font-size:28px;font-weight:700;letter-spacing:0.24em;">{{.Code}}</div>
			  </td>
			</tr>
			<tr>
			  <td style="padding:0 24px 24px 24px;font-size:13px;line-height:1.6;color:#9aa6db;">Если вы не запрашивали код, просто проигнорируйте это письмо.</td>
			</tr>
		  </table>
		</td>
	  </tr>
	</table>
  </body>
</html>`))

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
	var b bytes.Buffer
	if err := verificationEmailTemplate.Execute(&b, struct{ Code string }{Code: code}); err != nil {
		return "", err
	}
	return b.String(), nil
}

