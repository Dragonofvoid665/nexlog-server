package main

import (
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ─── Security Headers ─────────────────────────────────────────────────────────

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdnjs.cloudflare.com; "+
				"font-src 'self' https://fonts.gstatic.com https://cdnjs.cloudflare.com; "+
				"img-src 'self' data: https:; "+
				"script-src 'self' 'unsafe-inline'; "+
				"connect-src 'self';")
		next.ServeHTTP(w, r)
	})
}

// ─── Gzip Compression ─────────────────────────────────────────────────────────

type gzipWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	return g.gz.Write(b)
}

func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		defer gz.Close()
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		next.ServeHTTP(&gzipWriter{ResponseWriter: w, gz: gz}, r)
	})
}

// ─── Rate Limiter (per IP, sliding window) ────────────────────────────────────

type ipEntry struct {
	count     int
	resetAt   time.Time
	mu        sync.Mutex
}

type rateLimiter struct {
	mu      sync.Mutex
	ips     map[string]*ipEntry
	max     int
	window  time.Duration
	cleanup *time.Ticker
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		ips:     make(map[string]*ipEntry),
		max:     max,
		window:  window,
		cleanup: time.NewTicker(5 * time.Minute),
	}
	go func() {
		for range rl.cleanup.C {
			now := time.Now()
			rl.mu.Lock()
			for ip, e := range rl.ips {
				e.mu.Lock()
				if now.After(e.resetAt) {
					delete(rl.ips, ip)
				}
				e.mu.Unlock()
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	e, ok := rl.ips[ip]
	if !ok {
		e = &ipEntry{}
		rl.ips[ip] = e
	}
	rl.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	if now.After(e.resetAt) {
		e.count = 0
		e.resetAt = now.Add(rl.window)
	}
	e.count++
	return e.count <= rl.max
}

func rateLimitMiddleware(next http.Handler, max int, window time.Duration) http.Handler {
	rl := newRateLimiter(max, window)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only rate-limit API endpoints
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		ip := realIP(r)
		if !rl.allow(ip) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"Too many requests"}`, http.StatusTooManyRequests)
			return
		}
		w.Header().Set("X-RateLimit-Limit", "100")
		next.ServeHTTP(w, r)
	})
}

func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

// Satisfy io.ReadCloser for body — needed to avoid import error
var _ io.ReadCloser = nil
