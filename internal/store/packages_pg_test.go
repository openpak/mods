package store

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("MODS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MODS_TEST_DATABASE_URL not set; Postgres tests skipped")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS favourites, mod_packages, mods, schema_migrations`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func newMod() *Mod {
	return &Mod{ID: NewID(), Slug: "s", TitleID: "0100000000010000", Name: "n", Version: "1", Author: "a",
		Licence: "MIT", SourceURL: "https://example.org", Layout: "romfs", SHA256: "00", Size: 3, SubmittedBy: "acct"}
}

func TestPackageRoundTripAndRestart(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	m := newMod()
	zip := bytes.Repeat([]byte{0, 1, 2, 255}, 1<<16)
	if err := New(pool).InsertWithPackage(ctx, m, zip); err != nil {
		t.Fatal(err)
	}
	// A fresh store (and a second migrate) over the same database sees it.
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	got, err := New(pool).Package(ctx, m.ID)
	if err != nil || !bytes.Equal(got, zip) {
		t.Fatalf("after restart: %v len %d", err, len(got))
	}
	if _, err := New(pool).Package(ctx, NewID()); err != ErrNotFound {
		t.Fatalf("missing package: %v", err)
	}
}

func TestImportPackages(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	st := New(pool)
	m := newMod()
	if err := st.Insert(ctx, m); err != nil { // row without package, as before the move
		t.Fatal(err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, m.ID+".zip"), []byte("legacy zip"), 0o600)
	os.WriteFile(filepath.Join(dir, NewID()+".zip"), []byte("orphan"), 0o600)
	os.WriteFile(filepath.Join(dir, m.ID+".zip.tmp"), []byte("partial"), 0o600)
	imp, skip, err := st.ImportPackages(ctx, dir)
	if err != nil || imp != 1 || skip != 2 {
		t.Fatalf("first import: %d %d %v", imp, skip, err)
	}
	if imp, _, err = st.ImportPackages(ctx, dir); err != nil || imp != 0 {
		t.Fatalf("second import must add nothing: %d %v", imp, err)
	}
	if b, err := st.Package(ctx, m.ID); err != nil || string(b) != "legacy zip" {
		t.Fatalf("imported: %v %q", err, b)
	}
	if _, err := os.Stat(filepath.Join(dir, m.ID+".zip")); err != nil {
		t.Fatal("import deleted a file")
	}
}
