# bot.py
import os

import discord
from dotenv import load_dotenv
from discord.ext import commands

load_dotenv()
token = os.getenv('DISCORD_TOKEN')

client = discord.client
client = commands.Bot(command_prefix='~')

@client.command()
async def fuckTilde(ctx):
    await ctx.send('fuck your tilde')

@client.command()
async def countWord(ctx, arg):

    #can take a long time with getting the messages
    async with ctx.channel.typing():
        messages = await ctx.message.channel.history(limit=None, before=ctx.message.created_at).flatten()
        messages = [message for message in messages if ctx.author == message.author and arg in message.content]
        await ctx.send('you have used the word {} {} times'.format(arg, len(messages)))

@client.event
async def on_ready():
        print(f'{client.user} has connected to Discord!')



client.run(token)