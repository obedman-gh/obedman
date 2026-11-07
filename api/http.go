package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const maxBody = 16 << 10
const maxName = 80
const maxMessage = 2000

type api struct {
	store   store
	cfg     config
	logger  *slog.Logger
	limiter *rateLimiter
}
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, code, msg string) {
	var b errorBody
	b.Error.Code = code
	b.Error.Message = msg
	writeJSON(w, status, b)
}
func newHandler(s store, c config, logger *slog.Logger, csp string) http.Handler {
	a := &api{store: s, cfg: c, logger: logger, limiter: &rateLimiter{buckets: make(map[string]bucket), now: time.Now}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/guestbook", a.guestbook)
	mux.HandleFunc("/api/live", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			methodError(w, "GET")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/ready", a.ready)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "not_found", "Endpoint tidak ditemukan.") })
	files := http.FileServer(http.Dir(c.StaticDir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			methodError(w, "GET, HEAD")
			return
		}
		for _, part := range strings.Split(r.URL.Path, "/") {
			if strings.HasPrefix(part, ".") {
				http.NotFound(w, r)
				return
			}
		}
		path := filepath.Join(c.StaticDir, filepath.FromSlash(r.URL.Path))
		info, err := os.Stat(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if info.IsDir() {
			if _, err := os.Stat(filepath.Join(path, "index.html")); err != nil {
				http.NotFound(w, r)
				return
			}
		}
		files.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			fail(w, 503, "unavailable", "Layanan belum tersedia.")
			return
		}
		requestID := hex.EncodeToString(random[:])
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", csp)
		if c.Production {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		sw := &statusWriter{ResponseWriter: w, status: 200}
		// Route labels exclude query strings, client IPs and user-controlled paths from logs.
		route := "static"
		if r.URL.Path == "/api/guestbook" || r.URL.Path == "/api/live" || r.URL.Path == "/api/ready" {
			route = r.URL.Path
		}
		defer func() {
			if recover() != nil {
				fail(sw, 500, "internal", "Terjadi kesalahan internal.")
				logger.Error("request panic", "request_id", requestID)
			}
			logger.Info("request", "request_id", requestID, "route", route, "status", sw.status, "duration_ms", time.Since(started).Milliseconds())
		}()
		mux.ServeHTTP(sw, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.written {
		return
	}
	w.status = status
	w.written = true
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func methodError(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	fail(w, 405, "method_not_allowed", "Metode tidak didukung.")
}
func (a *api) ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		methodError(w, "GET")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	if err := a.store.Ready(ctx); err != nil {
		a.dbFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ready"})
}
func (a *api) dbFailure(w http.ResponseWriter, err error) {
	kind := "database"
	if errors.Is(err, context.DeadlineExceeded) {
		kind = "timeout"
	}
	a.logger.Error("dependency failure", "request_id", w.Header().Get("X-Request-ID"), "kind", kind)
	fail(w, 503, "unavailable", "Layanan sementara tidak tersedia. Coba lagi nanti.")
}
func (a *api) guestbook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		methodError(w, "GET, POST")
		return
	}
	limit := 60
	if r.Method == "POST" {
		limit = 5
	}
	if !a.limiter.allow(r.Method+":"+clientIP(r, a.cfg.TrustedProxies), limit) {
		w.Header().Set("Retry-After", "60")
		fail(w, 429, "rate_limited", "Terlalu banyak permintaan. Tunggu satu menit.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	if r.Method == "GET" {
		a.list(ctx, w, r)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") || (a.cfg.Production && u.Scheme != "https") {
			fail(w, 403, "origin_forbidden", "Origin tidak diizinkan.")
			return
		}
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		fail(w, 415, "content_type", "Gunakan Content-Type application/json.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, 413, "body_too_large", "Payload terlalu besar.")
		} else {
			fail(w, 400, "invalid_json", "JSON tidak valid.")
		}
		return
	}
	if !utf8.Valid(body) {
		fail(w, 400, "invalid_json", "JSON harus UTF-8 valid.")
		return
	}
	var in input
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		fail(w, 400, "invalid_json", "JSON tidak valid atau berisi field yang tidak didukung.")
		return
	}
	if dec.Decode(new(any)) != io.EOF {
		fail(w, 400, "invalid_json", "Kirim satu objek JSON.")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Message = strings.TrimSpace(in.Message)
	if strings.ContainsRune(in.Name, '\x00') || strings.ContainsRune(in.Message, '\x00') {
		fail(w, 422, "validation", "Input mengandung karakter yang tidak didukung.")
		return
	}
	if n, m := utf8.RuneCountInString(in.Name), utf8.RuneCountInString(in.Message); n < 1 || n > maxName || m < 1 || m > maxMessage {
		fail(w, 422, "validation", "Nama harus 1–80 karakter dan pesan 1–2000 karakter.")
		return
	}
	e, err := a.store.Create(ctx, in)
	if err != nil {
		a.dbFailure(w, err)
		return
	}
	writeJSON(w, 201, e)
}
func (a *api) list(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	limit := 20
	if values, exists := r.URL.Query()["limit"]; exists {
		var err error
		if len(values) != 1 {
			fail(w, 400, "pagination", "Limit tidak valid.")
			return
		}
		limit, err = strconv.Atoi(values[0])
		if err != nil || limit < 1 || limit > 50 {
			fail(w, 400, "pagination", "Limit harus 1–50.")
			return
		}
	}
	var after *cursor
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		b, err := base64.RawURLEncoding.DecodeString(raw)
		var c cursor
		if err != nil || len(raw) > 512 || json.Unmarshal(b, &c) != nil || c.ID < 1 || c.CreatedAt.IsZero() {
			fail(w, 400, "pagination", "Cursor tidak valid.")
			return
		}
		after = &c
	}
	entries, err := a.store.List(ctx, after, limit+1)
	if err != nil {
		a.dbFailure(w, err)
		return
	}
	next := ""
	if len(entries) > limit {
		entries = entries[:limit]
		last := entries[len(entries)-1]
		b, _ := json.Marshal(cursor{last.CreatedAt, last.ID})
		next = base64.RawURLEncoding.EncodeToString(b)
	}
	if entries == nil {
		entries = []entry{}
	}
	writeJSON(w, 200, struct {
		Items []entry `json:"items"`
		Next  string  `json:"next_cursor"`
	}{entries, next})
}
func clientIP(r *http.Request, trusted []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "unknown"
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	ip = ip.Unmap()
	for _, prefix := range trusted {
		if prefix.Contains(ip) {
			// cloudflared must be the direct peer; Cloudflare overwrites this single-value header.
			forwarded, err := netip.ParseAddr(r.Header.Get("CF-Connecting-IP"))
			if err == nil {
				return forwarded.Unmap().String()
			}
		}
	}
	return ip.String() // X-Forwarded-For is never used.
}

type bucket struct {
	start time.Time
	count int
}
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]bucket
	now     func() time.Time
}

func (l *rateLimiter) allow(key string, limit int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, exists := l.buckets[key]
	if !exists || now.Sub(b.start) >= time.Minute {
		if len(l.buckets) >= 10000 {
			for k, v := range l.buckets {
				if now.Sub(v.start) >= time.Minute {
					delete(l.buckets, k)
				}
			}
			if len(l.buckets) >= 10000 {
				return false
			}
		}
		b = bucket{start: now}
	}
	if b.count >= limit {
		return false
	}
	b.count++
	l.buckets[key] = b
	return true
}
