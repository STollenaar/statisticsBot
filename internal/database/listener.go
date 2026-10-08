package database

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// MessageCreateListener registers a simpler handler on a discordgo session to automatically parse incoming messages for you.
func MessageCreateListener(event *events.GuildMessageCreate) {
	message := event.Message
	if message.Flags != discord.MessageFlagLoading &&
		message.Type != discord.MessageTypeUserJoin &&
		message.Type != discord.MessageTypeChannelPinnedMessage &&
		message.Type != discord.MessageTypeGuildBoost &&
		message.Type != discord.MessageTypeGuildBoostTier1 &&
		message.Type != discord.MessageTypeGuildBoostTier2 &&
		message.Type != discord.MessageTypeGuildBoostTier3 &&
		message.Thread == nil &&
		message.Poll == nil &&
		message.StickerItems == nil {
		if message.Type == discord.MessageTypeDefault && message.ReferencedMessage == nil && message.MessageReference != nil {
			return
		}
		if len(message.Embeds) > 0 && message.Embeds[0].Type == "poll_result" {
			return
		}
		if len(message.Attachments) > 0 {
			return
		}
		ConstructCreateMessageObject(message, message.GuildID.String(), message.Author.Bot)

		// Embed and classify newly ingested messages, so they become searchable
		// and carry a mood. Both are best-effort and off the hot path; history is
		// backfilled via the /fixEmbeddings and /fixMoods routes.
		go EmbedMessage(message.ID.String(), message.Content)
		go MoodMessage(message.ID.String(), message.Content)
	}
}

// MessageUpdateListener registers a simpler handler on a discordgo session to automatically parse incoming messages for you.
func MessageUpdateListener(event *events.GuildMessageUpdate) {
	message := event.Message

	if message.Flags != discord.MessageFlagLoading &&
		message.Type != discord.MessageTypeUserJoin &&
		message.Type != discord.MessageTypeChannelPinnedMessage &&
		message.Type != discord.MessageTypeGuildBoost &&
		message.Type != discord.MessageTypeGuildBoostTier1 &&
		message.Type != discord.MessageTypeGuildBoostTier2 &&
		message.Type != discord.MessageTypeGuildBoostTier3 &&
		message.Thread == nil &&
		message.Poll == nil &&
		message.StickerItems == nil {
		if message.Type == discord.MessageTypeDefault && message.ReferencedMessage == nil && message.MessageReference != nil {
			return
		}
		if len(message.Embeds) > 0 && message.Embeds[0].Type == "poll_result" {
			return
		}
		if len(message.Attachments) > 0 {
			return
		}
		// guildID := message.GuildID
		// if guildID == "" {
		// 	channel, _ := session.Channel(message.ChannelID)
		// 	guildID = channel.GuildID
		// }

		constructUpdateMessageObject(message, message.GuildID.String(), message.Author.Bot)

		// Re-embed and re-score the edited message so search and mood reflect the
		// new content. Both saves upsert, overwriting the old values.
		go EmbedMessage(message.ID.String(), message.Content)
		go MoodMessage(message.ID.String(), message.Content)
	}
}

func MessageReactAddListener(event *events.GuildMessageReactionAdd) {

	// guildID := message.GuildID
	// if guildID == "" {
	// 	channel, _ := session.Channel(message.ChannelID)
	// 	guildID = channel.GuildID
	// }

	ConstructMessageReactObject(MessageReact{
		ID:        event.MessageID.String(),
		GuildID:   event.GuildID.String(),
		ChannelID: event.ChannelID.String(),
		Author:    event.Member.User.ID.String(),
		Reaction:  *event.Emoji.Name,
	}, false)

	// if event.Emoji.ID != "" && CustomEmojiCache[*event.Emoji.Name] == "" {
	// 	emoji, err := util.FetchDiscordEmojiImage(message.Emoji.ID, message.Emoji.Animated)

	// 	// emoji, err := session.GuildEmoji(guildID, message.Emoji.ID)
	// 	if err != nil {
	// 		fmt.Printf("error fetching emoji: %s\n", err)
	// 		return
	// 	}
	// 	ConstructEmojiObject(EmojiData{
	// 		ID:        message.Emoji.ID,
	// 		GuildID:   guildID,
	// 		Name:      message.Emoji.Name,
	// 		ImageData: emoji,
	// 	})
	// }
}

func MessageReactRemoveListener(event *events.GuildMessageReactionRemove) {
	// guildID := message.GuildID
	// if guildID == "" {
	// 	channel, _ := session.Channel(message.ChannelID)
	// 	guildID = channel.GuildID
	// }

	ConstructMessageReactObject(MessageReact{
		ID:        event.MessageID.String(),
		GuildID:   event.GuildID.String(),
		ChannelID: event.ChannelID.String(),
		Author:    event.UserID.String(),
		Reaction:  *event.Emoji.Name,
	}, true)
}
