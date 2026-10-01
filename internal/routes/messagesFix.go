package routes

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/jobs"
	"github.com/stollenaar/statisticsbot/internal/util"
)

type deleteBadEntriesResponse struct {
	Updates     map[string]int    `json:"updates"`
	BadMessages []discord.Message `json:"badMessages"`
}

type MessageBody struct {
	Embedding     []float32
	MoodEmbedding []float32
	Message       string
	GuildID       string
	ChannelID     string
	AuthorID      string
}

func addFixMessages(mux *http.ServeMux) {
	mux.HandleFunc("DELETE /fixMessages", deleteBadMessages)
	mux.HandleFunc("PUT /fixMessages", startMissingMessages)
	mux.HandleFunc("GET /fixMessages", getMissingMessages)
}

func deleteBadMessages(w http.ResponseWriter, r *http.Request) {
	query := `
	SELECT id AS message_id,
	channel_id,
	guild_id,
	content,
	date
	FROM messages 
	WHERE date IS NULL OR content = '' or guild_id = '';
	`

	updateDate := `
	UPDATE messages
	SET date = ?
	WHERE id = ?;
	`

	updateGuild := `
	UPDATE messages
	SET guild_id = ?
	WHERE id = ?;
	`

	deleteMessage := `
	DELETE FROM messages
	WHERE id = ?;
	`
	rs, err := database.QueryDuckDB(query, []interface{}{})

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	defer rs.Close()

	tx, err := database.StartTX()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	response := deleteBadEntriesResponse{
		Updates: make(map[string]int),
	}

	cachedGuilds := make(map[string]string)

	for rs.Next() {
		var channel_id, message_id, guild_id string
		var content, date any

		err = rs.Scan(&message_id, &channel_id, &guild_id, &content, &date)
		if err != nil {
			break
		}

		if date == nil {
			snflk, err := util.SnowflakeToTimestamp(message_id)
			if err != nil {
				slog.Error("messagesFix error", slog.Any("err", err))
				continue
			}
			_, err = tx.Exec(updateDate, snflk, message_id)
			response.Updates["date"] = response.Updates["date"] + 1
			if err != nil {
				slog.Error("messagesFix error", slog.Any("err", err))
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				tx.Rollback()
				return
			}
		}
		if guild_id == "" {
			var guild string
			var ok bool
			if guild, ok = cachedGuilds[channel_id]; !ok {
				channel, _ := client.Caches.Channel(snowflake.MustParse(channel_id))

				guild = channel.GuildID().String()
				cachedGuilds[channel_id] = guild
			}

			_, err = tx.Exec(updateGuild, guild, message_id)
			response.Updates["guild"] = response.Updates["guild"] + 1
			if err != nil {
				slog.Error("messagesFix error", slog.Any("err", err))
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				tx.Rollback()
				return
			}
		}
		if content == "" {
			message, _ := client.Caches.Message(snowflake.MustParse(channel_id), snowflake.MustParse(message_id))

			if message.Flags != discord.MessageFlagLoading &&
				message.Type != discord.MessageTypeUserJoin &&
				message.Type != discord.MessageTypeChannelPinnedMessage &&
				message.Type != discord.MessageTypeGuildBoost &&
				message.Type != discord.MessageTypeGuildBoostTier1 &&
				message.Type != discord.MessageTypeGuildBoostTier2 &&
				message.Type != discord.MessageTypeGuildBoostTier3 &&
				message.Thread == nil &&
				message.Poll == nil &&
				message.StickerItems == nil ||
				message.Author.Bot {
				_, err := tx.Exec(deleteMessage, message_id)
				response.Updates["deleted"] = response.Updates["deleted"] + 1
				if err != nil {
					slog.Error("messagesFix error", slog.Any("err", err))
				}
				continue
			}
			if message.Type == discord.MessageTypeDefault && message.ReferencedMessage == nil && message.MessageReference != nil {
				_, err := tx.Exec(deleteMessage, message_id)
				response.Updates["deleted"] = response.Updates["deleted"] + 1
				if err != nil {
					slog.Error("messagesFix error", slog.Any("err", err))
				}
				continue
			}
			if len(message.Embeds) > 0 && message.Embeds[0].Type == "poll_result" {
				_, err := tx.Exec(deleteMessage, message_id)
				response.Updates["deleted"] = response.Updates["deleted"] + 1
				if err != nil {
					slog.Error("messagesFix error", slog.Any("err", err))
				}
				continue
			}
			if len(message.Attachments) > 0 {
				_, err := tx.Exec(deleteMessage, message_id)
				response.Updates["deleted"] = response.Updates["deleted"] + 1
				if err != nil {
					slog.Error("messagesFix error", slog.Any("err", err))
				}
				continue
			}
			response.BadMessages = append(response.BadMessages, message)
			if util.ConfigFile.DEBUG {
				discordLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guild_id, channel_id, message.ID)
				slog.Debug("Discord link to the message", slog.String("link", discordLink))
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "message": response})
	} else {
		writeJSON(w, http.StatusOK, map[string]deleteBadEntriesResponse{"message": response})
	}
}

// startMissingMessages kicks off a scan of every message-bearing channel's full
// history, storing anything the database does not already have, and returns
// without waiting for it. The scan walks the REST API channel by channel and
// takes far longer than any sane HTTP timeout, so progress is read back from
// GET /fixMessages.
func startMissingMessages(w http.ResponseWriter, r *http.Request) {
	if !jobs.StartMessageSync(client) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":  "a message sync is already running",
			"status": jobs.MessageSync(),
		})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"message": "message sync started, poll GET /fixMessages for progress",
		"status":  jobs.MessageSync(),
	})
}

// getMissingMessages reports the progress of the current or last sync.
func getMissingMessages(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, jobs.MessageSync())
}
