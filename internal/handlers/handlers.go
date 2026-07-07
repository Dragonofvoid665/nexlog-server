// Package handlers contains all HTTP handlers.
// Each handler receives r.Context() and passes it to the service layer.
package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"nexlog/internal/cache"
	mw "nexlog/internal/middleware"
	"nexlog/internal/repository"
	"nexlog/internal/service"
)

type Handler struct {
	svc       *service.Service
	cache     *cache.Cache
	jwtSecret string
	uploadDir string
}

func New(svc *service.Service, c *cache.Cache, jwtSecret, uploadDir string) *Handler {
	return &Handler{svc: svc, cache: c, jwtSecret: jwtSecret, uploadDir: uploadDir}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func readBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v) // 1MB max body
}

// ─── Public ───────────────────────────────────────────────────────────────────

func (h *Handler) Content(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	ctx := r.Context()

	if v, ok := h.cache.Get("content"); ok {
		writeJSON(w, 200, v)
		return
	}

	site, _ := h.svc.GetSite(ctx)
	slides, _ := h.svc.GetHeroSlides(ctx)
	stats, _ := h.svc.GetStats(ctx)
	about, _ := h.svc.GetAbout(ctx)
	services, _ := h.svc.GetServices(ctx)
	news, _ := h.svc.GetNews(ctx)
	partners, _ := h.svc.GetPartners(ctx)
	contact, _ := h.svc.GetContact(ctx)

	payload := map[string]interface{}{
		"site": site, "hero": map[string]interface{}{"slides": slides},
		"stats": stats, "about": about, "services": services,
		"news": news, "partners": partners, "contact": contact,
	}
	h.cache.Set("content", payload)
	writeJSON(w, 200, payload)
}

func (h *Handler) Languages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if v, ok := h.cache.Get("languages"); ok {
		writeJSON(w, 200, v)
		return
	}
	langs, err := h.svc.GetLanguages(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	h.cache.Set("languages", langs)
	writeJSON(w, 200, langs)
}

func (h *Handler) SidePanel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if v, ok := h.cache.Get("sidepanel"); ok {
		writeJSON(w, 200, v)
		return
	}
	cfg, err := h.svc.GetSidePanel(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	h.cache.Set("sidepanel", cfg)
	writeJSON(w, 200, cfg)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := readBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad request"})
		return
	}
	hash, err := h.svc.GetAdminPasswordHash(r.Context())
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		time.Sleep(300 * time.Millisecond) // slow down brute force
		writeJSON(w, 401, map[string]string{"error": "Invalid password"})
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"admin": true,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(h.jwtSecret))
	writeJSON(w, 200, map[string]string{"token": tokenStr})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		NewPassword string `json:"newPassword"`
	}
	if err := readBody(r, &body); err != nil || len(body.NewPassword) < 6 {
		writeJSON(w, 400, map[string]string{"error": "Password must be at least 6 characters"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	h.svc.UpdateAdminPassword(r.Context(), string(hash))
	writeJSON(w, 200, map[string]string{"success": "true"})
}

// ─── Upload ───────────────────────────────────────────────────────────────────

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		writeJSON(w, 400, map[string]string{"error": "File too large (max 5MB)"})
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true}
	if !allowed[ext] {
		writeJSON(w, 400, map[string]string{"error": "Invalid file type"})
		return
	}
	if err := os.MkdirAll(h.uploadDir, 0755); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Cannot create upload dir"})
		return
	}
	filename := fmt.Sprintf("%d-%d%s", time.Now().UnixMilli(), rand.Int63n(1e9), ext)
	dst, err := os.Create(filepath.Join(h.uploadDir, filename))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Cannot save file"})
		return
	}
	defer dst.Close()
	io.Copy(dst, file)
	writeJSON(w, 200, map[string]string{"url": "/uploads/" + filename})
}

// ─── Admin helpers ────────────────────────────────────────────────────────────

func (h *Handler) adminPUT(w http.ResponseWriter, r *http.Request, fn func() error) {
	if r.Method != http.MethodPut {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if err := fn(); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	h.cache.Flush()
	writeJSON(w, 200, map[string]string{"success": "true"})
}

func (h *Handler) AdminSite(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v repository.Site
		readBody(r, &v)
		return h.svc.UpdateSite(r.Context(), v)
	})
}
func (h *Handler) AdminHero(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var body struct {
			Slides []repository.HeroSlide `json:"slides"`
		}
		readBody(r, &body)
		return h.svc.UpdateHeroSlides(r.Context(), body.Slides)
	})
}
func (h *Handler) AdminStats(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v []repository.Stat
		readBody(r, &v)
		return h.svc.UpdateStats(r.Context(), v)
	})
}
func (h *Handler) AdminAbout(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v repository.About
		readBody(r, &v)
		return h.svc.UpdateAbout(r.Context(), v)
	})
}
func (h *Handler) AdminServices(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v []repository.Service
		readBody(r, &v)
		return h.svc.UpdateServices(r.Context(), v)
	})
}
func (h *Handler) AdminNews(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v []repository.NewsItem
		readBody(r, &v)
		return h.svc.UpdateNews(r.Context(), v)
	})
}
func (h *Handler) AdminPartners(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v []repository.Partner
		readBody(r, &v)
		return h.svc.UpdatePartners(r.Context(), v)
	})
}
func (h *Handler) AdminContact(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v repository.Contact
		readBody(r, &v)
		return h.svc.UpdateContact(r.Context(), v)
	})
}
func (h *Handler) AdminLanguages(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v map[string]interface{}
		readBody(r, &v)
		return h.svc.UpdateLanguages(r.Context(), v)
	})
}
func (h *Handler) AdminSidePanel(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v map[string]interface{}
		readBody(r, &v)
		return h.svc.UpdateSidePanel(r.Context(), v)
	})
}

// ─── Router ───────────────────────────────────────────────────────────────────

func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth func(http.Handler) http.Handler, publicDir string) {
	// Health
	mux.HandleFunc("/health", h.Health)

	// Public API
	mux.HandleFunc("/api/content", h.Content)
	mux.HandleFunc("/api/languages", h.Languages)
	mux.HandleFunc("/api/sidepanel", h.SidePanel)

	// Admin auth (no JWT required)
	mux.HandleFunc("/api/admin/login", h.Login)

	// Admin protected
	mux.Handle("/api/admin/change-password", auth(http.HandlerFunc(h.ChangePassword)))
	mux.Handle("/api/admin/upload", auth(http.HandlerFunc(h.Upload)))
	mux.Handle("/api/admin/site", auth(http.HandlerFunc(h.AdminSite)))
	mux.Handle("/api/admin/hero", auth(http.HandlerFunc(h.AdminHero)))
	mux.Handle("/api/admin/stats", auth(http.HandlerFunc(h.AdminStats)))
	mux.Handle("/api/admin/about", auth(http.HandlerFunc(h.AdminAbout)))
	mux.Handle("/api/admin/services", auth(http.HandlerFunc(h.AdminServices)))
	mux.Handle("/api/admin/news", auth(http.HandlerFunc(h.AdminNews)))
	mux.Handle("/api/admin/partners", auth(http.HandlerFunc(h.AdminPartners)))
	mux.Handle("/api/admin/contact", auth(http.HandlerFunc(h.AdminContact)))
	mux.Handle("/api/admin/languages", auth(http.HandlerFunc(h.AdminLanguages)))
	mux.Handle("/api/admin/sidepanel", auth(http.HandlerFunc(h.AdminSidePanel)))

	// Static uploads with long cache
	mux.Handle("/uploads/", mw.AddCacheHeaders("604800",
		http.StripPrefix("/uploads/", http.FileServer(http.Dir(h.uploadDir)))))

	// Admin panel
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(publicDir, "admin.html"))
	})
	mux.HandleFunc("/admin/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(publicDir, "admin.html"))
	})

	// SPA fallback — serve index.html for unknown paths
	fs := http.FileServer(http.Dir(publicDir))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(publicDir, r.URL.Path)
		if _, err := os.Stat(path); err == nil && r.URL.Path != "/" {
			mw.AddCacheHeaders("86400", fs).ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(publicDir, "index.html"))
	}))
}
