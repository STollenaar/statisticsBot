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
client = commands.Bot(command_prefix='~')


async def index_channels(guildID, text_channels, before=None):  # indexing the channels

    for channel in text_channels:
        if not database.has_channel_saved(channel.id):
            database.add_channel(guildID, channel.id)

        # getting the last message datetime
        lastMessage = database.get_last_message_date_by_channel(channel.id)
        dt = datetime.strptime(
            lastMessage, '%Y-%m-%d %H:%M:%S.%f') if lastMessage is not None else None

        messages = await channel.history(limit=None, before=before, after=dt).flatten()
        for message in messages:
            database.add_message(message)


@client.command()
async def countWord(ctx, word, channel=None):

    rows = 0
    # getting the count through the database with the last indexes added
    async with ctx.channel.typing():
        await index_channels(ctx.guild.id, ctx.guild.text_channels, ctx.message.created_at)
        if channel is not None:
            channel_id = next(
                (c for c in ctx.guild.text_channels if c.name == channel), None).id

            if channel_id is not None:
                # getting the count through the database
                rows = database.count_word_in_channel(
                    channel_id, ctx.author.id, word)

            else:
                # getting the count through the database
                rows = database.count_word_in_guild(
                    ctx.guild.id, ctx.author.id, word)
        else:
            # getting the count through the database
            rows = database.count_word_in_guild(
                ctx.guild.id, ctx.author.id, word)

        await ctx.send('{} you have used the word {} {} times'.format(ctx.author.mention, word, rows))


@client.event
async def on_ready():
    print(f'{client.user} has connected to Discord!')
    # adding the guilds (servers) that the bot is in if it isn't already in the db.
    for guild in client.guilds:
        if not database.is_in_guild(guild.id):
            database.add_guild(guild.id)

        await index_channels(guild.id, guild.text_channels)

    print('done start up indexing')

client.run(token)
