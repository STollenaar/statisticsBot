package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// CountCommand counts the amount of occurences of a certain word
func CountCommand(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Loading Data...",
		},
	})

	parsedArguments := parseArguments(bot, interaction)
	if parsedArguments.UserTarget == nil {
		parsedArguments.UserTarget = interaction.Member.User
	}
	amount := FindSpecificWordOccurences(parsedArguments)

	var response string
	if parsedArguments.UserTarget != nil && parsedArguments.UserTarget.ID != interaction.Member.User.ID {
		response = fmt.Sprintf("%s has used the word \"%s\" %d time(s) \n", parsedArguments.UserTarget.Mention(), parsedArguments.Word, amount)
	} else {
		response = fmt.Sprintf("You have used the word \"%s\" %d time(s) \n", parsedArguments.Word, amount)
	}
	bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Content: &response,
	})
}

// FindSpecificWordOccurences finding the occurences of a word in the database
func FindSpecificWordOccurences(args *CommandParsed) int {

	filter, params := getFilter(args)

	messages, err := CountFilterOccurences(filter, args.Word, params)

	if err != nil {
		fmt.Println(err)
		return 0
	}
	if len(messages) == 0 {
		return 0
	}
	return messages[0].Word.Count
}
