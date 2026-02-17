CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE embeddings (
    content_hash BYTEA  NOT NULL REFERENCES chunks(content_hash),
    model        TEXT   NOT NULL,
    embedding    vector(3072) NOT NULL,
    PRIMARY KEY (content_hash, model)
);
