# database.py
import mysql.connector
import asyncio
from mysql.connector import pooling

dbconfig = {"host": "localhost", "user": "stats",
            "password": "stats", "database": "STATS_DB", "charset": "utf8mb4"}

connpool = mysql.connector.pooling.MySQLConnectionPool(pool_name="statspool",
                                                       pool_size=8,
                                                       **dbconfig)


# small way around the pool exhaustion
async def manage_connections():
    try:
        return connpool.get_connection()
    except:
        await asyncio.sleep(1)
        return await manage_connections()


async def add_guild(guild_id):
    conn = await manage_connections()
    cur = conn.cursor()
    cur.execute("INSERT INTO Guilds VALUES (%s)", (guild_id,))
    conn.commit()
    cur.close()
    conn.close()


async def is_in_guild(guild_id):
    conn = await manage_connections()
    cur = conn.cursor()
    cur.execute("SELECT * FROM Guilds WHERE id=%s", (guild_id,))
    row = cur.fetchone()
    cur.close()
    conn.close()
    return row is not None


async def add_channel(guild_id, channel_id):

    conn = await manage_connections()
    cur = conn.cursor()

    try:
        cur.execute("INSERT INTO Channels VALUES (%s,%s)",
                    (channel_id, guild_id,))
        conn.commit()
        cur.close()
        conn.close()
    except:
        return


async def has_channel_saved(channel_id):
    conn = await manage_connections()
    cur = conn.cursor()
    cur.execute("SELECT * FROM Channels WHERE id=%s", (channel_id,))
    row = cur.fetchone()
    cur.close()
    conn.close()
    return row is not None


async def add_message(message):

    conn = await manage_connections()
    cur = conn.cursor()

    try:
        if message.content:
            cur.execute("INSERT INTO Messages (id, guild_id, channel_id, author, content, date) VALUES (%s, %s,%s,%s,%s,%s)", (message.id, message.guild.id,
                                                                                                                               message.channel.id, message.author.id, message.content, message.created_at.strftime('%Y-%m-%d %H:%M:%S'),))

            for member in message.mentions:
                cur.execute(
                    "INSERT INTO Mentions (message_id, member_id) VALUES (%s,%s)", (message.id, member.id,))

            for word in message.content.split(' '):
                if word:
                    cur.execute("INSERT INTO Words (message_id, member_id, word) VALUES (%s, %s, %s)",
                                (message.id, message.author.id, word,))

        conn.commit()
        cur.close()
        conn.close()
    except: 
        cur.close()
        conn.close()
        return


async def get_last_message_date_by_channel(channel_id):
    conn = await manage_connections()
    cur = conn.cursor()
    cur.execute(
        "SELECT date FROM Messages WHERE channel_id=%s ORDER BY date DESC LIMIT 1", (channel_id,))
    row = cur.fetchone()
    cur.close()
    conn.close()
    return row[0] if row is not None else None


async def count_word_in_guild(guild_id, author_id, word):
    conn = await manage_connections()
    cur = conn.cursor()
    cur.execute("SELECT COUNT(*) FROM Messages as m INNER JOIN Words AS w ON m.id=w.message_id WHERE m.guild_id=%s AND m.author=%s AND w.word=%s LIMIT 1",
                (guild_id, author_id, word,))
    row = cur.fetchone()
    cur.close()
    conn.close()
    return row[0] if row is not None else None


async def count_word_in_channel(channel_id, author_id, word):
    conn = await manage_connections()
    cur = conn.cursor()
    cur.execute("SELECT COUNT(*) FROM Messages as m INNER JOIN Words AS w ON m.id=w.message_id WHERE m.channel_id=%s AND m.author=%s AND w.word=%s LIMIT 1",
                (channel_id, author_id, word,))
    row = cur.fetchone()
    cur.close()
    conn.close()
    return row[0] if row is not None else None


async def max_word_in_guild(guild_id, author_id=None):
    conn = await manage_connections()
    cur = conn.cursor()
    if author_id is not None:
        cur.execute("SELECT COUNT(w.word) AS amount, w.member_id, w.word FROM Messages as m INNER JOIN Words AS w ON m.id=w.message_id WHERE m.guild_id=%s AND w.member_id=%s GROUP BY w.word ORDER BY amount DESC", (guild_id, author_id,))
    else:
        cur.execute(
            "SELECT COUNT(w.word) AS amount, w.member_id, w.word FROM Messages as m INNER JOIN Words AS w ON m.id=w.message_id WHERE m.guild_id=%s GROUP BY w.word ORDER BY amount DESC", (guild_id,))
    row = cur.fetchall()
    cur.close()
    conn.close()
    return row


async def max_word_in_channel(channel_id, author_id=None):
    conn = await manage_connections()
    cur = conn.cursor()
    if author_id is not None:
        cur.execute("SELECT COUNT(w.word) AS w.amount, w.member_id, word FROM Messages as m INNER JOIN Words AS w ON m.id=w.message_id WHERE w.channel_id=%s AND w.word=%s GROUP BY w.member_id ORDER BY amount DESC", (channel_id, author_id,))
    else:
        cur.execute(
            "SELECT COUNT(w.word) AS w.amount, w.member_id, word FROM Messages as m INNER JOIN Words AS w ON m.id=w.message_id WHERE w.channel_id=%s GROUP BY w.word ORDER BY amount DESC", (channel_id,))
    row = cur.fetchall()
    cur.close()
    conn.close()
    return row
