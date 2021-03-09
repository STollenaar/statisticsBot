package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"statsisticsbot/lib"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/nint8835/parsley"
)

var bot *discordgo.Session

func main() {
	err := godotenv.Load(".env")

	if err != nil {
		fmt.Println("Error loading environment variables")
		return
	}

	bot, err = discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))
	if err != nil {
		fmt.Println("Error loading bot ", err)
		return
	}

	bot.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	lib.Bot = bot
	bot.AddHandler(lib.MessageListener)
	parser := parsley.New("stat~")
	parser.RegisterHandler(bot)
	parser.NewCommand("ping", "pong", PingCommand)
	parser.NewCommand("count", "Returns amount of times word is used.", lib.CountCommand)
	parser.NewCommand("max", "Returns the most used word", lib.MaxCommand)
	parser.NewCommand("last", "Returns the last time a user send a message", lib.LastMessage)

	if err != nil {
		fmt.Println("Error loading command ", err)
		return
	}

	err = bot.Open()
	if err != nil {
		fmt.Println("Error starting bot ", err)
		return
	}

	lib.Init()
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	bot.Close()

}

// PingCommand sends back the pong
func PingCommand(message *discordgo.MessageCreate, args struct{}) {
	bot.ChannelMessageSend(message.ChannelID, fmt.Sprintln("Pong"))
}
