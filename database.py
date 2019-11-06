# database.py
import sqlite3
conn = sqlite3.connect("stats.db")


def add_guild(guild_id):
    cur = conn.cursor()
    cur.execute("INSERT INTO Guilds VALUES (?);", (guild_id,))
    conn.commit()


def is_in_guild(guild_id):
    cur = conn.cursor()
    cur.execute("SELECT * FROM Guilds WHERE id=?", (guild_id,))
    return cur.fetchone() is not None


def add_channel(guild_id, channel_id):
    cur = conn.cursor()
    cur.execute("INSERT INTO Channels VALUES (?,?);", (channel_id, guild_id,))
    conn.commit()


def has_channel_saved(channel_id):
    cur = conn.cursor()
    cur.execute("SELECT * FROM Channels WHERE id=?;", (channel_id,))
    return cur.fetchone() is not None


def add_message(message):
    cur = conn.cursor()
    cur.execute("INSERT INTO Messages VALUES (?,?,?,?,?);", (message.id,
                                                             message.channel.id, message.author.id, message.content, message.created_at,))

    for member in message.mentions:
        cur.execute(
            "INSERT INTO Mentions (message_id, member_id) VALUES (?,?);", (message.id, member.id,))
    conn.commit()


def get_last_message_date_by_channel(channel_id):
    cur = conn.cursor()
    cur.execute(
        "SELECT date FROM Messages WHERE channel_id=? ORDER BY date DESC", (channel_id,))
    row = cur.fetchone()
    return row[0] if row is not None else None


def count_word_in_guild(guild_id, author_id, word):
    cur = conn.cursor()
    cur.execute("SELECT COUNT(*) FROM Channels AS c INNER JOIN Messages AS m ON c.id=m.channel_id WHERE c.guild_id=? AND m.author=? AND m.content LIKE ?;",
                (guild_id, author_id, '%'+word+'%',))
    row = cur.fetchone()
    return row[0] if row is not None else None


def count_word_in_channel(channel_id, author_id, word):
    cur = conn.cursor()
    cur.execute("SELECT COUNT(*) FROM Channels AS c INNER JOIN Messages AS m ON c.id=m.channel_id WHERE c.id=? AND m.author=? AND m.content LIKE ?;",
                (channel_id, author_id,  '%'+word+'%',))
    row = cur.fetchone()
    return row[0] if row is not None else None
