CREATE TABLE IF NOT EXISTS mods (
    id            UUID PRIMARY KEY,
    slug          TEXT NOT NULL,            -- mod.json "id": stable across versions
    title_id      TEXT NOT NULL,            -- 16 hex chars, upper case
    name          TEXT NOT NULL,
    version       TEXT NOT NULL,
    author        TEXT NOT NULL,
    licence       TEXT NOT NULL,
    source_url    TEXT NOT NULL,
    layout        TEXT NOT NULL CHECK (layout IN ('romfs','exefs')),
    summary       TEXT NOT NULL DEFAULT '',
    sha256        TEXT NOT NULL,
    size          BIGINT NOT NULL,
    status        TEXT NOT NULL CHECK (status IN ('pending','published','replaced','rejected','taken_down')),
    reason        TEXT NOT NULL DEFAULT '',
    submitted_by  TEXT NOT NULL,            -- core account id
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ,
    replaced_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS mods_title_status ON mods (title_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS mods_one_published ON mods (title_id, slug) WHERE status = 'published';

CREATE TABLE IF NOT EXISTS favourites (
    account_id TEXT NOT NULL,
    mod_id     UUID NOT NULL REFERENCES mods(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, mod_id)
);
