// Package service implements business logic between handlers and repository.
// It also owns the "seed" logic that runs on first boot via migration.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"nexlog/internal/logger"
	"nexlog/internal/repository"
)

type Service struct {
	repo *repository.Repo
	db   *sql.DB
}

func New(repo *repository.Repo, db *sql.DB) *Service {
	return &Service{repo: repo, db: db}
}

// ─── Seed (runs once on first boot after migrations) ──────────────────────────

func (s *Service) SeedIfEmpty(ctx context.Context) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM site`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		s.MigrateLanguages(ctx) // always ensure new languages are added
		return nil
	}

	logger.Info("📦 First boot — seeding default data...")
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.ExecContext(ctx, `INSERT INTO site (id, company_name, tagline) VALUES (1, $1, $2)`,
		"NexLog Global", "Connecting the World, Delivering Excellence")

	// Generate random first-boot password hint in logs (admin must change it)
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	tx.ExecContext(ctx, `INSERT INTO admin_password (id, hash) VALUES (1, $1)`, string(hash))

	tx.ExecContext(ctx,
		`INSERT INTO hero_slides (title, subtitle, image, button_text, button_link, sort_order)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		"Global Logistics Solutions", "Fast, Reliable & Trusted Worldwide",
		"/uploads/placeholder.jpg", "Explore", "#contact", 0)

	stats := []struct{ icon, value, label string }{
		{"🚛", "150+", "Countries Served"}, {"📦", "50K+", "Shipments/Month"},
		{"👥", "200+", "Team Members"}, {"⭐", "98%", "Client Satisfaction"},
		{"🏆", "20+", "Years Experience"},
	}
	for i, st := range stats {
		tx.ExecContext(ctx,
			`INSERT INTO stats (icon, value, label, sort_order) VALUES ($1,$2,$3,$4)`,
			st.icon, st.value, st.label, i)
	}

	tx.ExecContext(ctx,
		`INSERT INTO about (id, title, description, image) VALUES (1,$1,$2,$3)`,
		"About Us",
		"We are a leading global logistics provider with over 20 years of experience connecting businesses worldwide with fast, reliable shipping solutions.",
		"/uploads/placeholder.jpg")

	tx.ExecContext(ctx,
		`INSERT INTO contact (id, address, phone, email, whatsapp) VALUES (1,$1,$2,$3,$4)`,
		"123 Logistics St, Global City", "+1234567890", "hello@nexlog.com", "+1234567890")

	for code, lang := range DefaultLanguages() {
		flag := lang["flag"]
		name := lang["name"]
		cp := make(map[string]string, len(lang))
		for k, v := range lang {
			if k != "flag" && k != "name" {
				cp[k] = v
			}
		}
		data, _ := json.Marshal(cp)
		tx.ExecContext(ctx,
			`INSERT INTO languages (code, data, flag, name) VALUES ($1,$2,$3,$4)`,
			code, data, flag, name)
	}

	panel := map[string]interface{}{
		"enabled": true, "position": "right", "label": "Contact Us", "color": "#C9922A",
		"channels": []map[string]interface{}{
			{"type": "phone", "label": "Phone", "icon": "📞", "linkPrefix": "tel:", "value": "+1234567890", "enabled": true},
			{"type": "whatsapp", "label": "WhatsApp", "icon": "💬", "linkPrefix": "https://wa.me/", "value": "1234567890", "enabled": true},
		},
	}
	panelJSON, _ := json.Marshal(panel)
	tx.ExecContext(ctx, `INSERT INTO sidepanel (id, config) VALUES (1, $1)`, panelJSON)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	logger.Info("✅ Seed complete")
	return nil
}

// MigrateLanguages ensures all languages from DefaultLanguages() exist (upsert-style).
func (s *Service) MigrateLanguages(ctx context.Context) {
	for code, lang := range DefaultLanguages() {
		var count int
		s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM languages WHERE code = $1`, code).Scan(&count)
		if count > 0 {
			continue
		}
		flag := lang["flag"]
		name := lang["name"]
		cp := make(map[string]string, len(lang))
		for k, v := range lang {
			if k != "flag" && k != "name" {
				cp[k] = v
			}
		}
		data, _ := json.Marshal(cp)
		s.db.ExecContext(ctx,
			`INSERT INTO languages (code, data, flag, name) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			code, data, flag, name)
		logger.Info("🌐 Added missing language", "code", code)
	}
}

// ─── Delegate to repo (adds request timeout) ─────────────────────────────────

const dbTimeout = 8 * time.Second

func (s *Service) GetSite(ctx context.Context) (repository.Site, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.GetSite(ctx)
}
func (s *Service) GetHeroSlides(ctx context.Context) ([]repository.HeroSlide, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.GetHeroSlides(ctx)
}
func (s *Service) GetStats(ctx context.Context) ([]repository.Stat, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.GetStats(ctx)
}
func (s *Service) GetAbout(ctx context.Context) (repository.About, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.GetAbout(ctx)
}
func (s *Service) GetServices(ctx context.Context) ([]repository.Service, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.GetServices(ctx)
}
func (s *Service) GetNews(ctx context.Context) ([]repository.NewsItem, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.GetNews(ctx)
}
func (s *Service) GetPartners(ctx context.Context) ([]repository.Partner, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.GetPartners(ctx)
}
func (s *Service) GetContact(ctx context.Context) (repository.Contact, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.GetContact(ctx)
}
func (s *Service) GetLanguages(ctx context.Context) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.GetLanguages(ctx)
}
func (s *Service) GetSidePanel(ctx context.Context) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.GetSidePanel(ctx)
}
func (s *Service) GetAdminPasswordHash(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.GetAdminPasswordHash(ctx)
}

func (s *Service) UpdateSite(ctx context.Context, v repository.Site) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.UpdateSite(ctx, v)
}
func (s *Service) UpdateHeroSlides(ctx context.Context, v []repository.HeroSlide) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.UpdateHeroSlides(ctx, v)
}
func (s *Service) UpdateStats(ctx context.Context, v []repository.Stat) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.UpdateStats(ctx, v)
}
func (s *Service) UpdateAbout(ctx context.Context, v repository.About) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.UpdateAbout(ctx, v)
}
func (s *Service) UpdateServices(ctx context.Context, v []repository.Service) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.UpdateServices(ctx, v)
}
func (s *Service) UpdateNews(ctx context.Context, v []repository.NewsItem) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.UpdateNews(ctx, v)
}
func (s *Service) UpdatePartners(ctx context.Context, v []repository.Partner) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.UpdatePartners(ctx, v)
}
func (s *Service) UpdateContact(ctx context.Context, v repository.Contact) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.UpdateContact(ctx, v)
}
func (s *Service) UpdateLanguages(ctx context.Context, v map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.UpdateLanguages(ctx, v)
}
func (s *Service) UpdateSidePanel(ctx context.Context, v map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.UpdateSidePanel(ctx, v)
}
func (s *Service) UpdateAdminPassword(ctx context.Context, hash string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return s.repo.UpdateAdminPassword(ctx, hash)
}
