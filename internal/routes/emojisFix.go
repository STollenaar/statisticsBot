package routes

import (
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

func addFixEmojis(r *gin.Engine) {
	r.PUT("/fixEmojis", addMissingEmojis)
}

func addMissingEmojis(c *gin.Context) {

	guilds, err := bot.UserGuilds(100, "", "", false)
	if err != nil {
		fmt.Println(err)
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	var waitGroup sync.WaitGroup
	var missedEmojis []*database.EmojiData
	var mu sync.Mutex
	for _, guild := range guilds {
		emojis, err := bot.GuildEmojis(guild.ID)
		if err != nil {
			fmt.Println(err)
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}
		waitGroup.Add(1)
		go func(emojis []*discordgo.Emoji, waitGroup *sync.WaitGroup) {
			defer waitGroup.Done()
			guildEmojis := doEmojis(emojis, guild.ID)
			mu.Lock()
			missedEmojis = append(missedEmojis, guildEmojis...)
			mu.Unlock()
		}(emojis, &waitGroup)
	}
	waitGroup.Wait()

	var missed int
	for _, emoji := range missedEmojis {
		database.ConstructEmojiObject(*emoji)
		missed++
	}
	c.JSON(200, gin.H{
		"message": fmt.Sprintf("done, added %d emojis", missed),
	})
}

func doEmojis(emojis []*discordgo.Emoji, guildID string) (result []*database.EmojiData) {
	for _, emoji := range emojis {
		if emoji.ID != "" && database.CustomEmojiCache[emoji.Name] == "" {
			e, err := util.FetchDiscordEmojiImage(emoji.ID, emoji.Animated)
			if err != nil {
				fmt.Printf("Error fetching emoji data: %v\n", err)
				continue
			}
			result = append(result, &database.EmojiData{
				ID:        emoji.ID,
				Name:      emoji.Name,
				GuildID:   guildID,
				ImageData: e,
			})
		}
	}
	return
}
