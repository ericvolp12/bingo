-- Entries
CREATE TABLE IF NOT EXISTS entries (
    did TEXT NOT NULL,
    handle TEXT NOT NULL,
    is_valid BOOLEAN DEFAULT FALSE NOT NULL,
    last_checked_time TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ,
    PRIMARY KEY (did)
);
CREATE INDEX IF NOT EXISTS entries_handle ON entries (handle);
