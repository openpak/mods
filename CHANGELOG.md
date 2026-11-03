# Changelog — mods

Generated from git history on 2026-09-15. `git log` stays the source
of truth; this file is the readable summary.

## v0.2.0 — 2026-11-03
- Per-request logging: one slog line per HTTP request (method, path, status, ms) on stderr via internal/httplog. Query strings, headers and bodies are never logged; /healthz and /metrics are skipped.

## Unreleased

- `DELETE /internal/accounts/{account}/mods` for account erasure: published versions stay, credited
  to "deleted user" and tied to no account; pending, rejected and taken-down submissions (rows and
  packages) and the account's favourites are deleted. Idempotent; 200
  `{"anonymised","deleted","favourites"}`, 503 when a package could not be deleted (retry).

## v0.1.0 — 2026-09-11

- mods: catalogue, package store, provenance gate, manifest API, favourites, curation [0bebe22]

