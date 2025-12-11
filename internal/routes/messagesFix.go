package routes

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/gin-gonic/gin"
	"github.com/stollenaar/statisticsbot/internal/database"
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

func addFixMessages(r *gin.Engine) {
	r.DELETE("/fixMessages", deleteBadMessages)
	r.PUT("/fixMessages", addMissingMessages)
}

func deleteBadMessages(c *gin.Context) {
	query := `
	SELECT id AS message_id,
	channel_id,
	guild_id,
	content,
	date
	FROM messages 
	WHERE date IS NULL OR content == '' or guild_id == '';
	`

	updateDate := `
	UPDATE messages
	SET date = ?
	WHERE id == ?;
	`

	updateGuild := `
	UPDATE messages
	SET guild_id = ?
	WHERE id == ?;
	`

	deleteMessage := `
	DELETE FROM messages
	WHERE id == ?;
	`
	rs, err := database.QueryDuckDB(query, []interface{}{})
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	tx, err := database.StartTX()
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
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
				fmt.Println(err)
				continue
			}
			_, err = tx.Exec(updateDate, snflk, message_id)
			response.Updates["date"] = response.Updates["date"] + 1
			if err != nil {
				fmt.Println(err)
				c.JSON(500, gin.H{
					"error": err.Error(),
				})
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
				fmt.Println(err)
				c.JSON(500, gin.H{
					"error": err.Error(),
				})
				tx.Rollback()
				return
			}
		}
		if content == "" {
			message, _ := client.Caches.Message(snowflake.MustParse(channel_id), snowflake.MustParse(message_id))

			if err != nil {
				var apiErr *discordgo.RESTError
				if errors.As(err, &apiErr) && apiErr.Message.Code != discordgo.ErrCodeUnknownMessage {
					fmt.Println(err)
					c.JSON(500, gin.H{
						"error": err.Error(),
					})
					tx.Rollback()
					return
				} else {
					_, err := tx.Exec(deleteMessage, message_id)
					response.Updates["deleted"] = response.Updates["deleted"] + 1

					if err != nil {
						fmt.Println(err)
					}
					continue
				}
			}
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
					fmt.Println(err)
				}
				continue
			}
			if message.Type == discord.MessageTypeDefault && message.ReferencedMessage == nil && message.MessageReference != nil {
				_, err := tx.Exec(deleteMessage, message_id)
				response.Updates["deleted"] = response.Updates["deleted"] + 1
				if err != nil {
					fmt.Println(err)
				}
				continue
			}
			if len(message.Embeds) > 0 && message.Embeds[0].Type == "poll_result" {
				_, err := tx.Exec(deleteMessage, message_id)
				response.Updates["deleted"] = response.Updates["deleted"] + 1
				if err != nil {
					fmt.Println(err)
				}
				continue
			}
			if len(message.Attachments) > 0 {
				_, err := tx.Exec(deleteMessage, message_id)
				response.Updates["deleted"] = response.Updates["deleted"] + 1
				if err != nil {
					fmt.Println(err)
				}
				continue
			}
			response.BadMessages = append(response.BadMessages, message)
			if util.ConfigFile.DEBUG {
				discordLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guild_id, channel_id, message.ID)
				fmt.Println("Discord link to the message:", discordLink)
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		c.JSON(500, gin.H{
			"error":   err.Error(),
			"message": response,
		})
	} else {
		c.JSON(200, gin.H{
			"message": response,
		})
	}
}

func addMissingMessages(c *gin.Context) {
	query := `
		SELECT id FROM messages
		UNION ALL
		SELECT id FROM bot_messages;
	`

	reactions := `SELECT id, author_id, reaction FROM reactions`

	reactionTable := make(map[string]bool)

	rs, err := database.QueryDuckDB(query, nil)
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	var ids []string
	for rs.Next() {
		var id string
		err = rs.Scan(&id)
		if err != nil {
			break
		}
		ids = append(ids, id)
	}

	rs, err = database.QueryDuckDB(reactions, nil)
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	for rs.Next() {
		var id, author_id, reaction string
		err = rs.Scan(&id, &author_id, &reaction)
		if err != nil {
			break
		}
		reactionTable[fmt.Sprintf("%s_%s_%s", id, author_id, reaction)] = true
	}

	guilds := slices.Collect(client.Caches.Guilds())

	var waitGroup sync.WaitGroup
	var mu sync.Mutex
	var missed int

	for _, guild := range guilds {
		channels := slices.Collect(client.Caches.ChannelsForGuild(guild.ID))

		// Async checking the channels of guild for new messages
		waitGroup.Add(1)
		go func(client *bot.Client, channels []discord.GuildChannel, waitGroup *sync.WaitGroup) {
			defer waitGroup.Done()
			miss := doChannels(client, channels, ids, reactionTable)
			mu.Lock()
			missed += miss
			mu.Unlock()
		}(client, channels, &waitGroup)
	}
	// Waiting for all async calls to complete
	waitGroup.Wait()

	c.JSON(200, gin.H{
		"message": fmt.Sprintf("done, added %d messages", missed),
	})
}
func doChannels(client *bot.Client, channels []discord.GuildChannel, IDs []string, reactionTable map[string]bool) (missed int) {
	var waitGroup sync.WaitGroup
	var mu sync.Mutex
	for _, channel := range channels {
		// Check if channel is a guild text channel and not a voice or DM channel
		if channel.Type() != discord.ChannelTypeGuildText {
			continue
		}

		// Async loading of the messages in that channnel
		waitGroup.Add(1)
		go func(client *bot.Client, channel discord.GuildChannel, IDs []string, waitGroup *sync.WaitGroup) {
			defer waitGroup.Done()
			miss := loadMessages(client, channel, IDs, reactionTable)
			mu.Lock()
			missed += miss
			mu.Unlock()
		}(client, channel, IDs, &waitGroup)
	}
	waitGroup.Wait()
	return
}

// loadMessages loading messages from the channel
func loadMessages(client *bot.Client, channel discord.GuildChannel, IDs []string, reactionTable map[string]bool) (missed int) {
	fmt.Printf("DatabaseFix: loading %s", channel.Name)

	var result []discord.Message
	// Getting last message and first 100
	messages := slices.Collect(client.Caches.Messages(channel.ID()))

	// Constructing operations for first 100
	result = append(result, messages...)
	// Loading more messages if got 100 message the first time
	// if len(messages) == 100 {
	// 	lastMessageCollected := messages[len(messages)-1]
	// 	// Loading more messages, 100 at a time
	// 	for lastMessageCollected != nil {
	// 		moreMes, _ := Bot.ChannelMessages(channel.ID, int(100), lastMessageCollected.ID, "", "")

	// 		result = append(result, moreMes...)

	// 		if len(moreMes) != 0 {
	// 			lastMessageCollected = moreMes[len(moreMes)-1]
	// 		} else {
	// 			break
	// 		}
	// 	}
	// }
	fmt.Printf("DatabaseFix: done collecting messages for %s, found: %d messages\n", channel.Name, len(result))
	filtered := filterSlice(result, IDs)

	for _, message := range filtered {
		for _, reaction := range message.Reactions {
			// users, _ := client.MessageReactions(message.ChannelID.String(), message.ID.String(), reaction.Emoji.Name, 100, "", "")
			// for _, user := range users {
				if _, ok := reactionTable[fmt.Sprintf("%s_%s_%s", message.ID, reaction.Emoji.Creator.ID.String(), reaction.Emoji.Name)]; !ok {
					database.ConstructMessageReactObject(database.MessageReact{
						ID:        message.ID.String(),
						GuildID:   message.GuildID.String(),
						ChannelID: message.ChannelID.String(),
						Author:    reaction.Emoji.Creator.ID.String(),
						Reaction:  reaction.Emoji.Name,
					}, false)
				// }
			}
		}
		if message.Flags != discord.MessageFlagLoading &&
			message.Type != discord.MessageTypeUserJoin &&
			message.Type != discord.MessageTypeChannelPinnedMessage &&
			message.Type != discord.MessageTypeGuildBoost &&
			message.Type != discord.MessageTypeGuildBoostTier1 &&
			message.Type != discord.MessageTypeGuildBoostTier2 &&
			message.Type != discord.MessageTypeGuildBoostTier3 &&
			message.Thread == nil &&
			message.Poll == nil &&
			message.StickerItems == nil {
			if message.Type == discord.MessageTypeDefault && message.ReferencedMessage == nil && message.MessageReference != nil {
				continue
			}
			if len(message.Embeds) > 0 && message.Embeds[0].Type == "poll_result" {
				continue
			}
			if len(message.Attachments) > 0 {
				continue
			}

			database.ConstructCreateMessageObject(message, channel.GuildID().String(), message.Author.Bot)
			missed++
		}
	}
	return
}

// Remove items from A if their ID exists in B
func filterSlice(A []discord.Message, B []string) []discord.Message {
	idMap := make(map[string]struct{}, len(B))
	for _, id := range B {
		idMap[id] = struct{}{}
	}

	var filtered []discord.Message
	for _, item := range A {
		if _, exists := idMap[item.ID.String()]; !exists {
			filtered = append(filtered, item)
		}
	}

	return filtered
}
