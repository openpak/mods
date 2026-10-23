package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openpak/mods/internal/store"
)

// TestEraseAccount runs the erasure endpoint against a real Postgres. Set
// MODS_TEST_DATABASE_URL to a throwaway database; the test drops its tables.
func TestEraseAccount(t *testing.T) {
	dsn := os.Getenv("MODS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MODS_TEST_DATABASE_URL not set; Postgres tests skipped")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS favourites, mods, schema_migrations`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	data := t.TempDir()
	const key = "0123456789abcdef"
	srv := New(st, data, "http://website.invalid", key)

	add := func(slug, by, status string) string {
		m := &store.Mod{ID: store.NewID(), Slug: slug, TitleID: "0100000000010000", Name: slug, Version: "1", Author: "Real Name",
			Licence: "MIT", SourceURL: "https://example.org", Layout: "romfs", SHA256: "00", Size: 1, SubmittedBy: by}
		if err := st.Insert(ctx, m); err != nil {
			t.Fatal(err)
		}
		if status != "pending" {
			if _, err := pool.Exec(ctx, `UPDATE mods SET status=$2 WHERE id=$1`, m.ID, status); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(data, m.ID+".zip"), []byte("zip"), 0o644); err != nil {
			t.Fatal(err)
		}
		return m.ID
	}
	gone := "acct-gone"
	pub := add("pub", gone, "published")
	old := add("old", gone, "replaced")
	pending := add("pending", gone, "pending")
	rejected := add("rejected", gone, "rejected")
	down := add("down", gone, "taken_down")
	other := add("other", "acct-stays", "published")
	for _, fav := range []struct{ acct, mod string }{{gone, other}, {gone, pub}, {"acct-stays", pub}, {"acct-stays", other}} {
		if err := st.AddFavourite(ctx, fav.acct, fav.mod); err != nil {
			t.Fatal(err)
		}
	}

	erase := func(auth string) (int, store.Erasure) {
		req := httptest.NewRequest(http.MethodDelete, "/internal/accounts/"+gone+"/mods", nil)
		req.Header.Set("Authorization", auth)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		var e store.Erasure
		json.NewDecoder(rec.Body).Decode(&e)
		return rec.Code, e
	}
	if code, _ := erase("Bearer wrong-key-wrong-key"); code != 401 {
		t.Fatalf("wrong key: %d, want 401", code)
	}
	code, e := erase("Bearer " + key)
	if code != 200 || e != (store.Erasure{Anonymised: 2, Deleted: 3, Favourites: 2}) {
		t.Fatalf("first erase: %d %+v", code, e)
	}
	code, e = erase("Bearer " + key)
	if code != 200 || e != (store.Erasure{}) {
		t.Fatalf("second erase is not a no-op: %d %+v", code, e)
	}

	for _, id := range []string{pub, old} {
		m, err := st.Get(ctx, id)
		if err != nil {
			t.Fatalf("published %s gone: %v", id, err)
		}
		if m.SubmittedBy != "" || m.Author != store.DeletedUser {
			t.Errorf("%s not anonymised: by %q author %q", id, m.SubmittedBy, m.Author)
		}
		if _, err := os.Stat(filepath.Join(data, id+".zip")); err != nil {
			t.Errorf("published package %s removed: %v", id, err)
		}
	}
	for _, id := range []string{pending, rejected, down} {
		if _, err := st.Get(ctx, id); err != store.ErrNotFound {
			t.Errorf("unpublished %s survived: %v", id, err)
		}
		if _, err := os.Stat(filepath.Join(data, id+".zip")); !os.IsNotExist(err) {
			t.Errorf("unpublished package %s survived: %v", id, err)
		}
	}
	if m, _ := st.Get(ctx, other); m == nil || m.Author != "Real Name" || m.SubmittedBy != "acct-stays" {
		t.Errorf("another account's mod changed: %+v", m)
	}
	if favs, _ := st.Favourites(ctx, gone); len(favs) != 0 {
		t.Errorf("favourites survived: %d", len(favs))
	}
	if favs, _ := st.Favourites(ctx, "acct-stays"); len(favs) != 2 {
		t.Errorf("another account's favourites changed: %d, want 2", len(favs))
	}
}
