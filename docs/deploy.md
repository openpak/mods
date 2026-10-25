# Deploy: mods

## Run

```sh
cp example.env .env; set -a; . ./.env; set +a
go run ./cmd/mods
```

Postgres only; packages live in `MODS_DATA`. Image `ghcr.io/openpak/mods` on tag. AGPL-3.0-only.

## Ports and environment

Listens on `:20030` (`MODS_HTTP_ADDR`). Variables in `example.env`:

| Variable | Example value |
| --- | --- |
| `MODS_HTTP_ADDR` | `:20030` |
| `MODS_DATABASE_URL` | `postgres://openpak:secret@localhost:23000/mods?sslmode=disable` |
| `MODS_INTERNAL_KEY` | `change-me-at-least-sixteen-bytes` |
| `MODS_DATA` | `./data` |
| `WEBSITE_URL` | `https://openpak.org` |

## Image

The Dockerfile builds `/app/app` from `./cmd/mods` and runs it as uid 65532. `/app/data` holds
the packages: mount a volume there. Port 20030 needs no capability to bind.

## Release

Pushing a tag `v*.*.*` (or a manual dispatch) publishes `ghcr.io/openpak/mods` with the version
tags and `:latest`, for linux/amd64 and linux/arm64. Deployed hosts run podman-auto-update
against `:latest`, so pushing a tag is the deploy.

Pull requests run `gofmt`, `go vet` and `go test` (`.github/workflows/pr.yml`).
