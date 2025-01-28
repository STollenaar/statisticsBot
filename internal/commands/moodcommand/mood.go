package moodcommand

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
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

func (m MoodCommand) Handler(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Calculating Mood...",
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	parsedArguments := m.ParseArguments(bot, interaction).(*CommandParsed)

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
		fmt.Printf("mood duckDB error: %e\n", err)
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &eString,
		})
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
			bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
				Content: &eString,
			})
			return
		}
		messagMap[id] = content
		messageIds = append(messageIds, id)
	}

	if len(messageIds) <= util.ConfigFile.MIN_SAMPLES+1 {
		eString := "Not enough messages to mood. Try increasing the timeframe"
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &eString,
		})
		return
	}

	// Get the Milvus vectors
	mvResult, err := database.QueryMilvus(fmt.Sprintf(milvusQuery, fmt.Sprintf(`["%s"]`, strings.Join(messageIds, `", "`))), []string{"id", "mood_embedding"})
	if err != nil {
		eString := "error happened while trying to fetch the messages"
		fmt.Printf("query milvus error: %e\n", err)
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &eString,
		})
		return
	}

	for {
		rs, err := mvResult.Next(context.TODO())
		if err != nil {
			break
		}
		for i := 0; i < rs.GetColumn("id").Len(); i++ {
			var id string
			var vector []float32

			id, _ = rs.GetColumn("id").GetAsString(i)
			v, _ := rs.GetColumn("mood_embedding").Get(i)
			vector = v.([]float32)
			messages = append(messages, MoodBody{vector, messagMap[id]})
		}
	}

	// Get and create the Mood
	mood, err := getMood(messages)
	if err != nil {
		eString := "error happened while trying to generate the mood"
		fmt.Printf("mood error: %e\n", err)
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &eString,
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("Mood of the past %s", parsedArguments.Unit),
	}

	for topic, Mood := range mood.Mood {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  topic,
			Value: Mood,
		})
	}

	bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
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

func (m MoodCommand) CreateCommandArguments() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Name:        "unit",
			Description: "How far back to get the mood of a conversation",
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

func getMood(messages []MoodBody) (MoodResponse, error) {

	requestBody, _ := json.Marshal(MoodRequest{
		MoodBodies: messages,
		Eps:           util.ConfigFile.EPS,
		MinSamples:    util.ConfigFile.MIN_SAMPLES,
		TopN:          util.ConfigFile.TOP_N,
	})

	// os.WriteFile("req.json", requestBody, 0644)

	resp, err := http.Post(fmt.Sprintf("http://%s/mood", util.ConfigFile.SENTENCE_TRANSFORMERS), "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return MoodResponse{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	result := MoodResponse{
		Mood: map[string]string{},
	}
	json.Unmarshal(body, &result)
	return result, nil
}
