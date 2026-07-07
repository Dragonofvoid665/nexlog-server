-- 0001_init.up.sql
-- Initial schema for NexLog (PostgreSQL)

CREATE TABLE IF NOT EXISTS site (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    company_name TEXT NOT NULL DEFAULT 'NexLog Global',
    tagline TEXT NOT NULL DEFAULT 'Connecting the World, Delivering Excellence'
);

CREATE TABLE IF NOT EXISTS admin_password (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS hero_slides (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    subtitle TEXT NOT NULL,
    image TEXT NOT NULL,
    button_text TEXT NOT NULL DEFAULT 'Explore',
    button_link TEXT NOT NULL DEFAULT '#contact',
    sort_order INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS stats (
    id SERIAL PRIMARY KEY,
    icon TEXT NOT NULL,
    value TEXT NOT NULL,
    label TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS about (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    title TEXT NOT NULL DEFAULT 'About Us',
    description TEXT NOT NULL DEFAULT '',
    image TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS services (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    icon TEXT NOT NULL,
    image TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS news (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    date TEXT NOT NULL,
    excerpt TEXT NOT NULL,
    image TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS partners (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    logo TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS contact (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    address TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    whatsapp TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS languages (
    code TEXT PRIMARY KEY,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    flag TEXT NOT NULL,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sidepanel (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    config JSONB NOT NULL DEFAULT '{}'::jsonb
);

-- Indexes for ordering (sort_order is queried with ORDER BY on every read)
CREATE INDEX IF NOT EXISTS idx_hero_slides_sort ON hero_slides (sort_order);
CREATE INDEX IF NOT EXISTS idx_stats_sort ON stats (sort_order);
CREATE INDEX IF NOT EXISTS idx_services_sort ON services (sort_order);
CREATE INDEX IF NOT EXISTS idx_news_sort ON news (sort_order);
CREATE INDEX IF NOT EXISTS idx_partners_sort ON partners (sort_order);
