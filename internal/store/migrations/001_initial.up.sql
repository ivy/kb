CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE chunks (
    content_hash BYTEA PRIMARY KEY,
    content      TEXT NOT NULL
);

CREATE TABLE sources (
    url             TEXT PRIMARY KEY,
    kind            TEXT NOT NULL,
    config          JSONB NOT NULL DEFAULT '{}',
    last_indexed_ref TEXT,
    last_indexed_at  TIMESTAMPTZ
);

CREATE TABLE provenance (
    content_hash BYTEA NOT NULL REFERENCES chunks(content_hash),
    source_url   TEXT  NOT NULL REFERENCES sources(url),
    file_path    TEXT  NOT NULL,
    position     INT   NOT NULL DEFAULT 0,
    metadata     JSONB NOT NULL DEFAULT '{}',
    indexed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    ref          TEXT,
    PRIMARY KEY (content_hash, source_url, file_path)
);

CREATE INDEX idx_provenance_source ON provenance(source_url);
