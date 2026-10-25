# Mods Server

OpenPak's Mods Server.

| | |
| --- | --- |
| Role | Mod catalogue: cosmetic and local mods per title, installed by an emulator with one tap; provenance and licence recorded for every package; no game assets hosted |
| Depends on | PostgreSQL; the OpenPak website issues the `/api/v1` bearer tokens |
| Image | `ghcr.io/openpak/mods` |
| Licence | AGPL-3.0-only |

## What it serves

| Feature | What |
| --- | --- |
| Mod manifest | Published mods per title: id, version, sha256, size, licence, summary, package URL |
| Package download | The mod zip; `ETag` is the sha256 |
| Submissions | Upload a package (bearer); refused with the reason unless it passes the gate |
| Favourites | Per-user favourites (bearer) |
| Curation | Internal queue, publish, reject and takedown (internal key) |

## Run

```sh
cp example.env .env; set -a; . ./.env; set +a
go run ./cmd/mods
```

Ports, environment and image are in [Deploy](docs/deploy.md).

## Docs

- [Protocol](docs/protocol.md)
- [Evidence](docs/evidence.md)
- [Deploy](docs/deploy.md)
- [Notes](docs/notes.md)
- [Docs index](docs/README.md)

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE).
