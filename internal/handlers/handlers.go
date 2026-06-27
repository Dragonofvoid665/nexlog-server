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

	"nexlog/internal/cache"
	"nexlog/internal/db"
	mw "nexlog/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	db        *db.DB
	cache     *cache.Cache
	jwtSecret string
	uploadDir string
}

func New(database *db.DB, c *cache.Cache, jwtSecret, uploadDir string) *Handler {
	return &Handler{db: database, cache: c, jwtSecret: jwtSecret, uploadDir: uploadDir}
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func readBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func cached[T any](h *Handler, key string, fetch func() (T, error)) (T, error) {
	if v, ok := h.cache.Get(key); ok {
		return v.(T), nil
	}
	val, err := fetch()
	if err == nil {
		h.cache.Set(key, val)
	}
	return val, err
}

// ─── Public: /api/content ─────────────────────────────────────────────────────

func (h *Handler) Content(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	site, _ := cached(h, "site", h.db.GetSite)
	slides, _ := cached(h, "hero", h.db.GetHeroSlides)
	stats, _ := cached(h, "stats", h.db.GetStats)
	about, _ := cached(h, "about", h.db.GetAbout)
	services, _ := cached(h, "services", h.db.GetServices)
	news, _ := cached(h, "news", h.db.GetNews)
	partners, _ := cached(h, "partners", h.db.GetPartners)
	contact, _ := cached(h, "contact", h.db.GetContact)

	writeJSON(w, 200, map[string]interface{}{
		"site":     site,
		"hero":     map[string]interface{}{"slides": slides},
		"stats":    stats,
		"about":    about,
		"services": services,
		"news":     news,
		"partners": partners,
		"contact":  contact,
	})
}

// ─── Public: /api/languages ───────────────────────────────────────────────────

func (h *Handler) Languages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if v, ok := h.cache.Get("languages"); ok {
		writeJSON(w, 200, v)
		return
	}
	langs, err := h.db.GetLanguages()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	h.cache.Set("languages", langs)
	writeJSON(w, 200, langs)
}

// ─── Public: /api/sidepanel ───────────────────────────────────────────────────

func (h *Handler) SidePanel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if v, ok := h.cache.Get("sidepanel"); ok {
		writeJSON(w, 200, v)
		return
	}
	cfg, err := h.db.GetSidePanel()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	h.cache.Set("sidepanel", cfg)
	writeJSON(w, 200, cfg)
}

// ─── Admin: Login ─────────────────────────────────────────────────────────────

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
	hash, err := h.db.GetAdminPasswordHash()
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		writeJSON(w, 401, map[string]string{"error": "Invalid password"})
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"admin": true,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(h.jwtSecret))
	writeJSON(w, 200, map[string]string{"token": tokenStr})
}

// ─── Admin: Change Password ───────────────────────────────────────────────────

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
	h.db.UpdateAdminPassword(string(hash))
	writeJSON(w, 200, map[string]string{"success": "true"})
}

// ─── Admin: Upload ────────────────────────────────────────────────────────────

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

// ─── Admin: CRUD helpers ──────────────────────────────────────────────────────

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

func (h *Handler) Site(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v db.Site
		readBody(r, &v)
		return h.db.UpdateSite(v)
	})
}

func (h *Handler) Hero(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var body struct {
			Slides []db.HeroSlide `json:"slides"`
		}
		readBody(r, &body)
		return h.db.UpdateHeroSlides(body.Slides)
	})
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v []db.Stat
		readBody(r, &v)
		return h.db.UpdateStats(v)
	})
}

func (h *Handler) About(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v db.About
		readBody(r, &v)
		return h.db.UpdateAbout(v)
	})
}

func (h *Handler) Services(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v []db.Service
		readBody(r, &v)
		return h.db.UpdateServices(v)
	})
}

func (h *Handler) News(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v []db.NewsItem
		readBody(r, &v)
		return h.db.UpdateNews(v)
	})
}

func (h *Handler) Partners(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v []db.Partner
		readBody(r, &v)
		return h.db.UpdatePartners(v)
	})
}

func (h *Handler) Contact(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v db.Contact
		readBody(r, &v)
		return h.db.UpdateContact(v)
	})
}

func (h *Handler) AdminLanguages(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v map[string]interface{}
		readBody(r, &v)
		return h.db.UpdateLanguages(v)
	})
}

func (h *Handler) AdminSidePanel(w http.ResponseWriter, r *http.Request) {
	h.adminPUT(w, r, func() error {
		var v map[string]interface{}
		readBody(r, &v)
		return h.db.UpdateSidePanel(v)
	})
}

// ─── Router setup ─────────────────────────────────────────────────────────────

func (h *Handler) RegisterRoutes(mux *http.ServeMux, publicDir string) {
	auth := mw.Auth(h.jwtSecret)

	// Public API
	mux.HandleFunc("/api/content", h.Content)
	mux.HandleFunc("/api/languages", h.Languages)
	mux.HandleFunc("/api/sidepanel", h.SidePanel)

	// Admin auth
	mux.HandleFunc("/api/admin/login", h.Login)

	// Admin protected
	mux.Handle("/api/admin/change-password", auth(http.HandlerFunc(h.ChangePassword)))
	mux.Handle("/api/admin/upload", auth(http.HandlerFunc(h.Upload)))
	mux.Handle("/api/admin/site", auth(http.HandlerFunc(h.Site)))
	mux.Handle("/api/admin/hero", auth(http.HandlerFunc(h.Hero)))
	mux.Handle("/api/admin/stats", auth(http.HandlerFunc(h.Stats)))
	mux.Handle("/api/admin/about", auth(http.HandlerFunc(h.About)))
	mux.Handle("/api/admin/services", auth(http.HandlerFunc(h.Services)))
	mux.Handle("/api/admin/news", auth(http.HandlerFunc(h.News)))
	mux.Handle("/api/admin/partners", auth(http.HandlerFunc(h.Partners)))
	mux.Handle("/api/admin/contact", auth(http.HandlerFunc(h.Contact)))
	mux.Handle("/api/admin/languages", auth(http.HandlerFunc(h.AdminLanguages)))
	mux.Handle("/api/admin/sidepanel", auth(http.HandlerFunc(h.AdminSidePanel)))

	// Static uploads with caching
	uploadFS := http.StripPrefix("/uploads/", http.FileServer(http.Dir(h.uploadDir)))
	mux.Handle("/uploads/", addCacheHeaders("604800", uploadFS)) // 7 days

	// Static public files with caching
	staticFS := http.FileServer(http.Dir(publicDir))
	mux.Handle("/admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(publicDir, "admin.html"))
	}))

	// SPA fallback
	mux.Handle("/", addCacheHeaders("86400", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if file exists
		path := filepath.Join(publicDir, r.URL.Path)
		if _, err := os.Stat(path); err == nil && r.URL.Path != "/" {
			staticFS.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(publicDir, "index.html"))
	})))
}

func addCacheHeaders(maxAge string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age="+maxAge)
		next.ServeHTTP(w, r)
	})
}
