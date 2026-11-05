package mailsender

import (
	"testing"
	"time"
)

func TestResendEmailSenderRequiresConfig(t *testing.T) {
	s := NewResendEmailSender("", "", time.Second)
	if err := s.SendPlain("a@example.com", "subj", "body"); err == nil {
		t.Fatal("expected error when api key and from address are missing")
	}
}

func TestResendEmailSenderSatisfiesInterface(t *testing.T) {
	var _ EmailSender = (*ResendEmailSender)(nil)
}

func TestNewResendEmailSenderDefaultsHTTPClient(t *testing.T) {
	s := NewResendEmailSender("key", "from@example.com", time.Second)
	if s.HTTPClient == nil {
		t.Fatal("expected a default HTTP client to be set")
	}
}
