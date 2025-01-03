package routes

import (
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
)

var (
	bot *discordgo.Session
)

func CreateRouter(b *discordgo.Session) {
	bot = b
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	addGetUserMessages(r)
	addFixDatabase(r)
	r.Run()
}
