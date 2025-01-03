package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
	"github.com/stollenaar/statisticsbot/internal/database"
)

const DiscordEpoch int64 = 1420070400000

func addFixDatabase(r *gin.Engine) {
	r.DELETE("/fixDatabase", deleteBadEntries)
	r.PATCH("/fixDatabase", ensureParity)
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
			snflk, err := snowflakeToTimestamp(message_id)
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
			if message.Embeds != nil && message.Embeds[0].Type == "poll_result" {
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
	database.DeleteMilvus("id in " + fmt.Sprintf("%v", deletedIDs))
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

func ensureParity(c *gin.Context) {
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
	database.DeleteMilvus("id not in " + fmt.Sprintf("%v", ids))
}

// SnowflakeToTimestamp converts a Discord snowflake ID to a timestamp
func snowflakeToTimestamp(snowflakeID string) (time.Time, error) {
	id, err := strconv.ParseInt(snowflakeID, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	timestamp := (id >> 22) + DiscordEpoch
	return time.Unix(0, timestamp*int64(time.Millisecond)), nil
}
