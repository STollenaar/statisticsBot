# bot.py
import os
import database
import discord

from dotenv import load_dotenv
from discord.ext import commands
from datetime import datetime

load_dotenv()
token = os.getenv('DISCORD_TOKEN')

client = discord.client
client = commands.Bot(command_prefix='~', case_insensitive=True)


async def index_channels(guildID, text_channels, before=None):  # indexing the channels

    for channel in text_channels:
        if not await database.has_channel_saved(channel.id):
            await database.add_channel(guildID, channel.id)

        # getting the last message datetime
        lastMessage = await database.get_last_message_date_by_channel(channel.id)
        dt = datetime.strptime(
            lastMessage, '%Y-%m-%d %H:%M:%S.%f') if lastMessage is not None else None

        messages = await channel.history(limit=None, before=before, after=dt).flatten()
        for message in messages:
            await database.add_message(message)


@client.command(aliases=["count"])
async def countWord(ctx, word, channel=None):

    rows = 0
    author = ctx.message.mentions[0].id if len(
        ctx.message.mentions) != 0 else ctx.author.id
    # getting the count through the database
    async with ctx.channel.typing():
        if channel is not None:
            channel_id = next(
                (c for c in ctx.guild.text_channels if c.name == channel), None)

            if channel_id is not None:
                # getting the count through the database
                rows = await database.count_word_in_channel(
                    channel_id.id, author, word)

            else:
                # getting the count through the database
                rows = await database.count_word_in_guild(
                    ctx.guild.id, author, word)
        else:
            # getting the count through the database
            rows = await database.count_word_in_guild(
                ctx.guild.id, author, word)

    if rows is None:
        rows = 0
        
    if author != ctx.author.id:
        await ctx.send('{}: {} has used the word {} {} times'.format(ctx.author.mention, ctx.message.mentions[0].mention, word, int(rows)))
    else:
        await ctx.send('{} you have used the word {} {} times'.format(ctx.author.mention, word, int(rows)))


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

client.run(token)
