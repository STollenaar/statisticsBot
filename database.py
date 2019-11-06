# database.py
import mysql.connector

conn = mysql.connector.connect(host= 'localhost',
	user= 'stats',
	password= 'stats',
	database= 'STATS_DB',)
conn.set_charset_collation('utf8mb4')

def add_guild(guild_id):
    cur = conn.cursor()
    cur.execute("INSERT INTO Guilds VALUES (%s)", (guild_id,))
    conn.commit()


def is_in_guild(guild_id):
    cur = conn.cursor()
    cur.execute("SELECT * FROM Guilds WHERE id=%s", (guild_id,))
    return cur.fetchone() is not None


def add_channel(guild_id, channel_id):
    cur = conn.cursor()
    cur.execute("INSERT INTO Channels VALUES (%s,%s)", (channel_id, guild_id,))
    conn.commit()


def has_channel_saved(channel_id):
    cur = conn.cursor()
    cur.execute("SELECT * FROM Channels WHERE id=%s", (channel_id,))
    return cur.fetchone() is not None


def add_message(message):
    cur = conn.cursor()
    cur.execute("INSERT INTO Messages (id, channel_id, author, content, date) VALUES (%s,%s,%s,%s,%s)", (message.id,
                                                             message.channel.id, message.author.id, message.content, message.created_at,))

    for member in message.mentions:
        cur.execute(
            "INSERT INTO Mentions (message_id, member_id) VALUES (%s,%s)", (message.id, member.id,))
    conn.commit()


def get_last_message_date_by_channel(channel_id):
    cur = conn.cursor()
    cur.execute(
        "SELECT date FROM Messages WHERE channel_id=%s ORDER BY date DESC LIMIT 1", (channel_id,))
    row = cur.fetchone()
    return row[0] if row is not None else None


def count_word_in_guild(guild_id, author_id, word):
    cur = conn.cursor()
    cur.execute("SELECT COUNT(*) FROM Channels AS c INNER JOIN Messages AS m ON c.id=m.channel_id WHERE c.guild_id=%s AND m.author=%s AND m.content LIKE %s",
                (guild_id, author_id, '%'+word+'%',))
    row = cur.fetchone()
    return row[0] if row is not None else None


def count_word_in_channel(channel_id, author_id, word):
    cur = conn.cursor()
    cur.execute("SELECT COUNT(*) FROM Channels AS c INNER JOIN Messages AS m ON c.id=m.channel_id WHERE c.id=%s AND m.author=%s AND m.content LIKE %s",
                (channel_id, author_id,  '%'+word+'%',))
    row = cur.fetchone()
    return row[0] if row is not None else None
