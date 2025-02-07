package summarizecommand

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

var (
	SummarizeCmd = SummarizeCommand{
		Name:        "summarize",
		Description: "summarize past messages from a period of time",
	}
	pastMessages = `
		SELECT author_id, content
		FROM messages
		WHERE guild_id = ? 
		AND channel_id = ?
		AND date BETWEEN ? and ?;
	`
)

type SummarizeCommand struct {
	Name        string
	Description string
}

// CommandParsed parsed struct for count command
type CommandParsed struct {
	Unit string
}

type SummaryResponse struct {
	Summaries []SummaryResponseBody `json:"messages"`
}

type SummaryResponseBody struct {
	Topic   string `json:"topic"`
	Summary string `json:"summary"`
}

type SummaryRequest struct {
	SummaryBodies []SummaryBody `json:"messages"`
}

type SummaryBody struct {
	Author  string `json:"author"`
	Message string `json:"message"`
}

func (s SummarizeCommand) Handler(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Summarizing Data...",
			// Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	parsedArguments := s.ParseArguments(bot, interaction).(*CommandParsed)

	unit, err := parsedArguments.parseTimeArg()
	if err != nil {
		eString := err.Error()
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &eString,
		})
		return
	}

	now := time.Now()

	// Get all messages in the time frame
	rs, err := database.QueryDuckDB(pastMessages, []interface{}{interaction.GuildID, interaction.ChannelID, now.Add(-unit), now})
	if err != nil {
		eString := "error happened while trying to fetch the messages"
		fmt.Printf("summarize duckDB error: %e\n", err)
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &eString,
		})
		return
	}

	var messages []SummaryBody

	for rs.Next() {
		var author_id, content string
		err := rs.Scan(&author_id, &content)
		if err != nil {
			eString := "error happened while trying to build summary body"
			fmt.Printf("summarize duckDB error: %e\n", err)
			bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
				Content: &eString,
			})
			return
		}
		var nickname string
		member, err := bot.GuildMember(interaction.GuildID, author_id)
		if err != nil {
			nickname = author_id
		}else{
			nickname = member.Nick
		}
		messages = append(messages, SummaryBody{
			Author:  nickname,
			Message: content,
		})
	}

	// Get and create the summary
	summaries, err := getSummary(messages)
	if err != nil {
		eString := "error happened while trying to generate the summaries"
		fmt.Printf("summarize error: %e\n", err)
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &eString,
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("Summary of the past %s", parsedArguments.Unit),
	}

	for _, summary := range summaries.Summaries {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  summary.Topic,
			Value: summary.Summary,
		})
	}

	bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

func (s SummarizeCommand) ParseArguments(bot *discordgo.Session, interaction *discordgo.InteractionCreate) interface{} {
	parsedArguments := new(CommandParsed)

	// Access options in the order provided by the user.
	options := interaction.ApplicationCommandData().Options
	// Or convert the slice into a map
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if option, ok := optionMap["unit"]; ok {
		// Option values must be type asserted from interface{}.
		// Discordgo provides utility functions to make this simple.
		parsedArguments.Unit = option.StringValue()
	}
	return parsedArguments
}

func (s SummarizeCommand) CreateCommandArguments() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Name:        "unit",
			Description: "How far back to summarize the messages",
			Type:        discordgo.ApplicationCommandOptionString,
			Required:    true,
		},
	}
}

func (c *CommandParsed) parseTimeArg() (time.Duration, error) {
	// Regular expression to match a number followed by a unit
	re := regexp.MustCompile(`^(\d+)([smhd])$`)
	matches := re.FindStringSubmatch(c.Unit)
	if matches == nil {
		return 0, fmt.Errorf("invalid time format: %s", c.Unit)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid number: %v", err)
	}

	unit := matches[2]
	var duration time.Duration

	// Calculate duration based on the unit
	switch unit {
	case "s": // seconds
		duration = time.Duration(value) * time.Second
	case "m": // minutes
		duration = time.Duration(value) * time.Minute
	case "h": // hours
		duration = time.Duration(value) * time.Hour
	case "d": // days
		duration = time.Duration(value) * 24 * time.Hour
	default:
		return 0, fmt.Errorf("unknown time unit: %s", unit)
	}

	// Enforce maximum time limit (1 day)
	maxDuration := 24 * time.Hour
	if duration > maxDuration {
		return 0, fmt.Errorf("time cannot exceed 1 day (24h)")
	}

	return duration, nil
}

func getSummary(messages []SummaryBody) (out SummaryResponse, err error) {
	data, err := json.Marshal(messages)
	if err != nil {
		return SummaryResponse{}, err
	}
	resp, err := util.CreateOllamaGenaration(util.OllamaGenerateRequest{
		Model:  "mistral:7b",
		Prompt: fmt.Sprintf("group the following messages together and summarize. Make sure to return both the topic of the grouped messages, and summary. Return it as a json string of this format {\"messages\":[{\"topic\", \"summary\"}]}: %s", string(data)),
		Format: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"messages": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"topic": map[string]interface{}{
								"type": "string",
							},
							"summary": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
			"required": []string{
				"messages",
			},
		},
		Stream: false,
	})
	if err != nil {
		return SummaryResponse{}, nil
	}

	err = json.Unmarshal([]byte(resp.Response), &out)
	return
}
