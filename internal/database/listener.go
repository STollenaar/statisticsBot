package database

import (
	"github.com/bwmarrin/discordgo"
)

// MessageListener registers a simpler handler on a discordgo session to automatically parse incoming messages for you.
func MessageListener(session *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Flags != discordgo.MessageFlagsLoading &&
		message.Type != discordgo.MessageTypeGuildMemberJoin &&
		message.Type != discordgo.MessageTypeChannelPinnedMessage &&
		message.Type != discordgo.MessageTypeUserPremiumGuildSubscription &&
		message.Type != discordgo.MessageTypeUserPremiumGuildSubscriptionTierOne &&
		message.Type != discordgo.MessageTypeUserPremiumGuildSubscriptionTierTwo &&
		message.Type != discordgo.MessageTypeUserPremiumGuildSubscriptionTierThree &&
		message.Thread == nil &&
		message.Poll == nil &&
		message.StickerItems == nil &&
		!message.Author.Bot {
		if message.Type == discordgo.MessageTypeDefault && message.ReferencedMessage == nil && message.MessageReference != nil {
			return
		}
		if message.Embeds != nil && len(message.Embeds) > 0 && message.Embeds[0].Type == "poll_result" {
			return
		}
		if len(message.Attachments) > 0 {
			return
		}
		ConstructMessageObject(message.Message, message.GuildID)
	}
}
