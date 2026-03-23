package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type Mailer interface {
	SendVerificationCode(to, code string) error
}

type resendMailer struct {
	apiKey    string
	fromEmail string
	client    *http.Client
}

func NewResendMailer(apiKey, fromEmail string) Mailer {
	return &resendMailer{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		client:    &http.Client{},
	}
}

func (m *resendMailer) SendVerificationCode(to, code string) error {
	body, err := json.Marshal(map[string]any{
		"from":    m.fromEmail,
		"to":      []string{to},
		"subject": "Ваш код входа в Byte Battle",
		"html": fmt.Sprintf(
			`<p>Ваш код подтверждения: <strong>%s</strong></p><p>Код действителен 15 минут.</p>`,
			code,
		),
	})
	if err != nil {
		return fmt.Errorf("marshal email body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("resend API error: status %d", resp.StatusCode)
	}
	return nil
}

type devMailer struct{}

func NewDevMailer() Mailer {
	return &devMailer{}
}

func (m *devMailer) SendVerificationCode(to, code string) error {
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
