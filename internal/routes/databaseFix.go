package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

type MessageBody struct {
	Embedding     []float32
	MoodEmbedding []float32
	Message       string
	GuildID       string
	ChannelID     string
	AuthorID      string
}

func addFixDatabase(r *gin.Engine) {
	r.DELETE("/fixDatabase", deleteBadEntries)
	r.PUT("/fixDatabase", addMissingEntries)
}

func deleteBadEntries(c *gin.Context) {
	query := `
	SELECT id AS message_id,
	channel_id,
	guild_id,
	content,
	date
	FROM messages 
	WHERE date IS NULL OR content == '';
	`

	updateDate := `
	UPDATE messages
	SET date = ?
	WHERE id = ?;
	`

	deleteMessage := `
	DELETE FROM messages
	WHERE id = ?;
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

	var deletedIDs []string
	var messages []*discordgo.Message
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
			message, err := bot.ChannelMessage(channel_id, message_id)
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
					deletedIDs = append(deletedIDs, message_id)
					_, err := tx.Exec(deleteMessage, message_id)
					if err != nil {
						fmt.Println(err)
					}
					continue
				}
			}
			if message.Flags == discordgo.MessageFlagsLoading ||
				message.Type == discordgo.MessageTypeGuildMemberJoin ||
				message.Type == discordgo.MessageTypeChannelPinnedMessage ||
				message.Type == discordgo.MessageTypeUserPremiumGuildSubscription ||
				message.Type == discordgo.MessageTypeUserPremiumGuildSubscriptionTierOne ||
				message.Type == discordgo.MessageTypeUserPremiumGuildSubscriptionTierTwo ||
				message.Type == discordgo.MessageTypeUserPremiumGuildSubscriptionTierThree ||
				message.Thread != nil ||
				message.Poll != nil ||
				message.StickerItems != nil ||
				message.Author.Bot {
				deletedIDs = append(deletedIDs, message_id)
				_, err := tx.Exec(deleteMessage, message_id)
				if err != nil {
					fmt.Println(err)
				}
				continue
			}
			if message.Type == discordgo.MessageTypeDefault && message.ReferencedMessage == nil && message.MessageReference != nil {
				_, err := tx.Exec(deleteMessage, message_id)
				deletedIDs = append(deletedIDs, message_id)
				if err != nil {
					fmt.Println(err)
				}
				continue
			}
			if message.Embeds != nil && len(message.Embeds) > 0 && message.Embeds[0].Type == "poll_result" {
				_, err := tx.Exec(deleteMessage, message_id)
				deletedIDs = append(deletedIDs, message_id)
				if err != nil {
					fmt.Println(err)
				}
				continue
			}
			if len(message.Attachments) > 0 {
				_, err := tx.Exec(deleteMessage, message_id)
				deletedIDs = append(deletedIDs, message_id)
				if err != nil {
					fmt.Println(err)
				}
				continue
			}
			discordLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guild_id, channel_id, message.ID)
			messages = append(messages, message)
			fmt.Println("Discord link to the message:", discordLink)
			fmt.Println(message.Flags == discordgo.MessageFlagsIsCrossPosted)
		}
	}

	data, err := json.Marshal(messages)
	os.WriteFile("messages.json", []byte(data), 0644)
	err = tx.Commit()
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
	} else {
		c.JSON(200, gin.H{
			"message": "done",
		})
	}
}

func addMissingEntries(c *gin.Context) {
	query := `
	SELECT id FROM messages`

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
	guilds, err := bot.UserGuilds(100, "", "", false)
	if err != nil {
		fmt.Println(err)
	}

	var waitGroup sync.WaitGroup
	var missedMessages []*discordgo.Message
	var mu sync.Mutex
	for _, guild := range guilds {
		channels, err := bot.GuildChannels(guild.ID)
		if err != nil {
			fmt.Println("Error loading channels ", err)
			return
		}

		// Async checking the channels of guild for new messages
		waitGroup.Add(1)
		go func(bot *discordgo.Session, channels []*discordgo.Channel, waitGroup *sync.WaitGroup) {
			defer waitGroup.Done()
			messages := doChannels(bot, channels, ids, waitGroup)
			mu.Lock()
			missedMessages = append(missedMessages, messages...)
			mu.Unlock()
		}(bot, channels, &waitGroup)
	}
	// Waiting for all async calls to complete
	waitGroup.Wait()
	fmt.Printf("DatabaseFix: done collecting messages found %d messages\n", len(missedMessages))
	var missed int
	for _, message := range missedMessages {
		if !slices.Contains(ids, message.ID) {
			if message.Flags != discordgo.MessageFlagsLoading &&
				message.Type != discordgo.MessageTypeGuildMemberJoin &&
				message.Type != discordgo.MessageTypeChannelPinnedMessage &&
				message.Type != discordgo.MessageTypeUserPremiumGuildSubscription &&
				message.Type != discordgo.MessageTypeUserPremiumGuildSubscriptionTierOne &&
				message.Type != discordgo.MessageTypeUserPremiumGuildSubscriptionTierTwo &&
				message.Type != discordgo.MessageTypeUserPremiumGuildSubscriptionTierThree &&
				message.Thread == nil &&
				message.Poll == nil &&
				message.StickerItems == nil &&
				!message.Author.Bot {
				if message.Type == discordgo.MessageTypeDefault && message.ReferencedMessage == nil && message.MessageReference != nil {
					continue
				}
				if message.Embeds != nil && len(message.Embeds) > 0 && message.Embeds[0].Type == "poll_result" {
					continue
				}
				if len(message.Attachments) > 0 {
					continue
				}
				database.ConstructCreateMessageObject(message, message.GuildID)
				missed++
			}
		}
	}
	c.JSON(200, gin.H{
		"message": fmt.Sprintf("done, added %d messages", missed),
	})
}
func doChannels(bot *discordgo.Session, channels []*discordgo.Channel, IDs []string, waitGroup *sync.WaitGroup) (result []*discordgo.Message) {
	var mu sync.Mutex
	for _, channel := range channels {
		// Check if channel is a guild text channel and not a voice or DM channel
		if channel.Type != discordgo.ChannelTypeGuildText {
			continue
		}

		// Async loading of the messages in that channnel
		waitGroup.Add(1)
		go func(bot *discordgo.Session, channel *discordgo.Channel) {
			defer waitGroup.Done()
			messages := loadMessages(bot, channel)
			mu.Lock()
			result = append(result, messages...)
			mu.Unlock()
		}(bot, channel)
	}
	return
}

// loadMessages loading messages from the channel
func loadMessages(Bot *discordgo.Session, channel *discordgo.Channel) (result []*discordgo.Message) {
	fmt.Printf("DatabaseFix: loading %s", channel.Name)

	// Getting last message and first 100
	messages, _ := Bot.ChannelMessages(channel.ID, int(100), "", "", "")

	// Constructing operations for first 100
	result = append(result, messages...)
	// Loading more messages if got 100 message the first time
	if len(messages) == 100 {
		lastMessageCollected := messages[len(messages)-1]
		// Loading more messages, 100 at a time
		for lastMessageCollected != nil {
			moreMes, _ := Bot.ChannelMessages(channel.ID, int(100), lastMessageCollected.ID, "", "")

			result = append(result, moreMes...)

			if len(moreMes) != 0 {
				lastMessageCollected = moreMes[len(moreMes)-1]
			} else {
				break
			}
		}
	}
	fmt.Printf("DatabaseFix: done collecting messages for %s\n", channel.Name)
	return
}
