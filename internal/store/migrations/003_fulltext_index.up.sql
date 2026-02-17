CREATE INDEX idx_chunks_content_fts ON chunks
    USING GIN (to_tsvector('english', content));
