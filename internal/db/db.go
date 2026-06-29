package db

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

type DB struct {
	*sql.DB
}

func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	dbFile := filepath.Join(dataDir, "nexlog.db")
	db, err := sql.Open("sqlite3", dbFile+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite: single writer
	db.SetMaxIdleConns(10)
	return &DB{db}, nil
}

func (db *DB) Init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS site (
		id INTEGER PRIMARY KEY CHECK (id=1),
		companyName TEXT NOT NULL DEFAULT 'NexLog Global',
		tagline TEXT NOT NULL DEFAULT 'Connecting the World, Delivering Excellence'
	);
	CREATE TABLE IF NOT EXISTS admin_password (
		id INTEGER PRIMARY KEY CHECK (id=1),
		hash TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS hero_slides (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		subtitle TEXT NOT NULL,
		image TEXT NOT NULL,
		buttonText TEXT NOT NULL DEFAULT 'Explore',
		buttonLink TEXT NOT NULL DEFAULT '#contact',
		sort_order INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS stats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		icon TEXT NOT NULL,
		value TEXT NOT NULL,
		label TEXT NOT NULL,
		sort_order INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS about (
		id INTEGER PRIMARY KEY CHECK (id=1),
		title TEXT NOT NULL DEFAULT 'About Us',
		description TEXT NOT NULL DEFAULT '',
		image TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS services (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		icon TEXT NOT NULL,
		image TEXT NOT NULL,
		sort_order INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS news (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		date TEXT NOT NULL,
		excerpt TEXT NOT NULL,
		image TEXT NOT NULL,
		sort_order INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS partners (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		logo TEXT NOT NULL,
		sort_order INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS contact (
		id INTEGER PRIMARY KEY CHECK (id=1),
		address TEXT NOT NULL DEFAULT '',
		phone TEXT NOT NULL DEFAULT '',
		email TEXT NOT NULL DEFAULT '',
		whatsapp TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS languages (
		code TEXT PRIMARY KEY,
		data TEXT NOT NULL,
		flag TEXT NOT NULL,
		name TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS sidepanel (
		id INTEGER PRIMARY KEY CHECK (id=1),
		config TEXT NOT NULL
	);
	`
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	if err := db.seed(); err != nil {
		return err
	}
	return db.migrateLanguages()
}

func (db *DB) migrateLanguages() error {
	newLangs := defaultLanguages()
	for code, lang := range newLangs {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM languages WHERE code=?", code).Scan(&count)
		if count > 0 {
			continue // уже есть — не трогаем
		}
		log.Printf("🌐 Adding missing language: %s (%s)", code, lang["name"])
		flag := lang["flag"]
		name := lang["name"]
		delete(lang, "flag")
		delete(lang, "name")
		data, _ := json.Marshal(lang)
		db.Exec("INSERT INTO languages (code,data,flag,name) VALUES (?,?,?,?)", code, string(data), flag, name)
	}
	return nil
}

func defaultLanguages() map[string]map[string]string {
	return map[string]map[string]string{
		"en": {
			"flag": "🇬🇧", "name": "English",
			"nav_home": "Home", "nav_about": "About", "nav_services": "Services", "nav_news": "News", "nav_quote": "Get Quote",
			"eyebrow_who": "Who We Are", "eyebrow_offer": "What We Offer", "eyebrow_updates": "Latest Updates",
			"eyebrow_partners": "Trusted By", "eyebrow_touch": "Get In Touch",
			"sec_about": "About Us", "sec_services": "Our Services", "sec_news": "Our News", "sec_partners": "Our Key Partners",
			"cta_title": "Ready to Ship With Us?", "cta_sub": "Contact our team for a custom logistics solution.",
			"cta_btn": "Request a Quote →", "about_btn": "Contact Us →", "slide_tag": "Global Logistics",
			"footer_services": "Services", "footer_company": "Company", "footer_team": "Team",
			"footer_careers": "Careers", "footer_news": "News", "footer_contact": "Contact", "years": "Years",
		},
		"ru": {
			"flag": "🇷🇺", "name": "Русский",
			"nav_home": "Главная", "nav_about": "О нас", "nav_services": "Услуги", "nav_news": "Новости", "nav_quote": "Запрос",
			"eyebrow_who": "Кто мы", "eyebrow_offer": "Что мы предлагаем", "eyebrow_updates": "Последние новости",
			"eyebrow_partners": "Нам доверяют", "eyebrow_touch": "Связаться с нами",
			"sec_about": "О компании", "sec_services": "Наши услуги", "sec_news": "Новости", "sec_partners": "Наши партнёры",
			"cta_title": "Готовы отправить груз?", "cta_sub": "Свяжитесь с нашей командой для решения логистических задач.",
			"cta_btn": "Запросить цену →", "about_btn": "Связаться →", "slide_tag": "Глобальная логистика",
			"footer_services": "Услуги", "footer_company": "Компания", "footer_team": "Команда",
			"footer_careers": "Карьера", "footer_news": "Новости", "footer_contact": "Контакты", "years": "Лет",
		},
		"ja": {
			"flag": "🇯🇵", "name": "日本語",
			"nav_home": "ホーム", "nav_about": "会社概要", "nav_services": "サービス", "nav_news": "ニュース", "nav_quote": "見積もり",
			"eyebrow_who": "私たちについて", "eyebrow_offer": "サービス内容", "eyebrow_updates": "最新ニュース",
			"eyebrow_partners": "信頼されています", "eyebrow_touch": "お問い合わせ",
			"sec_about": "会社概要", "sec_services": "サービス", "sec_news": "ニュース", "sec_partners": "主要パートナー",
			"cta_title": "一緒に出荷しませんか？", "cta_sub": "お客様のニーズに合った物流ソリューションについて、チームにお問い合わせください。",
			"cta_btn": "見積もりを依頼する →", "about_btn": "お問い合わせ →", "slide_tag": "グローバル物流",
			"footer_services": "サービス", "footer_company": "会社", "footer_team": "チーム",
			"footer_careers": "採用情報", "footer_news": "ニュース", "footer_contact": "お問い合わせ", "years": "年",
		},
		"tk": {
			"flag": "🇹🇲", "name": "Türkmen",
			"nav_home": "Baş sahypa", "nav_about": "Biz hakda", "nav_services": "Hyzmatlar", "nav_news": "Habarlar", "nav_quote": "Teklip al",
			"eyebrow_who": "Biz kimler", "eyebrow_offer": "Näme hödürleýäris", "eyebrow_updates": "Soňky habarlar",
			"eyebrow_partners": "Bize ynanýarlar", "eyebrow_touch": "Habarlaşyň",
			"sec_about": "Biz hakda", "sec_services": "Hyzmatlarymyz", "sec_news": "Habarlar", "sec_partners": "Esasy hyzmatdaşlar",
			"cta_title": "Biz bilen ibermäge taýynmy?", "cta_sub": "Öz zerurlyklaryňyza laýyk logistika çözgüdi üçin toparymyz bilen habarlaşyň.",
			"cta_btn": "Teklip soramak →", "about_btn": "Habarlaşmak →", "slide_tag": "Global logistika",
			"footer_services": "Hyzmatlar", "footer_company": "Kompaniýa", "footer_team": "Topar",
			"footer_careers": "Iş orunlary", "footer_news": "Habarlar", "footer_contact": "Habarlaşmak", "years": "Ýyl",
		},
		"zh": {
			"flag": "🇨🇳", "name": "中文",
			"nav_home": "首页", "nav_about": "关于我们", "nav_services": "服务", "nav_news": "新闻", "nav_quote": "获取报价",
			"eyebrow_who": "我们是谁", "eyebrow_offer": "我们提供什么", "eyebrow_updates": "最新动态",
			"eyebrow_partners": "合作伙伴", "eyebrow_touch": "联系我们",
			"sec_about": "关于我们", "sec_services": "我们的服务", "sec_news": "新闻资讯", "sec_partners": "主要合作伙伴",
			"cta_title": "准备好与我们合作了吗？", "cta_sub": "联系我们的团队，获取专属物流解决方案。",
			"cta_btn": "申请报价 →", "about_btn": "联系我们 →", "slide_tag": "全球物流",
			"footer_services": "服务", "footer_company": "公司", "footer_team": "团队",
			"footer_careers": "招聘", "footer_news": "新闻", "footer_contact": "联系我们", "years": "年",
		},
	}
}

func (db *DB) seed() error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM site").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	log.Println("📦 First init, seeding default data...")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec("INSERT INTO site (id, companyName, tagline) VALUES (1,'NexLog Global','Connecting the World, Delivering Excellence')")

	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	tx.Exec("INSERT INTO admin_password (id, hash) VALUES (1, ?)", string(hash))

	tx.Exec("INSERT INTO hero_slides (title,subtitle,image,buttonText,buttonLink,sort_order) VALUES (?,?,?,?,?,?)",
		"Global Logistics Solutions", "Fast, Reliable & Trusted Worldwide", "/uploads/placeholder.jpg", "Explore", "#contact", 0)

	defaultStats := [][]string{
		{"🚛", "150+", "Countries Served"},
		{"📦", "50K+", "Shipments/Month"},
		{"👥", "200+", "Team Members"},
		{"⭐", "98%", "Client Satisfaction"},
		{"🏆", "20+", "Years Experience"},
	}
	for i, s := range defaultStats {
		tx.Exec("INSERT INTO stats (icon,value,label,sort_order) VALUES (?,?,?,?)", s[0], s[1], s[2], i)
	}

	tx.Exec("INSERT INTO about (id,title,description,image) VALUES (1,'About Us','We are a leading global logistics provider with over 20 years of experience.','/uploads/placeholder.jpg')")

	tx.Exec("INSERT INTO contact (id,address,phone,email,whatsapp) VALUES (1,'123 Logistics St, Global City','+1234567890','hello@nexlog.com','+1234567890')")

	for code, lang := range defaultLanguages() {
		flag := lang["flag"]
		name := lang["name"]
		delete(lang, "flag")
		delete(lang, "name")
		data, _ := json.Marshal(lang)
		tx.Exec("INSERT INTO languages (code,data,flag,name) VALUES (?,?,?,?)", code, string(data), flag, name)
	}

	defaultPanel := map[string]interface{}{
		"enabled": true, "position": "right", "label": "Contact Us", "color": "#C9922A",
		"channels": []map[string]interface{}{
			{"type": "phone", "label": "Phone", "icon": "📞", "linkPrefix": "tel:", "value": "+1234567890", "enabled": true},
			{"type": "whatsapp", "label": "WhatsApp", "icon": "💬", "linkPrefix": "https://wa.me/", "value": "1234567890", "enabled": true},
		},
	}
	panelJSON, _ := json.Marshal(defaultPanel)
	tx.Exec("INSERT INTO sidepanel (id,config) VALUES (1,?)", string(panelJSON))

	log.Println("✅ Seed complete")
	return tx.Commit()
}

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

// ─── Read helpers ─────────────────────────────────────────────────────────────

func (db *DB) GetSite() (Site, error) {
	var s Site
	err := db.QueryRow("SELECT companyName,tagline FROM site WHERE id=1").Scan(&s.CompanyName, &s.Tagline)
	return s, err
}

func (db *DB) GetHeroSlides() ([]HeroSlide, error) {
	rows, err := db.Query("SELECT title,subtitle,image,buttonText,buttonLink FROM hero_slides ORDER BY sort_order")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var slides []HeroSlide
	for rows.Next() {
		var s HeroSlide
		rows.Scan(&s.Title, &s.Subtitle, &s.Image, &s.ButtonText, &s.ButtonLink)
		slides = append(slides, s)
	}
	if slides == nil {
		slides = []HeroSlide{}
	}
	return slides, nil
}

func (db *DB) GetStats() ([]Stat, error) {
	rows, err := db.Query("SELECT icon,value,label FROM stats ORDER BY sort_order")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []Stat
	for rows.Next() {
		var s Stat
		rows.Scan(&s.Icon, &s.Value, &s.Label)
		stats = append(stats, s)
	}
	if stats == nil {
		stats = []Stat{}
	}
	return stats, nil
}

func (db *DB) GetAbout() (About, error) {
	var a About
	err := db.QueryRow("SELECT title,description,image FROM about WHERE id=1").Scan(&a.Title, &a.Description, &a.Image)
	return a, err
}

func (db *DB) GetServices() ([]Service, error) {
	rows, err := db.Query("SELECT title,description,icon,image FROM services ORDER BY sort_order")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Service
	for rows.Next() {
		var s Service
		rows.Scan(&s.Title, &s.Description, &s.Icon, &s.Image)
		list = append(list, s)
	}
	if list == nil {
		list = []Service{}
	}
	return list, nil
}

func (db *DB) GetNews() ([]NewsItem, error) {
	rows, err := db.Query("SELECT title,date,excerpt,image FROM news ORDER BY sort_order")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []NewsItem
	for rows.Next() {
		var n NewsItem
		rows.Scan(&n.Title, &n.Date, &n.Excerpt, &n.Image)
		list = append(list, n)
	}
	if list == nil {
		list = []NewsItem{}
	}
	return list, nil
}

func (db *DB) GetPartners() ([]Partner, error) {
	rows, err := db.Query("SELECT name,logo FROM partners ORDER BY sort_order")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Partner
	for rows.Next() {
		var p Partner
		rows.Scan(&p.Name, &p.Logo)
		list = append(list, p)
	}
	if list == nil {
		list = []Partner{}
	}
	return list, nil
}

func (db *DB) GetContact() (Contact, error) {
	var c Contact
	err := db.QueryRow("SELECT address,phone,email,whatsapp FROM contact WHERE id=1").Scan(&c.Address, &c.Phone, &c.Email, &c.Whatsapp)
	return c, err
}

func (db *DB) GetLanguages() (map[string]interface{}, error) {
	rows, err := db.Query("SELECT code,data,flag,name FROM languages")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]interface{}{}
	for rows.Next() {
		var code, data, flag, name string
		rows.Scan(&code, &data, &flag, &name)
		var langData map[string]interface{}
		json.Unmarshal([]byte(data), &langData)
		if langData == nil {
			langData = map[string]interface{}{}
		}
		langData["flag"] = flag
		langData["name"] = name
		result[code] = langData
	}
	return result, nil
}

func (db *DB) GetSidePanel() (map[string]interface{}, error) {
	var config string
	err := db.QueryRow("SELECT config FROM sidepanel WHERE id=1").Scan(&config)
	if err != nil {
		return map[string]interface{}{"enabled": false, "channels": []interface{}{}}, nil
	}
	var result map[string]interface{}
	json.Unmarshal([]byte(config), &result)
	return result, nil
}

func (db *DB) GetAdminPasswordHash() (string, error) {
	var hash string
	err := db.QueryRow("SELECT hash FROM admin_password WHERE id=1").Scan(&hash)
	return hash, err
}

// ─── Write helpers ────────────────────────────────────────────────────────────

func (db *DB) UpdateSite(s Site) error {
	_, err := db.Exec("UPDATE site SET companyName=?,tagline=? WHERE id=1", s.CompanyName, s.Tagline)
	return err
}

func (db *DB) UpdateHeroSlides(slides []HeroSlide) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec("DELETE FROM hero_slides")
	for i, s := range slides {
		tx.Exec("INSERT INTO hero_slides (title,subtitle,image,buttonText,buttonLink,sort_order) VALUES (?,?,?,?,?,?)",
			s.Title, s.Subtitle, s.Image, s.ButtonText, s.ButtonLink, i)
	}
	return tx.Commit()
}

func (db *DB) UpdateStats(stats []Stat) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec("DELETE FROM stats")
	for i, s := range stats {
		tx.Exec("INSERT INTO stats (icon,value,label,sort_order) VALUES (?,?,?,?)", s.Icon, s.Value, s.Label, i)
	}
	return tx.Commit()
}

func (db *DB) UpdateAbout(a About) error {
	_, err := db.Exec("UPDATE about SET title=?,description=?,image=? WHERE id=1", a.Title, a.Description, a.Image)
	return err
}

func (db *DB) UpdateServices(list []Service) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec("DELETE FROM services")
	for i, s := range list {
		tx.Exec("INSERT INTO services (title,description,icon,image,sort_order) VALUES (?,?,?,?,?)",
			s.Title, s.Description, s.Icon, s.Image, i)
	}
	return tx.Commit()
}

func (db *DB) UpdateNews(list []NewsItem) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec("DELETE FROM news")
	for i, n := range list {
		tx.Exec("INSERT INTO news (title,date,excerpt,image,sort_order) VALUES (?,?,?,?,?)",
			n.Title, n.Date, n.Excerpt, n.Image, i)
	}
	return tx.Commit()
}

func (db *DB) UpdatePartners(list []Partner) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec("DELETE FROM partners")
	for i, p := range list {
		tx.Exec("INSERT INTO partners (name,logo,sort_order) VALUES (?,?,?)", p.Name, p.Logo, i)
	}
	return tx.Commit()
}

func (db *DB) UpdateContact(c Contact) error {
	_, err := db.Exec("UPDATE contact SET address=?,phone=?,email=?,whatsapp=? WHERE id=1",
		c.Address, c.Phone, c.Email, c.Whatsapp)
	return err
}

func (db *DB) UpdateLanguages(langs map[string]interface{}) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec("DELETE FROM languages")
	for code, val := range langs {
		langMap, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		flag, _ := langMap["flag"].(string)
		name, _ := langMap["name"].(string)
		delete(langMap, "flag")
		delete(langMap, "name")
		data, _ := json.Marshal(langMap)
		tx.Exec("INSERT INTO languages (code,data,flag,name) VALUES (?,?,?,?)", code, string(data), flag, name)
	}
	return tx.Commit()
}

func (db *DB) UpdateSidePanel(cfg map[string]interface{}) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE sidepanel SET config=? WHERE id=1", string(data))
	return err
}

func (db *DB) UpdateAdminPassword(hash string) error {
	_, err := db.Exec("UPDATE admin_password SET hash=? WHERE id=1", hash)
	return err
}
