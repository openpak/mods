package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
)

// InsertWithPackage records a pending mod and its zip bytes in one transaction.
func (s *Store) InsertWithPackage(ctx context.Context, m *Mod, data []byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `INSERT INTO mods (`+cols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'pending','',$13,now(),NULL,NULL)`,
		m.ID, m.Slug, m.TitleID, m.Name, m.Version, m.Author, m.Licence, m.SourceURL, m.Layout, m.Summary, m.SHA256, m.Size, m.SubmittedBy); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO mod_packages (mod_id, data) VALUES ($1,$2)`, m.ID, data); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Package returns the zip bytes of a mod, ErrNotFound when there are none.
func (s *Store) Package(ctx context.Context, id string) ([]byte, error) {
	var b []byte
	err := s.pool.QueryRow(ctx, `SELECT data FROM mod_packages WHERE mod_id=$1`, id).Scan(&b)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return b, err
}

// ImportPackages copies every <mod id>.zip in dir whose mod exists and has no package yet into
// the table. Files are never deleted. Returns counts only.
func (s *Store) ImportPackages(ctx context.Context, dir string) (imported, skipped int, err error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	for _, e := range ents {
		id, ok := strings.CutSuffix(e.Name(), ".zip")
		if !ok || e.IsDir() {
			skipped++
			continue
		}
		var have bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM mod_packages WHERE mod_id::text=$1)`, id).Scan(&have); err != nil {
			return imported, skipped, err
		}
		if have {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil || len(data) == 0 {
			skipped++
			continue
		}
		tag, err := s.pool.Exec(ctx, `INSERT INTO mod_packages (mod_id, data) SELECT id, $2 FROM mods WHERE id::text=$1 ON CONFLICT DO NOTHING`, id, data)
		if err != nil {
			return imported, skipped, err
		}
		if tag.RowsAffected() == 0 {
			skipped++
		}
		imported += int(tag.RowsAffected())
	}
	return imported, skipped, nil
}
