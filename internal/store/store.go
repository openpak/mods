// Package store is the catalogue and workflow over Postgres.
package store

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

var ErrNotFound = errors.New("store: not found")

type Mod struct {
	ID, Slug, TitleID, Name, Version, Author, Licence, SourceURL, Layout, Summary, SHA256 string
	Size                                                                                  int64
	Status, Reason, SubmittedBy                                                           string
	CreatedAt                                                                             time.Time
	PublishedAt, ReplacedAt                                                               *time.Time
}

// Downloadable: a published package, or a replaced one within its 30-day grace (MOD-04).
func (m *Mod) Downloadable(now time.Time) bool {
	switch m.Status {
	case "published":
		return true
	case "replaced":
		return m.ReplacedAt != nil && now.Sub(*m.ReplacedAt) < 30*24*time.Hour
	}
	return false
}

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	for _, name := range names {
		var done bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)`, name).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		sql, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
			return err
		}
	}
	return nil
}

func NewID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

const cols = `id, slug, title_id, name, version, author, licence, source_url, layout, summary, sha256, size, status, reason, submitted_by, created_at, published_at, replaced_at`

func scan(row pgx.Row) (*Mod, error) {
	var m Mod
	err := row.Scan(&m.ID, &m.Slug, &m.TitleID, &m.Name, &m.Version, &m.Author, &m.Licence, &m.SourceURL, &m.Layout, &m.Summary,
		&m.SHA256, &m.Size, &m.Status, &m.Reason, &m.SubmittedBy, &m.CreatedAt, &m.PublishedAt, &m.ReplacedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &m, err
}

func (s *Store) Insert(ctx context.Context, m *Mod) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO mods (`+cols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'pending','',$13,now(),NULL,NULL)`,
		m.ID, m.Slug, m.TitleID, m.Name, m.Version, m.Author, m.Licence, m.SourceURL, m.Layout, m.Summary, m.SHA256, m.Size, m.SubmittedBy)
	return err
}

func (s *Store) Get(ctx context.Context, id string) (*Mod, error) {
	return scan(s.pool.QueryRow(ctx, `SELECT `+cols+` FROM mods WHERE id=$1`, id))
}

func (s *Store) list(ctx context.Context, where string, args ...any) ([]*Mod, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+cols+` FROM mods WHERE `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Mod
	for rows.Next() {
		m, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) Published(ctx context.Context, titleID string) ([]*Mod, error) {
	return s.list(ctx, `title_id=$1 AND status='published'`, titleID)
}

func (s *Store) Queue(ctx context.Context) ([]*Mod, error) { return s.list(ctx, `status='pending'`) }

// Publish makes a pending mod live and retires the version it replaces (MOD-04).
func (s *Store) Publish(ctx context.Context, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	m, err := scan(tx.QueryRow(ctx, `SELECT `+cols+` FROM mods WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return err
	}
	if m.Status != "pending" {
		return fmt.Errorf("store: %s is %s, not pending", id, m.Status)
	}
	if _, err := tx.Exec(ctx, `UPDATE mods SET status='replaced', replaced_at=now() WHERE title_id=$1 AND slug=$2 AND status='published'`, m.TitleID, m.Slug); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE mods SET status='published', published_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Reject(ctx context.Context, id, reason string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE mods SET status='rejected', reason=$2 WHERE id=$1 AND status='pending'`, id, reason)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// Takedown removes every version of the mod at once, grace period included (MOD-04).
func (s *Store) Takedown(ctx context.Context, id, reason string) error {
	m, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE mods SET status='taken_down', reason=$3 WHERE title_id=$1 AND slug=$2 AND status IN ('pending','published','replaced')`, m.TitleID, m.Slug, reason)
	return err
}

func (s *Store) Favourites(ctx context.Context, accountID string) ([]*Mod, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+cols+` FROM mods WHERE id IN (SELECT mod_id FROM favourites WHERE account_id=$1) ORDER BY created_at DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Mod
	for rows.Next() {
		m, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) AddFavourite(ctx context.Context, accountID, modID string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO favourites (account_id, mod_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, accountID, modID)
	return err
}

func (s *Store) RemoveFavourite(ctx context.Context, accountID, modID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM favourites WHERE account_id=$1 AND mod_id=$2`, accountID, modID)
	return err
}

func (s *Store) Submitted(ctx context.Context, accountID string) ([]*Mod, error) {
	return s.list(ctx, `submitted_by=$1`, accountID)
}
