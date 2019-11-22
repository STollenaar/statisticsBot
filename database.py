# database.py
import mysql.connector
import asyncio
from mysql.connector import pooling

dbconfig = {"host": "databases", "user": "stats",
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
    cur.execute("INSERT INTO Guilds VALUES (%s);", (guild_id,))
    conn.commit()
    cur.close()
    conn.close()


async def is_in_guild(guild_id):
    conn = await manage_connections()
    cur = conn.cursor()
    cur.execute("SELECT * FROM Guilds WHERE id=%s;", (guild_id,))
    row = cur.fetchone()
    cur.close()
    conn.close()
    return row is not None


async def add_channel(guild_id, channel_id):

    conn = await manage_connections()
    cur = conn.cursor()

    try:
        cur.execute("INSERT INTO Channels VALUES (%s,%s);",
                    (channel_id, guild_id,))
        conn.commit()
        cur.close()
        conn.close()
    except:
        return


async def has_channel_saved(channel_id):
    conn = await manage_connections()
    cur = conn.cursor()
    cur.execute("SELECT * FROM Channels WHERE id=%s;", (channel_id,))
    row = cur.fetchone()
    cur.close()
    conn.close()
    return row is not None


async def add_message(message):

    conn = await manage_connections()
    cur = conn.cursor()

    try:
        if message.content:
            cur.execute("INSERT INTO Messages (id, guild_id, channel_id, author, content, date) VALUES (%s, %s,%s,%s,%s,%s);", (message.id, message.guild.id,
                                                                                                                                message.channel.id, message.author.id, message.content, message.created_at.strftime('%Y-%m-%d %H:%M:%S'),))

            for member in message.mentions:
                cur.execute(
                    "INSERT INTO Mentions (message_id, member_id) VALUES (%s,%s);", (message.id, member.id,))

            for word in message.content.split(' '):
                if word:
                    cur.execute("INSERT INTO Words (message_id, member_id, word) VALUES (%s, %s, %s);",
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
        "SELECT date FROM Messages WHERE channel_id=%s ORDER BY date DESC LIMIT 1;", (channel_id,))
    row = cur.fetchone()
    cur.close()
    conn.close()
    return row[0] if row is not None else None


async def count_word_in_guild(guild_id, author_id, word):
    conn = await manage_connections()
    cur = conn.cursor()
    if len(word.split(' ')) == 1:
        cur.execute("SELECT COUNT(*) FROM Messages as m INNER JOIN Words AS w ON m.id=w.message_id WHERE m.guild_id=%s AND m.author=%s AND w.word=%s LIMIT 1;",
                    (guild_id, author_id, word,))
    else:
        cur.execute("SELECT COUNT(*) FROM Messages as m WHERE m.guild_id=%s AND m.author=%s AND m.content LIKE %s LIMIT 1",
                    (guild_id, author_id, '%'+word+'%'))

    row = cur.fetchone()
    cur.close()
    conn.close()
    return row[0] if row is not None else -1


async def count_word_in_channel(channel_id, author_id, word):
    conn = await manage_connections()
    cur = conn.cursor()
    if len(word.split(' ')) == 1:
        cur.execute("SELECT COUNT(*) FROM Messages as m INNER JOIN Words AS w ON m.id=w.message_id WHERE m.channel_id=%s AND m.author=%s AND w.word=%s LIMIT 1;",
                    (channel_id, author_id, word,))
    else:
        cur.execute("SELECT COUNT(*) FROM Messages as m WHERE m.channel_id=%s AND m.author=%s AND m.content LIKE %s LIMIT 1",
                    (channel_id, author_id, '%'+word+'%'))

    row = cur.fetchone()
    cur.close()
    conn.close()
    return row[0] if row is not None else -1


async def max_word(guild_id, channel_id=None, author_id=None, word=None):
    conn = await manage_connections()
    cur = conn.cursor()
    query = "SELECT COUNT(w.word) AS amount, m.author, word FROM Messages as m INNER JOIN Words AS w ON m.id=w.message_id WHERE m.guild_id=%s"
    pars = (guild_id,)

    if author_id is not None:
        query += " AND m.author = %s"
        pars += (author_id,)
    if channel_id is not None:
        query += " AND m.id=%s"
        pars += (channel_id,)
    if word is not None:
        if len(word.split(' ')) == 1:
            query += " AND w.word=%s"
        else:
            query = query.replace('SELECT COUNT(w.word) AS amount, m.author, word FROM Messages as m INNER JOIN Words AS w ON m.id=w.message_id', 'SELECT COUNT(*) AS amount, m.author FROM Messages AS m')
            query += " AND m.content LIKE %s"
        pars += ('% '+word+' %',)

    if word is not None and len(word.split(' ')) != 1:
        query += " GROUP BY m.author ORDER BY amount DESC;"
    else:
        query += " GROUP BY w.word, m.author ORDER BY amount DESC;"

    cur.execute(query, pars)
    row = cur.fetchall()
    cur.close()
    conn.close()

    if len(row[0]) < 3:
        row[0]+= (word,)

    return row


async def last_message_of_user(guild_id, author_id, channel_id=None):
    conn = await manage_connections()
    cur = conn.cursor()
    if channel_id is not None:
        cur.execute("SELECT date FROM Messages WHERE guild_id=%s AND channel_id=%s AND author=%s ORDER BY id DESC LIMIT 1;",
                    (guild_id, channel_id.id, author_id,))
    else:
        cur.execute(
            "SELECT date FROM Messages WHERE guild_id=%s AND author=%s ORDER BY id DESC LIMIT 1;", (guild_id, author_id,))
    row = cur.fetchone()
    cur.close()
    conn.close()
    return row[0] + "UTC." if row is not None else "no message found."
