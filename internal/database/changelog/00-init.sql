CREATE TABLE IF NOT EXISTS messages (
    id VARCHAR,
    guild_id VARCHAR,
    channel_id VARCHAR,
    author_id VARCHAR,
    reply_message_id VARCHAR,
    content VARCHAR,
    date TIMESTAMP,
    version INTEGER DEFAULT 1,
    PRIMARY KEY (id, version)
);

CREATE TABLE IF NOT EXISTS reactions (
    id VARCHAR,
    guild_id VARCHAR,
    channel_id VARCHAR,
    author_id VARCHAR,
    reaction VARCHAR,
    date TIMESTAMP,
    PRIMARY KEY (id, reaction, author_id)
);

CREATE INDEX IF NOT EXISTS idx_message_reactions ON reactions (id);

CREATE TABLE IF NOT EXISTS emojis (
    id VARCHAR,
    name VARCHAR,
    guild_id VARCHAR,
    image_data VARCHAR,
    PRIMARY KEY (guild_id, id)
);

CREATE TABLE IF NOT EXISTS bot_messages (
    id VARCHAR,
    guild_id VARCHAR,
    channel_id VARCHAR,
    author_id VARCHAR,
    reply_message_id VARCHAR,
    interaction_author_id VARCHAR,
    content VARCHAR,
    date TIMESTAMP,
    version INTEGER DEFAULT 1,
    PRIMARY KEY (id, version)
);