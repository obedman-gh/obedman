package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestLogsDoNotContainSensitiveInput(t *testing.T) {
	var logs bytes.Buffer
	h := newHandler(&fakeStore{fail: true}, config{}, slog.New(slog.NewJSONHandler(&logs, nil)), "default-src 'self'")
	r := httptest.NewRequest("POST", "/api/guestbook?token=synthetic-private-query", strings.NewReader(`{"name":"synthetic-private-name","message":"synthetic-private-body"}`))
	r.RemoteAddr = "192.0.2.22:1234"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Request-ID", "synthetic-untrusted-id")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	for _, value := range []string{"synthetic-private", "private driver details", "192.0.2.22", "synthetic-untrusted-id"} {
		if strings.Contains(logs.String(), value) {
			t.Fatal("sensitive value reached logs")
		}
	}
	if w.Header().Get("X-Request-ID") == "" || !strings.Contains(logs.String(), w.Header().Get("X-Request-ID")) {
		t.Fatal("request ID not correlated")
	}
}

type fakeStore struct {
	fail    bool
	created int
	last    input
	entries []entry
}

func (s *fakeStore) Create(ctx context.Context, in input) (entry, error) {
	if s.fail {
		return entry{}, errors.New("private driver details must not reach response")
	}
	s.created++
	s.last = in
	return entry{ID: 1, Name: in.Name, Message: in.Message, CreatedAt: time.Now().UTC(), Status: "pending"}, nil
}
func (s *fakeStore) List(ctx context.Context, c *cursor, n int) ([]entry, error) {
	if s.fail {
		return nil, errors.New("query failed")
	}
	return s.entries, nil
}
func (s *fakeStore) Ready(context.Context) error {
	if s.fail {
		return errors.New("unavailable")
	}
	return nil
}
func testHandler(s store) http.Handler {
	return newHandler(s, config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "default-src 'self'")
}
func request(h http.Handler, method, path, body, contentType string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.1:1234"
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestInput(t *testing.T) {
	cases := []struct {
		name, body, contentType string
		status                  int
	}{
		{"valid", `{"name":" visitor ","message":" hello "}`, "application/json", 201},
		{"unicode boundary", `{"name":"` + strings.Repeat("😀", 80) + `","message":"` + strings.Repeat("界", 2000) + `"}`, "application/json; charset=utf-8", 201},
		{"name too long", `{"name":"` + strings.Repeat("😀", 81) + `","message":"hello"}`, "application/json", 422},
		{"message too long", `{"name":"visitor","message":"` + strings.Repeat("界", 2001) + `"}`, "application/json", 422},
		{"whitespace", `{"name":" \t\n","message":"hi"}`, "application/json", 422},
		{"empty message", `{"name":"a","message":" "}`, "application/json", 422},
		{"null character", `{"name":"a","message":"\u0000"}`, "application/json", 422},
		{"null", `null`, "application/json", 422},
		{"malformed", `{"name":`, "application/json", 400},
		{"array", `[]`, "application/json", 400},
		{"type", `{"name":4,"message":"a"}`, "application/json", 400},
		{"extra json", `{"name":"a","message":"b"}{}`, "application/json", 400},
		{"trailing junk", `{"name":"a","message":"b"}xx`, "application/json", 400},
		{"id", `{"name":"a","message":"b","id":999}`, "application/json", 400},
		{"timestamp", `{"name":"a","message":"b","created_at":"2000-01-01T00:00:00Z"}`, "application/json", 400},
		{"status", `{"name":"a","message":"b","status":"approved"}`, "application/json", 400},
		{"unknown", `{"name":"a","message":"b","other":true}`, "application/json", 400},
		{"content type", `{"name":"a","message":"b"}`, "text/plain", 415},
		{"missing content type", `{}`, "", 415},
		{"body too large", strings.Repeat(" ", maxBody+1), "application/json", 413},
		{"invalid utf8", string([]byte{0xff}), "application/json", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeStore{}
			w := request(testHandler(s), "POST", "/api/guestbook", tc.body, tc.contentType)
			if w.Code != tc.status {
				t.Fatalf("status=%d expected=%d", w.Code, tc.status)
			}
			if tc.status == 201 {
				var e entry
				if json.Unmarshal(w.Body.Bytes(), &e) != nil || e.ID != 1 || e.Status != "pending" || e.CreatedAt.IsZero() {
					t.Fatal("server fields missing")
				}
				if strings.TrimSpace(s.last.Name) != s.last.Name {
					t.Fatal("input not trimmed")
				}
			} else if s.created != 0 {
				t.Fatal("invalid input reached insert")
			}
		})
	}
}
func TestDatabaseFailuresAndHealth(t *testing.T) {
	h := testHandler(&fakeStore{fail: true})
	for _, method := range []string{"GET", "POST"} {
		w := request(h, method, "/api/guestbook", `{"name":"a","message":"b"}`, "application/json")
		if w.Code != 503 || strings.Contains(w.Body.String(), "private") {
			t.Fatal("database failure exposed or reported success")
		}
	}
	if request(h, "GET", "/api/live", "", "").Code != 200 {
		t.Fatal("liveness depends on DB")
	}
	if request(h, "GET", "/api/ready", "", "").Code != 503 {
		t.Fatal("readiness ignored DB")
	}
	w := request(testHandler(&fakeStore{}), "GET", "/api/guestbook", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatal("empty response incorrect")
	}
}
func TestRateLimitAndForgedHeaders(t *testing.T) {
	h := testHandler(&fakeStore{})
	for i := 0; i < 6; i++ {
		r := httptest.NewRequest("POST", "/api/guestbook", strings.NewReader(`{"name":"a","message":"b"}`))
		r.RemoteAddr = "192.0.2.1:9999"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Forwarded-For", netip.AddrFrom4([4]byte{198, 51, 100, byte(i + 1)}).String())
		r.Header.Set("CF-Connecting-IP", r.Header.Get("X-Forwarded-For"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if i < 5 && w.Code != 201 {
			t.Fatal("request blocked too early")
		}
		if i == 5 && (w.Code != 429 || w.Header().Get("Retry-After") == "") {
			t.Fatal("spoofing bypassed limiter")
		}
	}
}
func TestTrustedProxy(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1000"
	r.Header.Set("CF-Connecting-IP", "198.51.100.8")
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	if clientIP(r, trusted) != "198.51.100.8" {
		t.Fatal("trusted proxy missing")
	}
	r.Header.Set("CF-Connecting-IP", "invalid")
	if clientIP(r, trusted) != "127.0.0.1" {
		t.Fatal("invalid header trusted")
	}
}
func TestLimiterExpiryAndCapacity(t *testing.T) {
	now := time.Now()
	l := &rateLimiter{buckets: map[string]bucket{}, now: func() time.Time { return now }}
	if !l.allow("a", 1) || l.allow("a", 1) {
		t.Fatal("limit broken")
	}
	now = now.Add(time.Minute)
	if !l.allow("a", 1) {
		t.Fatal("expiry broken")
	}
	for i := 0; i < 10000; i++ {
		l.buckets[time.Unix(int64(i), 0).String()] = bucket{start: now}
	}
	if l.allow("new", 1) {
		t.Fatal("limiter capacity must fail closed")
	}
	now = now.Add(time.Minute)
	if !l.allow("new", 1) {
		t.Fatal("expired capacity not reclaimed")
	}
}
func TestPaginationValidation(t *testing.T) {
	for _, q := range []string{"limit=0", "limit=51", "limit=abc", "limit=1&limit=2", "cursor=garbage"} {
		if request(testHandler(&fakeStore{}), "GET", "/api/guestbook?"+q, "", "").Code != 400 {
			t.Fatal("invalid pagination accepted")
		}
	}
	entries := []entry{{ID: 2, CreatedAt: time.Now()}, {ID: 1, CreatedAt: time.Now()}}
	w := request(testHandler(&fakeStore{entries: entries}), "GET", "/api/guestbook?limit=1", "", "")
	var page struct {
		Items []entry `json:"items"`
		Next  string  `json:"next_cursor"`
	}
	if json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Items) != 1 || page.Next == "" {
		t.Fatal("next page missing")
	}
}
func TestHeadersOriginsAndMethods(t *testing.T) {
	h := testHandler(&fakeStore{})
	w := request(h, "GET", "/api/live", "", "")
	for _, header := range []string{"X-Request-ID", "Content-Security-Policy", "X-Content-Type-Options", "Cache-Control"} {
		if w.Header().Get(header) == "" {
			t.Fatalf("missing %s", header)
		}
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unexpected CORS")
	}
	r := httptest.NewRequest("POST", "http://localhost/api/guestbook", strings.NewReader(`{}`))
	r.Header.Set("Origin", "https://evil.invalid")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross origin write accepted")
	}
	if request(h, "DELETE", "/api/guestbook", "", "").Code != 405 {
		t.Fatal("invalid method accepted")
	}
}
