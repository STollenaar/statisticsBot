-- message_embeddings stores only the vector for a message. The id is a logical
-- foreign key to messages.id; all other fields (content, guild, channel, author,
-- date) are read back by joining to the latest version of that message.
-- A real FK constraint is not declared because messages' primary key is
-- composite (id, version), so id alone is not a unique key.
CREATE TABLE IF NOT EXISTS message_embeddings (
    id VARCHAR PRIMARY KEY,
    model VARCHAR,
    embedding FLOAT[] NOT NULL
);
