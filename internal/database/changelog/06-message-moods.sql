-- message_moods stores one emotion classification per message. The id is a
-- logical foreign key to messages.id; as with message_embeddings no real FK is
-- declared, because messages' primary key is composite (id, version) and id
-- alone is not unique.
--
-- label/score are the top-scoring emotion, denormalized so the common "what was
-- the mood of this message" query does not unpack the vector. scores is the full
-- distribution in the model's own label order, which is what makes averaging
-- over a time window meaningful. Both are only comparable across rows sharing
-- the same model, hence storing it: a different classifier changes the label set
-- and its order.
CREATE TABLE IF NOT EXISTS message_moods (
    id VARCHAR PRIMARY KEY,
    model VARCHAR NOT NULL,
    label VARCHAR NOT NULL,
    score FLOAT NOT NULL,
    scores FLOAT[] NOT NULL
);
