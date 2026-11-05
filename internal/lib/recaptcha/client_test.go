package recaptcha

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_Verify_success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("secret") != "sec" {
			t.Errorf("secret %q", r.Form.Get("secret"))
		}
		if r.Form.Get("response") != "tok" {
			t.Errorf("response %q", r.Form.Get("response"))
		}
		if r.Form.Get("remoteip") != "198.51.100.1" {
			t.Errorf("remoteip %q", r.Form.Get("remoteip"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true}`))
	}))
	defer ts.Close()

	c := NewClient("sec", time.Second)
	c.verifyURL = ts.URL

	ok, err := c.Verify(context.Background(), "tok", "198.51.100.1")
	if err != nil || !ok {
		t.Fatalf("Verify() = %v, %v", ok, err)
	}
}

func TestClient_Verify_rejected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success": false}`))
	}))
	defer ts.Close()

	c := NewClient("sec", time.Second)
	c.verifyURL = ts.URL

	ok, err := c.Verify(context.Background(), "tok", "")
	if err != nil || ok {
		t.Fatalf("Verify() = %v, %v", ok, err)
	}
}

func TestClient_Verify_emptyToken(t *testing.T) {
	c := NewClient("sec", time.Second)
	ok, err := c.Verify(context.Background(), "   ", "")
	if err != nil || ok {
		t.Fatalf("Verify() = %v, %v", ok, err)
	}
}

func TestClient_Verify_badStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := NewClient("sec", time.Second)
	c.verifyURL = ts.URL

	_, err := c.Verify(context.Background(), "tok", "")
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected 500 error, got %v", err)
	}
}
