package service

import (
	"strings"
	"testing"
)

func TestRenderVerificationEmailHTML_Smoke(t *testing.T) {
	const code = "123456"
	html, err := renderVerificationEmailHTML(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, digit := range code {
		if !strings.Contains(html, string(digit)) {
			t.Fatalf("expected rendered html to contain digit %q", string(digit))
		}
	}
}

func TestNewMailer_SelectsDevMailerWithoutAPIKey(t *testing.T) {
	mailer := NewMailer("", "noreply@bytebattle.dev")
	if _, ok := mailer.(*devMailer); !ok {
		t.Fatalf("expected devMailer when api key is empty")
	}
}

func TestNewMailer_SelectsResendMailerWithAPIKey(t *testing.T) {
	mailer := NewMailer("test-api-key", "noreply@bytebattle.dev")
	if _, ok := mailer.(*resendMailer); !ok {
		t.Fatalf("expected resendMailer when api key is set")
	}
}
