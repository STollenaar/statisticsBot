# bot.py
import os
import database
import discord

from discord.ext import commands
from datetime import datetime
from collections import defaultdict
from collections import Counter

client = discord.client
client = commands.Bot(command_prefix='~', case_insensitive=True)


async def index_channels(guildID, text_channels, before=None):  # indexing the channels
    for channel in text_channels:
        if not await database.has_channel_saved(channel.id):
            await database.add_channel(guildID, channel.id)

        # getting the last message datetime
        lastMessage = await database.get_last_message_date_by_channel(channel.id)
        dt = datetime.strptime(
            lastMessage, '%Y-%m-%d %H:%M:%S') if lastMessage is not None else None

        messages = await channel.history(limit=None, before=before, after=dt).flatten()
        for message in messages:
            await database.add_message(message)


def max_by_weigh(sequence):
    if not sequence:
        raise ValueError('empty sequence')

    maximum = sequence[0]

    for item in sequence:
        # Compare elements by their weight stored
        # in their second element.
        if item[1] > maximum[1]:
            maximum = item

    return maximum


@client.command(aliases=["last"])
async def lastMessage(ctx, channel=None):
    author = ctx.message.mentions[0].id if len(
        ctx.message.mentions) != 0 else None

    if author is None:
        await ctx.send("Error using this command, you didn't specify who")
        return

    rows = ""
    # getting the count through the database
    async with ctx.channel.typing():
        channel_id = next(
            (c for c in ctx.guild.text_channels if channel is not None and c.name == channel), None)
        rows = await database.last_message_of_user(ctx.guild.id, author, channel_id)

    await ctx.send("{}, {} last sent something on {} ".format(ctx.author.mention, ctx.guild.get_member(author).mention, rows))


@client.command(aliases=["max"])
async def maxWord(ctx, word=None, channel=None):
    author = ctx.message.mentions[0].id if len(
        ctx.message.mentions) != 0 else None

    rows = []
    # getting the count through the database
    async with ctx.channel.typing():
        channel_id = next(
            (c for c in ctx.guild.text_channels if channel is not None and c.name == channel), None)

        channel_id = next(
            (c for c in ctx.guild.text_channels if word is not None and c.name == word), None)

        # getting the count through the database
        rows = (await database.max_word(
            ctx.guild.id, channel_id, author, word))[0]

    try:
        await ctx.send("{}: The word(s) \"{}\" has been the most used by {} and is used {} times".format(ctx.author.mention, rows[2], client.get_user(rows[1]).mention, rows[0]))
    except:
        await ctx.send("{}: The word(s) \"{}\" has been the most used by {} and is used {} times".format(ctx.author.mention, rows[2], "Deleted User", rows[0]))


@client.command(aliases=["count"])
async def countWord(ctx, word, channel=None):

    rows = 0
    author = ctx.message.mentions[0].id if len(
        ctx.message.mentions) != 0 else ctx.author.id
    # getting the count through the database
    async with ctx.channel.typing():
        channel_id = next(
            (c for c in ctx.guild.text_channels if channel is not None and c.name == channel), None)

        # getting the count through the database
        rows = await database.count_word(
            ctx.guild.id, author, word, channel_id)

    if author != ctx.author.id:
        await ctx.send('{}: {} has used the word(s) \"{}\" {} times'.format(ctx.author.mention, ctx.message.mentions[0].mention, word, int(rows)))
    else:
        await ctx.send('{} you have used the word(s) \"{}\" {} times'.format(ctx.author.mention, word, int(rows)))


@client.event
async def on_message(message):
    await client.process_commands(message)
    await index_channels(message.guild.id, message.guild.text_channels)


@client.event
async def on_ready():
    print(f'{client.user} has connected to Discord!')
    # adding the guilds (servers) that the bot is in if it isn't already in the db.
    for guild in client.guilds:
        if not await database.is_in_guild(guild.id):
            await database.add_guild(guild.id)

        await index_channels(guild.id, guild.text_channels)

    print('done start up indexing')

client.run(os.environ['DISCORD_TOKEN'])
