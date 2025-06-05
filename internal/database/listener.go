package database

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/stollenaar/statisticsbot/internal/util"
)

// MessageCreateListener registers a simpler handler on a discordgo session to automatically parse incoming messages for you.
func MessageCreateListener(session *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Flags != discordgo.MessageFlagsLoading &&
		message.Type != discordgo.MessageTypeGuildMemberJoin &&
		message.Type != discordgo.MessageTypeChannelPinnedMessage &&
		message.Type != discordgo.MessageTypeUserPremiumGuildSubscription &&
		message.Type != discordgo.MessageTypeUserPremiumGuildSubscriptionTierOne &&
		message.Type != discordgo.MessageTypeUserPremiumGuildSubscriptionTierTwo &&
		message.Type != discordgo.MessageTypeUserPremiumGuildSubscriptionTierThree &&
		message.Thread == nil &&
		message.Poll == nil &&
		message.StickerItems == nil {
		if message.Type == discordgo.MessageTypeDefault && message.ReferencedMessage == nil && message.MessageReference != nil {
			return
		}
		if len(message.Embeds) > 0 && message.Embeds[0].Type == "poll_result" {
			return
		}
		if len(message.Attachments) > 0 {
			return
		}
		guildID := message.GuildID
		if guildID == "" {
			channel, _ := session.Channel(message.ChannelID)
			guildID = channel.GuildID
		}
		ConstructCreateMessageObject(message.Message, guildID, !message.Author.Bot)
	}
}

// MessageUpdateListener registers a simpler handler on a discordgo session to automatically parse incoming messages for you.
func MessageUpdateListener(session *discordgo.Session, message *discordgo.MessageUpdate) {
	if message.Flags != discordgo.MessageFlagsLoading &&
		message.Type != discordgo.MessageTypeGuildMemberJoin &&
		message.Type != discordgo.MessageTypeChannelPinnedMessage &&
		message.Type != discordgo.MessageTypeUserPremiumGuildSubscription &&
		message.Type != discordgo.MessageTypeUserPremiumGuildSubscriptionTierOne &&
		message.Type != discordgo.MessageTypeUserPremiumGuildSubscriptionTierTwo &&
		message.Type != discordgo.MessageTypeUserPremiumGuildSubscriptionTierThree &&
		message.Thread == nil &&
		message.Poll == nil &&
		message.StickerItems == nil {
		if message.Type == discordgo.MessageTypeDefault && message.ReferencedMessage == nil && message.MessageReference != nil {
			return
		}
		if len(message.Embeds) > 0 && message.Embeds[0].Type == "poll_result" {
			return
		}
		if len(message.Attachments) > 0 {
			return
		}
		guildID := message.GuildID
		if guildID == "" {
			channel, _ := session.Channel(message.ChannelID)
			guildID = channel.GuildID
		}

		constructUpdateMessageObject(message.Message, guildID, !message.Author.Bot)
	}
}

func MessageReactAddListener(session *discordgo.Session, message *discordgo.MessageReactionAdd) {
	guildID := message.GuildID
	if guildID == "" {
		channel, _ := session.Channel(message.ChannelID)
		guildID = channel.GuildID
	}

	ConstructMessageReactObject(MessageReact{
		ID:        message.MessageID,
		GuildID:   guildID,
		ChannelID: message.ChannelID,
		Author:    message.UserID,
		Reaction:  message.Emoji.Name,
	}, false)

	if message.Emoji.ID != "" && CustomEmojiCache[message.Emoji.Name] == "" {
		emoji, err := util.FetchDiscordEmojiImage(message.Emoji.ID, message.Emoji.Animated)

		// emoji, err := session.GuildEmoji(guildID, message.Emoji.ID)
		if err != nil {
			fmt.Printf("error fetching emoji: %s\n", err)
			return
		}
		ConstructEmojiObject(EmojiData{
			ID:        message.Emoji.ID,
			GuildID:   guildID,
			Name:      message.Emoji.Name,
			ImageData: emoji,
		})
	}
}

func MessageReactRemoveListener(session *discordgo.Session, message *discordgo.MessageReactionRemove) {
	guildID := message.GuildID
	if guildID == "" {
		channel, _ := session.Channel(message.ChannelID)
		guildID = channel.GuildID
	}

	ConstructMessageReactObject(MessageReact{
		ID:        message.MessageID,
		GuildID:   guildID,
		ChannelID: message.ChannelID,
		Author:    message.UserID,
		Reaction:  message.Emoji.ID,
	}, true)
}
