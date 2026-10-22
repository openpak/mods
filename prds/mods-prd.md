# OpenPak mods PRD

> **Current Switch client (2026-10-13): Harbor**, at
> `~/REPOS/Openpak/emulators/harbor`. Harbor is an independent
> OpenPak project based on Eden. Citron, Ryujinx (desktop and Android), and
> the former OpenPak Eden checkouts have been removed. References to those
> forks below record historical evidence only: do not run their commands,
> reinstall them, or select them for current work. Use Harbor's README and
> current handoff for builds and testing; old results do not establish Harbor parity.

Status: **M1 built and deployed 2026-09-11** (repo `mods`, proxied under openpak.org/api/v1); curation is an internal API until M2; M3 (Citron client) not started. Original brief (2026-09-07):

## 1. Product objective

A curated catalogue of cosmetic and local mods per title that an emulator installs with one
tap, with provenance and licence recorded for every package, and no copyrighted game assets
ever hosted.

## 2. Relationship to existing plans

Independent of the Switch adapter and the core, except for the account id behind
favourites and for the website hosting the browsing and curation pages. Console delivery
(LayeredFS on the SD card) belongs to the setup homebrew later, not to this PRD.

## 3. Evidence and current baseline

Earlier research notes (evidence only): per-title manifest listing mods, per-mod zip streamed to
the client, bearer-gated, client extracts into the game's mod folder; admin-curated,
cosmetic and local only; GameBanana favourites. Citron calls `/api/mods/` and a favourites
route.

## 4. Users and journeys

1. Player: mods tab for a title in the emulator; catalogue; install; mod active on next
   launch; uninstall.
2. Curator: submission with source URL, licence, author, checksum; review; publish or reject
   with reason.
3. Author: submit via the website; see status.
4. Operator: takedown; clients stop serving it; installed copies untouched.

## 5. Scope and non-goals

In: catalogue service, package store, curation workflow, favourites, emulator API, website
pages, takedown. Out: hosting game assets or patches derived from game data, mod-building
tools, ratings and comments, console delivery.

## 6. Independent implementation rules (apply verbatim)

- OpenPak servers are AGPL-3.0-only and OpenPak-authored.
- Go 1.26, module `github.com/openpak/<repo>`, AGPL-3.0-only, `LICENSE` verbatim from
  gnu.org. Server-rendered templates and minimal browser JavaScript for web surfaces.
  Native mobile code is the recorded non-Go exception.
- Talk to the account core **only** through its versioned contracts (gRPC: Identity, Links,
  Social, Sessions, Invitations, Events; HTTP `/v1/*`). Never read its database. If a
  contract is missing, write the proposal in this PRD's "Contract requests" section and
  raise it as an `account` issue; do not fork the core.
- Do not touch `Openpak/nx-baas` (the Switch adapter, owned by the T4/T5 agent) or the
  console-facing hostnames it serves. Console-side needs go into "Contract requests".
- No secrets, tokens, saves, or copyrighted game material in any repository. `example.env` only.
- Definition of done for every milestone: `go vet ./...` and `go test -race ./...` green in
  CI, a runbook in `README.md`, and the acceptance journey below reproduced by the owner.

## 7. Functional requirements

- MOD-01 Package format: zip with `mod.json` (id, name, version, title id, author, licence as
  SPDX id or custom text, source URL, install layout `romfs/` or `exefs/`); server computes
  SHA-256 and size.
- MOD-02 Provenance gate: no publish without licence and source; a denylist of file patterns
  that indicate game assets; manual review always.
- MOD-03 Catalogue API: per-title manifest (id, version, hash, size, licence, summary),
  package download with `ETag` and `If-None-Match`, favourites per account.
- MOD-04 Versioning: a new version replaces the manifest entry; old versions downloadable
  for 30 days; takedown removes all versions immediately.
- MOD-05 Curation pages in the website (queue, `mod.json` diff, publish or reject, audit).
- MOD-06 Client contract for emulators: manifest polling, install layout, uninstall.

## 8. Architecture and ownership

Repo `mods`: `cmd/mods`, Postgres for catalogue and workflow, object store for packages,
HTTP API for emulators (website token model), gRPC for the website's curation pages.

## 9. Contract requests

- MR-1 Favourites need only the account id from the token; no core change.
- MR-2 Emulator token: the website's `/api/v1` bearer tokens (companion apps PRD).

## 10. Milestones and acceptance

- M1 Catalogue, package store, provenance gate, manifest API. Gate: a sample mod with a valid
  licence publishes; one with a game-asset pattern is rejected.
- M2 Curation pages and takedown. Gate: takedown leaves the manifest within a minute.
- M3 Emulator install and uninstall through Citron. Gate: a cosmetic mod visibly active in
  Outbound or Stardew on Citron.

## 11. Risks

- Legal exposure from uploads: the provenance gate and manual review are mandatory.
- Emulator client work lives in a GPL fork; coordinate with the maintainer of the OpenPak
  Citron build.

## 12. Evidence references

Earlier research notes (behaviour only): manifest plus zip streaming, bearer-gated,
cosmetic and local; Citron client paths.
