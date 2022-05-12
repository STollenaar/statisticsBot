package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"statsisticsbot/lib"
	botcommand "statsisticsbot/lib/commands"
	"statsisticsbot/lib/routes"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/nint8835/parsley"
)

var (
	bot *discordgo.Session

	GuildID        = flag.String("guild", "", "Test guild ID. If not passed - bot registers commands globally")
	RemoveCommands = flag.Bool("rmcmd", true, "Remove all commands after shutdowning or not")

	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "ping",
			Description: "pong",
		},
		{
			Name:        "count",
			Description: "Returns the amount of times a word is used.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "word",
					Description: "Word to count",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
				{
					Name:        "user",
					Description: "User to filter with",
					Type:        discordgo.ApplicationCommandOptionUser,
					Required:    false,
				},
				{
					Name:        "channel",
					Description: "Channel to filter with",
					Type:        discordgo.ApplicationCommandOptionChannel,
					Required:    false,
				},
			},
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"ping":  PingCommand,
		"count": botcommand.CountCommand,
	}
)

func init() {
	flag.Parse()
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatal("Error loading environment variables")
	}

	bot, err = discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))
	if err != nil {
		log.Fatal("Error loading bot ", err)
	}

	bot.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
}

func main() {

	bot.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	lib.Bot = bot
	bot.AddHandler(lib.MessageListener)

	err := bot.Open()
	if err != nil {
		log.Fatal("Error starting bot ", err)
	}

	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := bot.ApplicationCommandCreate(bot.State.User.ID, *GuildID, v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	parser := parsley.New("stat~")
	parser.RegisterHandler(bot)
	parser.NewCommand("ping", "pong", PingCommand)
	// parser.NewCommand("count", "Returns amount of times word is used.", commands.CountCommand)
	// parser.NewCommand("max", "Returns the most used word", commands.MaxCommand)
	// parser.NewCommand("last", "Returns the last time a user send a message", commands.LastMessage)

	lib.Init(GuildID)
	handleRequests()

	defer bot.Close()

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

// PingCommand sends back the pong
func PingCommand(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Pong",
		},
	})
}

func handleRequests() {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/userMessages/{guildID}/{userID}", routes.GetUserMessages)
	log.Fatal(http.ListenAndServe(":3000", router))
}
