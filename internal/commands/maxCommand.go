package commands

import (
	"fmt"

	"github.com/stollenaar/statisticsbot/util"

	"github.com/bwmarrin/discordgo"
)

// MaxCommand counts the amount of occurences of a certain word
func MaxCommand(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Loading Data...",
		},
	})

	parsedArguments := parseArguments(bot, interaction)
	if !parsedArguments.isNotEmpty() {
		parsedArguments.UserTarget = interaction.Member.User
	}

	maxWord := FindAllWordOccurences(parsedArguments)

	var response string
	if (parsedArguments.UserTarget != nil && parsedArguments.UserTarget.ID != interaction.Member.User.ID) || maxWord.Author != interaction.Member.User.ID {
		targetUser := parsedArguments.UserTarget
		if targetUser == nil {
			t, _ := bot.GuildMember(interaction.GuildID, maxWord.Author)
			if t == nil {
				targetUser = &discordgo.User{Username: "Unknown User", ID: maxWord.Author}
				// response = fmt.Sprintf("%s has used the word \"%s\" the most, and is used %d time(s) \n", maxWord.Author, maxWord.Word.Word, maxWord.Word.Count)
			} else {
				targetUser = t.User
			}
		}
		response = fmt.Sprintf("%s has used the word \"%s\" the most, and is used %d time(s) \n", targetUser.Mention(), maxWord.Word.Word, maxWord.Word.Count)
	} else {
		response = fmt.Sprintf("You have used the word \"%s\" the most, and is used %d time(s) \n", maxWord.Word.Word, maxWord.Word.Count)
	}
	fmt.Printf("Max Output: %s", response)
	bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Content: &response,
	})
}

// FindAllWordOccurences finding the occurences of a word in the database
func FindAllWordOccurences(arguments *CommandParsed) util.CountGrouped {
	filter, params := getFilter(arguments)

	messageObject, err := CountFilterOccurences(filter, arguments.Word, params)
	if err != nil {
		fmt.Println(err)
		return util.CountGrouped{}
	}

	if len(messageObject) != 0 {
		return messageObject[0]
	} else {
		return util.CountGrouped{}
	}
}