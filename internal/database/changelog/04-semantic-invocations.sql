-- semantic_invocations records each /semantic search so an admin can review
-- what has been searched for. Only the query and its outcome are kept; the
-- matched messages are not copied here because they are already in messages and
-- the ranking can be reproduced by re-running the search.
CREATE TABLE IF NOT EXISTS semantic_invocations (
    id VARCHAR PRIMARY KEY,
    guild_id VARCHAR NOT NULL,
    channel_id VARCHAR NOT NULL,
    author_id VARCHAR NOT NULL,
    query VARCHAR NOT NULL,
    model VARCHAR NOT NULL,
    pool_size INTEGER NOT NULL,
    result_count INTEGER NOT NULL,
    requested_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR DEFAULT 'success',
    error VARCHAR
);
