package routes

import (
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
	"github.com/stollenaar/statisticsbot/internal/util"
)

var (
	bot *discordgo.Session
)

func CreateRouter(b *discordgo.Session) {
	bot = b
	if !util.ConfigFile.DEBUG {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.SetTrustedProxies(nil)
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	addGetUserMessages(r)
	addFixMessages(r)
	addFixEmojis(r)
	r.Run()
}
