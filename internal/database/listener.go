package database

import (
	"github.com/bwmarrin/discordgo"
)

// MessageListener registers a simpler handler on a discordgo session to automatically parse incoming messages for you.
func MessageListener(session *discordgo.Session, message *discordgo.MessageCreate) {
	constructMessageObject(message.Message, message.GuildID)
}

