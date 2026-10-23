# mods

> **Current Switch client (2026-10-13): Harbor**, at
> `~/REPOS/Openpak/emulators/harbor`. Harbor is an independent
> OpenPak project based on Eden. Citron, Ryujinx (desktop and Android), and
> the former OpenPak Eden checkouts have been removed. References to those
> forks below record historical evidence only: do not run their commands,
> reinstall them, or select them for current work. Use Harbor's README and
> current handoff for builds and testing; old results do not establish Harbor parity.

The OpenPak mod catalogue: cosmetic and local mods per title, installed by an emulator with one
tap, with provenance and licence recorded for every package and no game assets ever hosted.
PRD: `prds/mods-prd.md`. Status: **M1 built** (catalogue, package store, provenance gate,
manifest API) plus favourites and takedown; curation is an internal API until the website grows
its pages (M2); the Citron client (M3) is not started.

## API (emulator and app; bearer = a website `/api/v1` token)

| Route | What |
| --- | --- |
| `GET /api/v1/titles/{title_id}/mods` | manifest of published mods: id, version, sha256, size, licence, summary, package_url |
| `GET /api/v1/mods/{id}` | one entry |
| `GET /api/v1/mods/{id}/package` | the zip; `ETag` is the sha256, honours `If-None-Match`; a replaced version stays for 30 days |
| `POST /api/v1/submissions` (bearer) | multipart field `package`: the zip. Refused (422, with the reason) unless it passes the gate |
| `GET /api/v1/me/submissions` (bearer) | your submissions and their status |
| `GET /api/v1/me/favourites`, `PUT`/`DELETE /api/v1/me/favourites/{id}` (bearer) | favourites |

## Package format (MOD-01)

A zip with `mod.json` at the root and every file under `romfs/` or `exefs/`:

```json
{"id":"blue-hat","name":"Blue Hat","version":"1.0","title_id":"0100000000010000",
 "author":"someone","licence":"CC-BY-4.0","source_url":"https://example.org/blue-hat",
 "layout":"romfs","summary":"A blue hat."}
```

The gate (MOD-02) refuses packages without a licence or source, with files outside the layout,
or containing game data, executables or keys (`.nca .nsp .xci .nso .npdm .kip .tik .cert`,
`*.keys`, `exefs/main`). Manual review always follows.

## Curation (`Authorization: Bearer $MODS_INTERNAL_KEY`)

```sh
curl -H "Authorization: Bearer $KEY" http://127.0.0.1:20030/internal/queue
curl -X POST -H "Authorization: Bearer $KEY" http://127.0.0.1:20030/internal/mods/$ID/publish
curl -X POST -H "Authorization: Bearer $KEY" -d '{"reason":"..."}' http://127.0.0.1:20030/internal/mods/$ID/reject
curl -X POST -H "Authorization: Bearer $KEY" -d '{"reason":"..."}' http://127.0.0.1:20030/internal/mods/$ID/takedown
```

Publishing retires the previously published version of the same mod (30-day grace for
downloads); takedown removes every version at once.

## Run

```sh
cp example.env .env; set -a; . ./.env; set +a
go run ./cmd/mods
```

Postgres only; packages live in `MODS_DATA`. Image `ghcr.io/openpak/mods` on tag. AGPL-3.0-only.
