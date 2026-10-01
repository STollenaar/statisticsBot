-- latest_messages exposes only the newest version of each message.
--
-- messages is append-only per edit: constructUpdateMessageObject inserts a new
-- row with version = MAX(version) + 1 rather than updating in place, so almost
-- every read wants the latest version and nothing else. That "join against
-- MAX(version) grouped by id" shape was copy-pasted into six queries across the
-- database and command packages; this view is the single definition they share,
-- and the one place to optimize if the table outgrows the current plan.
--
-- SELECT m.* keeps the view column-for-column identical to messages, version
-- included, so callers can swap `messages` for `latest_messages` without
-- touching their projections.
CREATE OR REPLACE VIEW latest_messages AS
SELECT m.*
FROM messages m
JOIN (
    SELECT id, MAX(version) AS latest_version
    FROM messages
    GROUP BY id
) l ON m.id = l.id AND m.version = l.latest_version;
