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
        cur.execute("INSERT INTO Messages (id, channel_id, author, content, date) VALUES (%s,%s,%s,%s,%s)", (message.id,
                                                                                                             message.channel.id, message.author.id, message.content, message.created_at,))

        for member in message.mentions:
            cur.execute(
                "INSERT INTO Mentions (message_id, member_id) VALUES (%s,%s)", (message.id, member.id,))
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
    cur.execute("SELECT SUM((LENGTH(m.content) - LENGTH(REPLACE(m.content, %s, ''))) / LENGTH(%s)) FROM Channels AS c INNER JOIN Messages AS m ON c.id=m.channel_id WHERE c.guild_id=%s AND m.author=%s AND m.content LIKE %s LIMIT 1",
                (word, word, guild_id, author_id, '%'+word+'%', ))
    row = cur.fetchone()
    cur.close()
    conn.close()
    return row[0] if row is not None else None


async def count_word_in_channel(channel_id, author_id, word):
    conn = await manage_connections()
    cur = conn.cursor()
    cur.execute("SELECT SUM((LENGTH(m.content) - LENGTH(REPLACE(m.content, %s, ''))) / LENGTH(%s)) FROM Channels AS c INNER JOIN Messages AS m ON c.id=m.channel_id WHERE c.id=%s AND m.author=%s AND m.content LIKE %s LIMIT 1",
                (word, word, channel_id, author_id, '%'+word+'%',))
    row = cur.fetchone()
    cur.close()
    conn.close()
    return row[0] if row is not None else None
