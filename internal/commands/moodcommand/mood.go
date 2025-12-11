package moodcommand

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

var (
	MoodCmd = MoodCommand{
		Name:        "mood",
		Description: "get the mood of messages from a period of time",
	}
	pastMessages = `
		SELECT id, content
		FROM messages
		WHERE guild_id = ? 
		AND channel_id = ?
		AND date BETWEEN ? and ?;
	`

	milvusQuery = `
		id in %s
	`
)

type MoodCommand struct {
	Name        string
	Description string
}

type CommandParsed struct {
	Unit string
}

type MoodResponse struct {
	// The key can be a string (e.g., a topic title), and the value is the Mood of that topic.
	Mood map[string]string `json:"mood"`
}

type MoodRequest struct {
	MoodBodies []MoodBody `json:"messages"`
	Eps        float32    `json:"eps"`
	MinSamples int        `json:"minSamples"`
	TopN       int        `json:"topN"`
}

type MoodBody struct {
	Vector  []float32 `json:"vector"`
	Message string    `json:"message"`
}

func (m MoodCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
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
	rs, err := database.QueryDuckDB(pastMessages, []interface{}{event.GuildID().String(), event.Channel().String(), now.Add(-unit), now})
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

	var messages []MoodBody
	messagMap := make(map[string]string)
	var messageIds []string

	for rs.Next() {
		var id, content string
		err := rs.Scan(&id, &content)
		if err != nil {
			eString := "error happened while trying to build Mood body"
			fmt.Printf("mood duckDB error: %e\n", err)
			_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
				Content: &eString,
			})
			if err != nil {
				slog.Error("Error editing the response:", slog.Any("err", err))
			}
			return
		}
		messagMap[id] = content
		messageIds = append(messageIds, id)
	}

	// Get and create the Mood
	mood, err := getMood(messages)
	if err != nil {
		eString := "error happened while trying to generate the mood"
		fmt.Printf("mood error: %e\n", err)
		_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Content: &eString,
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err))
		}
		return
	}

	embed := discord.Embed{
		Title: fmt.Sprintf("Mood of the past %s", sub.Options["unit"].String()),
	}

	for topic, Mood := range mood.Mood {
		embed.Fields = append(embed.Fields, discord.EmbedField{
			Name:  topic,
			Value: Mood,
		})
	}

	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Embeds: &[]discord.Embed{embed},
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
}

func (m MoodCommand) ParseArguments(bot *discordgo.Session, interaction *discordgo.InteractionCreate) interface{} {
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

func (m MoodCommand) CreateCommandArguments() []discord.ApplicationCommandOption {
	return []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "unit",
			Description: "How far back to get the mood of a conversation",
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

func getMood(messages []MoodBody) (out MoodResponse, err error) {
	data, err := json.Marshal(messages)
	if err != nil {
		return MoodResponse{}, err
	}
	resp, err := util.CreateOllamaGeneration(util.OllamaGenerateRequest{
		Model:  "mistral:7b",
		Prompt: fmt.Sprintf("group the following messages together and analyze the mood. Make sure to return both the topic of the grouped messages, and mood analysis. Return it as a json string of this format {\"messages\":[{\"topic\", \"mood\"}]}: %s", string(data)),
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
							"mood": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
			"required": []string{
				"messages",
				"topic",
				"mood",
			},
		},
		Stream: false,
	})
	if err != nil {
		return MoodResponse{}, nil
	}

	err = json.Unmarshal([]byte(resp.Response), &out)
	return
}
