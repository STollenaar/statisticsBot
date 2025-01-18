package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/stollenaar/statisticsbot/internal/commands"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/routes"
	"github.com/stollenaar/statisticsbot/internal/util"

	"github.com/bwmarrin/discordgo"
)

var (
	bot *discordgo.Session

	GuildID        = flag.String("guild", "", "Test guild ID. If not passed - bot registers commands globally")
	RemoveCommands = flag.Bool("rmcmd", true, "Remove all commands after shutdowning or not")
)

func init() {
	flag.Parse()

	bot, _ = discordgo.New("Bot " + util.GetDiscordToken())

	bot.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commands.CommandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
}

func main() {
	fmt.Println("STARTING DEBUGGING ISSUE")
	bot.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages | discordgo.IntentsGuildMembers)

	bot.AddHandler(database.MessageListener)
	err := bot.Open()
	if err != nil {
		log.Fatal("Error starting bot ", err)
	}

	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands.ApplicationCommands))
	for i, v := range commands.ApplicationCommands {
		cmd, err := bot.ApplicationCommandCreate(bot.State.User.ID, *GuildID, v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	database.Init(bot, GuildID)
	defer bot.Close()
	go routes.CreateRouter(bot)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	if *RemoveCommands {
		log.Println("Removing commands...")
		// We need to fetch the commands, since deleting requires the command ID.
		// We are doing this from the returned commands on line 375, because using
		// this will delete all the commands, which might not be desirable, so we
		// are deleting only the commands that we added.
		// registeredCommands, err := s.ApplicationCommands(s.State.User.ID, *GuildID)
		// if err != nil {
		// 	log.Fatalf("Could not fetch registered commands: %v", err)
		// }

		for _, v := range registeredCommands {
			err := bot.ApplicationCommandDelete(bot.State.User.ID, *GuildID, v.ID)
			if err != nil {
				log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
			}
		}
	}
}
