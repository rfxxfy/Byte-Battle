package service

import (
	"strings"
	"testing"
)

func TestRenderVerificationEmailHTML_IncludesCode(t *testing.T) {
	html, err := renderVerificationEmailHTML("123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "123456") {
		t.Fatalf("expected rendered html to contain code")
	}
	if !strings.Contains(html, "Byte Battle") {
		t.Fatalf("expected rendered html to contain branding")
	}
}

func TestRenderVerificationEmailHTML_EscapesContent(t *testing.T) {
	html, err := renderVerificationEmailHTML("<b>123</b>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(html, "<b>123</b>") {
		t.Fatalf("expected raw html to be escaped")
	}
	if !strings.Contains(html, "&lt;b&gt;123&lt;/b&gt;") {
		t.Fatalf("expected escaped content in html")
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
