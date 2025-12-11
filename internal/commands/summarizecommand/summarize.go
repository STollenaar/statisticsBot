package summarizecommand

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

var (
	SummarizeCmd = SummarizeCommand{
		Name:        "summarize",
		Description: "summarize past messages from a period of time",
	}
	pastMessages = `
	SELECT m.author_id, m.content
	FROM messages m
	JOIN (
		SELECT id, MAX(version) AS latest_version
		FROM messages
		WHERE guild_id = ?
		AND channel_id = ?
		AND date BETWEEN ? AND ?
		GROUP BY id
	) sub ON m.id = sub.id AND m.version = sub.latest_version;
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

func (s SummarizeCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	err := event.DeferCreateMessage(util.ConfigFile.SetEphemeral() == discord.MessageFlagEphemeral)
	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}

	sub := event.SlashCommandInteractionData()

	unit, err := parseTimeArg(sub.Options["unit"].String())
	if err != nil {
		eString := err.Error()
		_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Content: &eString,
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err))
		}
		return
	}

	now := time.Now()

	// Get all messages in the time frame
	rs, err := database.QueryDuckDB(pastMessages, []interface{}{event.GuildID().String(), event.Channel().ID().String(), now.Add(-unit), now})
	if err != nil {
		eString := "error happened while trying to fetch the messages"
		fmt.Printf("mood duckDB error: %e\n", err)
		_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Content: &eString,
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err))
		}
		return
	}

	var messages []SummaryBody

	for rs.Next() {
		var author_id, content string
		err := rs.Scan(&author_id, &content)
		if err != nil {
			eString := "error happened while trying to build summary body"
			fmt.Printf("summarize duckDB error: %e\n", err)
			_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
				Content: &eString,
			})
			if err != nil {
				slog.Error("Error editing the response:", slog.Any("err", err))
			}
			return
		}
		var nickname string

		member, _ := event.Client().Caches.Member(*event.GuildID(), snowflake.MustParse(author_id))
		if member.Nick == nil {
			nickname = author_id
		} else {
			nickname = *member.Nick
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
		_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Content: &eString,
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err))
		}
		return
	}

	embed := discord.Embed{
		Title: fmt.Sprintf("Summary of the past %s", sub.Options["unit"].String()),
	}

	for _, summary := range summaries.Summaries {
		embed.Fields = append(embed.Fields, discord.EmbedField{
			Name:  summary.Topic,
			Value: summary.Summary,
		})
	}
	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Embeds: &[]discord.Embed{embed},
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
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

func (s SummarizeCommand) CreateCommandArguments() []discord.ApplicationCommandOption {
	return []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "unit",
			Description: "How far back to summarize the messages",
			Required:    true,
		},
	}
}

func parseTimeArg(timeUnit string) (time.Duration, error) {
	// Regular expression to match a number followed by a unit
	re := regexp.MustCompile(`^(\d+)([smhd])$`)
	matches := re.FindStringSubmatch(timeUnit)
	if matches == nil {
		return 0, fmt.Errorf("invalid time format: %s", timeUnit)
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
	if util.ConfigFile.DEBUG {
		d, _ := json.MarshalIndent(messages, "", "    ")
		os.WriteFile("summary.json", d, 0644)
	}
	resp, err := util.CreateOllamaGeneration(util.OllamaGenerateRequest{
		Model: "mistral:7b",
		Prompt: fmt.Sprintf(
			`You are an AI that outputs valid JSON only. 
			Do not anonymize or replace author IDs.
				Use the exact author ID provided in the input.
			Summarize and group the following messages by topic. 

			Return a JSON array of objects in this exact format:
			{
			"messages": [
				{
				"topic": "string",
				"summary": "string"
				}
			]
			}

			Here is the input: 
			%s`,
			string(data)),
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
	fmt.Printf("Raw response for summarize: %s\n", resp.Response)
	return
}
