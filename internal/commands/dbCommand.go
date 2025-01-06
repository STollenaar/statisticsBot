package commands

import (
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/stollenaar/statisticsbot/internal/database"
)

// CountCommand counts the amount of occurences of a certain word
func DBCommand(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Uploading Database...",
		},
	})

	dbFile, err := os.Open(database.GetDBPath())
	if err != nil {
		content := "Error accessing database file: " + err.Error()
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}
	defer dbFile.Close()

	file := &discordgo.File{
		Name:        "statsbot.db",
		ContentType: "application/octet-stream",
		Reader:      dbFile,
	}

	bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Files: []*discordgo.File{file},
	})
}
