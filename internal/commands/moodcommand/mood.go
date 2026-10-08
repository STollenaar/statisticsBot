package moodcommand

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

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
		SELECT content
		FROM latest_messages
		WHERE guild_id = ?
		AND channel_id = ?
		AND date BETWEEN ? AND ?
		AND content <> '';
	`
)

// Discord's embed limits; exceeding any of them is a rejected request.
const (
	maxEmbedFields = 25
	maxFieldName   = 256
	maxFieldValue  = 1024
)

type MoodCommand struct {
	Name        string
	Description string
}

type CommandParsed struct {
	Unit string
}

// MoodResponse mirrors the json getMood asks the model for, and the
// response_format schema it declares: one entry per detected topic.
type MoodResponse struct {
	Messages []MoodTopic `json:"messages"`
}

type MoodTopic struct {
	Topic string `json:"topic"`
	Mood  string `json:"mood"`
}

func (m MoodCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	err := event.DeferCreateMessage(util.ConfigFile.SetEphemeral() == discord.MessageFlagEphemeral)

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}
	sub := event.SlashCommandInteractionData()

	unit, err := util.ParseTimeArg(sub.Options["unit"].String())
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
	// Channel().String() renders the channel as display text, not its id, so
	// this used to filter on a value no row could ever hold.
	rs, err := database.QueryDuckDB(pastMessages, []interface{}{event.GuildID().String(), event.Channel().ID().String(), now.Add(-unit), now})

	if err != nil {
		eString := "error happened while trying to fetch the messages"
		slog.Error("mood duckDB error", slog.Any("err", err))
		_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Content: &eString,
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err))
		}
		return
	}
	defer rs.Close()

	var messages []string

	for rs.Next() {
		var content string
		if err := rs.Scan(&content); err != nil {
			eString := "error happened while trying to build Mood body"
			slog.Error("mood duckDB error", slog.Any("err", err))
			_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
				Content: &eString,
			})
			if err != nil {
				slog.Error("Error editing the response:", slog.Any("err", err))
			}
			return
		}
		messages = append(messages, content)
	}
	if err := rs.Err(); err != nil {
		eString := "error happened while trying to fetch the messages"
		slog.Error("mood duckDB error", slog.Any("err", err))
		_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Content: &eString,
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err))
		}
		return
	}

	// Nothing to analyse; asking the model about an empty list only wastes a
	// round trip and produces an empty embed.
	if len(messages) == 0 {
		eString := fmt.Sprintf("No messages in this channel in the past %s", sub.Options["unit"].String())
		if _, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Content: &eString,
		}); err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err))
		}
		return
	}

	// Get and create the Mood
	mood, err := getMood(messages)
	if err != nil {
		eString := "error happened while trying to generate the mood"
		slog.Error("mood error", slog.Any("err", err))
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

	for _, topic := range mood.Messages {
		// Discord rejects an embed with more than maxEmbedFields fields, and
		// truncates nothing for you.
		if len(embed.Fields) == maxEmbedFields {
			break
		}
		embed.Fields = append(embed.Fields, discord.EmbedField{
			Name:  truncate(topic.Topic, maxFieldName),
			Value: truncate(topic.Mood, maxFieldValue),
		})
	}

	if len(embed.Fields) == 0 {
		eString := "could not determine a mood for those messages"
		if _, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Content: &eString,
		}); err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err))
		}
		return
	}

	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Embeds: &[]discord.Embed{embed},
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
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

func getMood(messages []string) (out MoodResponse, err error) {
	data, err := json.Marshal(messages)
	if err != nil {
		return MoodResponse{}, err
	}

	prompt := fmt.Sprintf("group the following messages together and analyze the mood. Make sure to return both the topic of the grouped messages, and mood analysis. Return it as a json string of this format {\"messages\":[{\"topic\":\"...\",\"mood\":\"...\"}]}: %s", string(data))

	resp, err := util.CreateOllamaGeneration(util.OllamaGenerateRequest{
		Model:            util.ConfigFile.OLLAMA_MODEL,
		Temperature:      0.2,
		FrequencePenalty: 1.8,
		PresencePenalty:  1.2,
		MaxTokens:        len(data) + 1000,
		Messages:         []map[string]string{{"role": "user", "content": prompt}},
		ResponseFormat: map[string]interface{}{
			"type": "json_object",
			"properties": map[string]interface{}{
				"messages": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "json_object",
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
		// Was `return MoodResponse{}, nil`, which reported success on failure and
		// left the caller rendering an empty embed.
		return MoodResponse{}, err
	}

	// An empty choices array is a valid json response from the endpoint, so this
	// has to be checked rather than indexed.
	if len(resp.Choices) == 0 {
		return MoodResponse{}, errors.New("ollama returned no choices")
	}

	rawResponse := resp.Choices[0].Message.Content
	slog.Debug("Raw response for mood", slog.String("rawResponse", rawResponse))
	if err := json.Unmarshal([]byte(rawResponse), &out); err != nil {
		return MoodResponse{}, fmt.Errorf("unmarshalling mood response: %w", err)
	}
	return out, nil
}

// truncate shortens s to at most n runes, appending an ellipsis when cut.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
