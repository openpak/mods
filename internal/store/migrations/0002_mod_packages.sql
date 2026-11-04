-- Package bytes live in the database; deleting a mod deletes its package.
CREATE TABLE IF NOT EXISTS mod_packages (
    mod_id UUID PRIMARY KEY REFERENCES mods(id) ON DELETE CASCADE,
    data   BYTEA NOT NULL
);
