package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/debug"
	"excentrico-tools-go/internal/logger"
)

// captureOutput runs fn with os.Stdout (structured logger) and the standard
// log package (debug.Printf) redirected, and returns everything written.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdout, origLog := os.Stdout, log.Writer()
	var buf bytes.Buffer
	var mu sync.Mutex
	done := make(chan struct{})
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, r)
		mu.Lock()
		buf.Write(b.Bytes())
		mu.Unlock()
		close(done)
	}()
	var logBuf bytes.Buffer
	os.Stdout = w
	log.SetOutput(&logBuf)
	defer func() {
		os.Stdout = origStdout
		log.SetOutput(origLog)
	}()

	fn()

	_ = w.Close()
	<-done
	mu.Lock()
	defer mu.Unlock()
	return buf.String() + logBuf.String()
}

func TestWordPressService_NeverLogsCredentials(t *testing.T) {
	t.Chdir(t.TempDir()) // the logger creates logs/ in the working directory

	const user, pass = "editor", "s3cret-app-pass"
	encoded := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))

	var gotAuth []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		mu.Unlock()
		if strings.Contains(r.URL.Path, "menus") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[{"id":5,"name":"Main 2026","slug":"main-2026"}]`)
			return
		}
		http.Error(w, strings.Repeat("x", 5000), http.StatusInternalServerError)
	}))
	defer srv.Close()

	debug.SetEnabled(true)
	defer debug.SetEnabled(false)
	logger.Get().SetSampleRate(1.0)

	out := captureOutput(t, func() {
		s := NewWordPressService(config.WordPressConfig{BaseURL: srv.URL, Username: user, ApplicationPassword: pass})
		if _, err := s.GetNavMenus(context.Background()); err != nil {
			t.Errorf("GetNavMenus: %v", err)
		}
		if _, err := s.SearchCategories(context.Background(), "2026"); err == nil {
			t.Error("SearchCategories: expected error from HTTP 500")
		}
	})

	if out == "" {
		t.Fatal("captured no output; the test would not detect a leak")
	}
	for _, secret := range []string{encoded, pass} {
		if strings.Contains(out, secret) {
			t.Errorf("output contains credential %q", secret)
		}
	}
	if strings.Contains(out, strings.Repeat("x", maxLoggedBodyBytes+1)) {
		t.Error("response body was logged without truncation")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(gotAuth) == 0 || gotAuth[0] != "Basic "+encoded {
		t.Errorf("server got Authorization %v, want Basic credentials", gotAuth)
	}
}

func TestWordPressService_PasswordFallback(t *testing.T) {
	s := NewWordPressService(config.WordPressConfig{BaseURL: "http://x", Username: "u", Password: "p"})
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("u:p"))
	if s.authHeader != want {
		t.Errorf("authHeader = %q, want %q", s.authHeader, want)
	}
	if s.client.Timeout != httpTimeout {
		t.Errorf("client timeout = %v, want %v", s.client.Timeout, httpTimeout)
	}
}

func TestWordPressService_Download(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			http.Error(w, "auth header must not be sent", http.StatusBadRequest)
			return
		}
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, "image-bytes")
	}))
	defer srv.Close()

	s := NewWordPressService(config.WordPressConfig{BaseURL: srv.URL, Username: "u", Password: "p"})

	data, err := s.Download(context.Background(), srv.URL+"/img.jpg")
	if err != nil || string(data) != "image-bytes" {
		t.Errorf("Download = %q, %v; want image-bytes, nil", data, err)
	}
	if _, err := s.Download(context.Background(), srv.URL+"/missing"); err == nil {
		t.Error("Download of 404: expected error")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Download(ctx, srv.URL+"/img.jpg"); !errors.Is(err, context.Canceled) {
		t.Errorf("Download with cancelled ctx: err = %v, want context.Canceled", err)
	}
}

func TestTruncateForLog(t *testing.T) {
	if got := truncateForLog([]byte("short")); got != "short" {
		t.Errorf("truncateForLog(short) = %q", got)
	}
	long := bytes.Repeat([]byte("a"), maxLoggedBodyBytes+10)
	got := truncateForLog(long)
	if !strings.HasPrefix(got, string(long[:maxLoggedBodyBytes])+"...") || strings.Contains(got, string(long)) {
		t.Errorf("truncateForLog(long) not truncated: len %d", len(got))
	}
	if !strings.Contains(got, fmt.Sprintf("%d bytes total", len(long))) {
		t.Errorf("truncateForLog(long) = %q, want total size noted", got[len(got)-40:])
	}
}

func TestErrMetadataNotFound_IsWrapped(t *testing.T) {
	err := fmt.Errorf("%w for film '%s' type '%s'", ErrMetadataNotFound, "f", "t")
	if !errors.Is(fmt.Errorf("outer: %w", err), ErrMetadataNotFound) {
		t.Error("errors.Is should see ErrMetadataNotFound through wrapping")
	}
}
