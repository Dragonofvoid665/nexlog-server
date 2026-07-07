// Package migrate provides a minimal SQL migration runner.
// Applies numbered *.up.sql files in order, tracks applied versions in schema_migrations.
package migrate

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type migration struct {
	version int
	name    string
	path    string
}

// Run applies all pending *.up.sql migrations from dir, in numeric order.
func Run(db *sql.DB, dir string) error {
	if err := ensureMigrationsTable(db); err != nil {
		return fmt.Errorf("migrate: create schema_migrations: %w", err)
	}
	files, err := collectUpMigrations(dir)
	if err != nil {
		return fmt.Errorf("migrate: collect: %w", err)
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return fmt.Errorf("migrate: read applied: %w", err)
	}
	for _, m := range files {
		if applied[m.version] {
			continue
		}
		log.Printf("🔧 Applying migration %04d_%s...", m.version, m.name)
		if err := applyOne(db, m); err != nil {
			return fmt.Errorf("migrate: apply %s: %w", m.path, err)
		}
		log.Printf("✅ Migration %04d_%s applied", m.version, m.name)
	}
	return nil
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    BIGINT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	return err
}

func appliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func collectUpMigrations(dir string) ([]migration, error) {
	var out []migration
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".up.sql") {
			return err
		}
		base := filepath.Base(path)
		numStr := base
		if idx := strings.Index(base, "_"); idx > 0 {
			numStr = base[:idx]
		}
		version, convErr := strconv.Atoi(numStr)
		if convErr != nil {
			return fmt.Errorf("migration %q: filename must start with numeric version", base)
		}
		name := strings.TrimSuffix(strings.TrimPrefix(base, numStr+"_"), ".up.sql")
		out = append(out, migration{version: version, name: name, path: path})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func applyOne(db *sql.DB, m migration) error {
	content, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(string(content)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, m.version); err != nil {
		return err
	}
	return tx.Commit()
}
