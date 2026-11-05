package mailsender

import (
	"bufio"
	"errors"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNoopEmailSender(t *testing.T) {
	var s EmailSender = NoopEmailSender{}
	if err := s.SendPlain("a@example.com", "subj", "body"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestSMTPEmailSenderRequiresConfig(t *testing.T) {
	s := NewSMTPEmailSender(EmailConfig{})
	if err := s.SendPlain("a@example.com", "subj", "body"); err == nil {
		t.Fatal("expected error when SMTP is not configured")
	}
}

func TestSMTPEmailSenderSatisfiesInterface(t *testing.T) {
	var _ EmailSender = (*SMTPEmailSender)(nil)
}

// TestRecipient - one bare address only: no list, no name, no line break
// that would add headers.
func TestRecipient(t *testing.T) {
	if got, err := recipient("a@example.com"); err != nil || got != "a@example.com" {
		t.Fatalf("%q %v", got, err)
	}
	for _, to := range []string{"a@example.com\r\nBcc: b@example.com", "a@example.com, b@example.com", "A <a@example.com>", "", "not an address"} {
		if _, err := recipient(to); !errors.Is(err, ErrBadAddress) {
			t.Errorf("%q: %v", to, err)
		}
	}
}

// TestBuildMessage - a subject stays one header line, encoded when it
// isn't ASCII; Date and Message-ID are set; the body comes back as sent.
func TestBuildMessage(t *testing.T) {
	sender := &mail.Address{Name: "Site", Address: "noreply@example.com"}
	body := "Здравствуйте!\nСсылка: https://example.com/" + strings.Repeat("x", 200) + "\n"
	raw, err := buildMessage(sender, "a@example.com", "Привет\r\nBcc: b@example.com", body, time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Header.Get("Bcc") != "" {
		t.Fatal("the subject added a header")
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || subject != "Привет Bcc: b@example.com" {
		t.Errorf("subject %q %v", subject, err)
	}
	if msg.Header.Get("Date") != "Sat, 31 Oct 2026 12:00:00 +0000" || !strings.HasSuffix(msg.Header.Get("Message-ID"), "@example.com>") {
		t.Errorf("Date %q, Message-ID %q", msg.Header.Get("Date"), msg.Header.Get("Message-ID"))
	}
	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 78 {
			t.Fatalf("a line of %d characters", len(line))
		}
	}
	decoded, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil || strings.ReplaceAll(string(decoded), "\r\n", "\n") != body {
		t.Errorf("body %q %v", decoded, err)
	}
}

// fakeSMTP - a server taking one message: the lines it got after DATA
// go to got.
func fakeSMTP(t *testing.T, got chan<- string) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		r := bufio.NewReader(conn)
		say := func(s string) { _, _ = io.WriteString(conn, s+"\r\n") }
		say("220 fake")
		var data strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			switch cmd := strings.ToUpper(strings.TrimSpace(line)); {
			case strings.HasPrefix(cmd, "EHLO"):
				say("250 fake")
			case cmd == "DATA":
				say("354 go on")
				for {
					l, err := r.ReadString('\n')
					if err != nil || l == ".\r\n" {
						break
					}
					data.WriteString(l)
				}
				got <- data.String()
				say("250 kept")
			case cmd == "QUIT":
				say("221 bye")
				return
			default:
				say("250 ok")
			}
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestSMTPEmailSenderSends(t *testing.T) {
	got := make(chan string, 1)
	port := fakeSMTP(t, got)
	s := NewSMTPEmailSender(EmailConfig{SMTPHost: "127.0.0.1", SMTPPort: port, FromAddress: "Site <noreply@example.com>", Timeout: 5 * time.Second})
	if err := s.SendPlain("a@example.com", "Hello", "Body"); err != nil {
		t.Fatal(err)
	}
	if msg := <-got; !strings.Contains(msg, "Subject: Hello\r\n") || !strings.Contains(msg, "Body") {
		t.Fatalf("got %q", msg)
	}
}

// TestSMTPEmailSenderTimeout - a server that never answers doesn't hold
// the send beyond the timeout.
func TestSMTPEmailSenderTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()
	port, _ := strconv.Atoi(strings.TrimPrefix(ln.Addr().String(), "127.0.0.1:"))
	s := NewSMTPEmailSender(EmailConfig{SMTPHost: "127.0.0.1", SMTPPort: port, FromAddress: "noreply@example.com", Timeout: 200 * time.Millisecond})
	start := time.Now()
	if err := s.SendPlain("a@example.com", "s", "b"); err == nil {
		t.Fatal("sent to a server that never answered")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("took %v", d)
	}
}
