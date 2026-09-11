// mods: the OpenPak mod catalogue.
//
//	MODS_HTTP_ADDR      listener, default :20030
//	MODS_DATABASE_URL   Postgres; required
//	MODS_INTERNAL_KEY   shared secret for /internal/* (curation); required
//	MODS_DATA           package directory, default /app/data (mount a volume)
//	WEBSITE_URL         where /api/v1/me lives, default https://openpak.org
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openpak/mods/internal/api"
	"github.com/openpak/mods/internal/store"
)

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	dbURL, key := os.Getenv("MODS_DATABASE_URL"), os.Getenv("MODS_INTERNAL_KEY")
	if dbURL == "" || len(key) < 16 {
		log.Fatal("MODS_DATABASE_URL and MODS_INTERNAL_KEY (>=16 bytes) must be set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := store.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	srv := api.New(store.New(pool), envOr("MODS_DATA", "/app/data"), envOr("WEBSITE_URL", "https://openpak.org"), key)
	addr := envOr("MODS_HTTP_ADDR", ":20030")
	log.Printf("mods listening %s", addr)
	log.Fatal(http.ListenAndServe(addr, srv))
}
