// Package repository implements the data access layer.
// All queries use parameterised statements ($1, $2…) — no string concatenation.
// Every public function accepts a context.Context so DB timeouts propagate.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// ─── Models ──────────────────────────────────────────────────────────────────

type Site struct {
	CompanyName string `json:"companyName"`
	Tagline     string `json:"tagline"`
}
type HeroSlide struct {
	Title      string `json:"title"`
	Subtitle   string `json:"subtitle"`
	Image      string `json:"image"`
	ButtonText string `json:"buttonText"`
	ButtonLink string `json:"buttonLink"`
}
type Stat struct {
	Icon  string `json:"icon"`
	Value string `json:"value"`
	Label string `json:"label"`
}
type About struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image"`
}
type Service struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Image       string `json:"image"`
}
type NewsItem struct {
	Title   string `json:"title"`
	Date    string `json:"date"`
	Excerpt string `json:"excerpt"`
	Image   string `json:"image"`
}
type Partner struct {
	Name string `json:"name"`
	Logo string `json:"logo"`
}
type Contact struct {
	Address  string `json:"address"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Whatsapp string `json:"whatsapp"`
}

// ─── Repo ─────────────────────────────────────────────────────────────────────

type Repo struct{ db *sql.DB }

func New(db *sql.DB) *Repo { return &Repo{db: db} }

// ─── Reads ────────────────────────────────────────────────────────────────────

func (r *Repo) GetSite(ctx context.Context) (Site, error) {
	var s Site
	err := r.db.QueryRowContext(ctx,
		`SELECT company_name, tagline FROM site WHERE id = 1`).
		Scan(&s.CompanyName, &s.Tagline)
	return s, err
}

func (r *Repo) GetHeroSlides(ctx context.Context) ([]HeroSlide, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT title, subtitle, image, button_text, button_link
		 FROM hero_slides ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HeroSlide
	for rows.Next() {
		var s HeroSlide
		if err := rows.Scan(&s.Title, &s.Subtitle, &s.Image, &s.ButtonText, &s.ButtonLink); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if out == nil {
		out = []HeroSlide{}
	}
	return out, rows.Err()
}

func (r *Repo) GetStats(ctx context.Context) ([]Stat, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT icon, value, label FROM stats ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Stat
	for rows.Next() {
		var s Stat
		if err := rows.Scan(&s.Icon, &s.Value, &s.Label); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if out == nil {
		out = []Stat{}
	}
	return out, rows.Err()
}

func (r *Repo) GetAbout(ctx context.Context) (About, error) {
	var a About
	err := r.db.QueryRowContext(ctx,
		`SELECT title, description, image FROM about WHERE id = 1`).
		Scan(&a.Title, &a.Description, &a.Image)
	return a, err
}

func (r *Repo) GetServices(ctx context.Context) ([]Service, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT title, description, icon, image FROM services ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Service
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.Title, &s.Description, &s.Icon, &s.Image); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if out == nil {
		out = []Service{}
	}
	return out, rows.Err()
}

func (r *Repo) GetNews(ctx context.Context) ([]NewsItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT title, date, excerpt, image FROM news ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NewsItem
	for rows.Next() {
		var n NewsItem
		if err := rows.Scan(&n.Title, &n.Date, &n.Excerpt, &n.Image); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if out == nil {
		out = []NewsItem{}
	}
	return out, rows.Err()
}

func (r *Repo) GetPartners(ctx context.Context) ([]Partner, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT name, logo FROM partners ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Partner
	for rows.Next() {
		var p Partner
		if err := rows.Scan(&p.Name, &p.Logo); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []Partner{}
	}
	return out, rows.Err()
}

func (r *Repo) GetContact(ctx context.Context) (Contact, error) {
	var c Contact
	err := r.db.QueryRowContext(ctx,
		`SELECT address, phone, email, whatsapp FROM contact WHERE id = 1`).
		Scan(&c.Address, &c.Phone, &c.Email, &c.Whatsapp)
	return c, err
}

func (r *Repo) GetLanguages(ctx context.Context) (map[string]interface{}, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT code, data, flag, name FROM languages`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]interface{}{}
	for rows.Next() {
		var code, flag, name string
		var data []byte
		if err := rows.Scan(&code, &data, &flag, &name); err != nil {
			return nil, err
		}
		var langData map[string]interface{}
		_ = json.Unmarshal(data, &langData)
		if langData == nil {
			langData = map[string]interface{}{}
		}
		langData["flag"] = flag
		langData["name"] = name
		result[code] = langData
	}
	return result, rows.Err()
}

func (r *Repo) GetSidePanel(ctx context.Context) (map[string]interface{}, error) {
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT config FROM sidepanel WHERE id = 1`).Scan(&raw)
	if err != nil {
		return map[string]interface{}{"enabled": false, "channels": []interface{}{}}, nil
	}
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	return out, nil
}

func (r *Repo) GetAdminPasswordHash(ctx context.Context) (string, error) {
	var hash string
	err := r.db.QueryRowContext(ctx, `SELECT hash FROM admin_password WHERE id = 1`).Scan(&hash)
	return hash, err
}

// ─── Writes ───────────────────────────────────────────────────────────────────

func (r *Repo) UpdateSite(ctx context.Context, s Site) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE site SET company_name = $1, tagline = $2 WHERE id = 1`,
		s.CompanyName, s.Tagline)
	return err
}

func (r *Repo) UpdateHeroSlides(ctx context.Context, slides []HeroSlide) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM hero_slides`); err != nil {
		return err
	}
	for i, s := range slides {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO hero_slides (title, subtitle, image, button_text, button_link, sort_order)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			s.Title, s.Subtitle, s.Image, s.ButtonText, s.ButtonLink, i); err != nil {
			return fmt.Errorf("insert slide %d: %w", i, err)
		}
	}
	return tx.Commit()
}

func (r *Repo) UpdateStats(ctx context.Context, stats []Stat) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.ExecContext(ctx, `DELETE FROM stats`)
	for i, s := range stats {
		tx.ExecContext(ctx,
			`INSERT INTO stats (icon, value, label, sort_order) VALUES ($1, $2, $3, $4)`,
			s.Icon, s.Value, s.Label, i)
	}
	return tx.Commit()
}

func (r *Repo) UpdateAbout(ctx context.Context, a About) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE about SET title = $1, description = $2, image = $3 WHERE id = 1`,
		a.Title, a.Description, a.Image)
	return err
}

func (r *Repo) UpdateServices(ctx context.Context, list []Service) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.ExecContext(ctx, `DELETE FROM services`)
	for i, s := range list {
		tx.ExecContext(ctx,
			`INSERT INTO services (title, description, icon, image, sort_order) VALUES ($1,$2,$3,$4,$5)`,
			s.Title, s.Description, s.Icon, s.Image, i)
	}
	return tx.Commit()
}

func (r *Repo) UpdateNews(ctx context.Context, list []NewsItem) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.ExecContext(ctx, `DELETE FROM news`)
	for i, n := range list {
		tx.ExecContext(ctx,
			`INSERT INTO news (title, date, excerpt, image, sort_order) VALUES ($1,$2,$3,$4,$5)`,
			n.Title, n.Date, n.Excerpt, n.Image, i)
	}
	return tx.Commit()
}

func (r *Repo) UpdatePartners(ctx context.Context, list []Partner) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.ExecContext(ctx, `DELETE FROM partners`)
	for i, p := range list {
		tx.ExecContext(ctx,
			`INSERT INTO partners (name, logo, sort_order) VALUES ($1,$2,$3)`,
			p.Name, p.Logo, i)
	}
	return tx.Commit()
}

func (r *Repo) UpdateContact(ctx context.Context, c Contact) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE contact SET address=$1, phone=$2, email=$3, whatsapp=$4 WHERE id=1`,
		c.Address, c.Phone, c.Email, c.Whatsapp)
	return err
}

func (r *Repo) UpdateLanguages(ctx context.Context, langs map[string]interface{}) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.ExecContext(ctx, `DELETE FROM languages`)
	for code, val := range langs {
		langMap, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		flag, _ := langMap["flag"].(string)
		name, _ := langMap["name"].(string)
		cp := make(map[string]interface{}, len(langMap))
		for k, v := range langMap {
			if k != "flag" && k != "name" {
				cp[k] = v
			}
		}
		data, _ := json.Marshal(cp)
		tx.ExecContext(ctx,
			`INSERT INTO languages (code, data, flag, name) VALUES ($1,$2,$3,$4)`,
			code, data, flag, name)
	}
	return tx.Commit()
}

func (r *Repo) UpdateSidePanel(ctx context.Context, cfg map[string]interface{}) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE sidepanel SET config = $1 WHERE id = 1`, data)
	return err
}

func (r *Repo) UpdateAdminPassword(ctx context.Context, hash string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE admin_password SET hash = $1 WHERE id = 1`, hash)
	return err
}
