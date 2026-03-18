CREATE TABLE IF NOT EXISTS summary_invocations (
    id VARCHAR PRIMARY KEY,
    guild_id VARCHAR NOT NULL,
    channel_id VARCHAR NOT NULL,
    unit VARCHAR NOT NULL,
    requested_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    messages_json VARCHAR NOT NULL,
    raw_response VARCHAR,
    status VARCHAR DEFAULT 'pending'
);
