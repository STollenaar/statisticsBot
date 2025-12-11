package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/statisticsbot/internal/commands"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/routes"
	"github.com/stollenaar/statisticsbot/internal/util"

	"github.com/bwmarrin/discordgo"
)

var (
	client *bot.Client

	GuildID        = flag.String("guild", "", "Test guild ID. If not passed - bot registers commands globally")
	Debug          = flag.Bool("debug", false, "Run in debug mode")
	RemoveCommands = flag.Bool("rmcmd", true, "Remove all commands after shutdowning or not")
	PurgeCommands  = flag.Bool("purgecmd", false, "Remove all loaded commands")
)

func init() {
	flag.Parse()

	c, err := disgo.New(util.ConfigFile.GetDiscordToken(),
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuildMessages|gateway.IntentGuildMembers|gateway.IntentMessageContent|gateway.IntentGuildMessageReactions,
			),
		),
		bot.WithEventListenerFunc(func(event *events.ApplicationCommandInteractionCreate) {
			data := event.Data
			if event.Data.Type() == discord.ApplicationCommandTypeSlash {
				commands.CommandHandlers[data.CommandName()](event)
			} else {
				commands.MessageCommandHandlers[data.CommandName()](event)
			}
		}),
		bot.WithEventListenerFunc(func(event *events.ComponentInteractionCreate) {
			commands.ComponentHandlers[event.Message.Interaction.Name](event)
		}),
		bot.WithEventListenerFunc(func(event *events.ModalSubmitInteractionCreate) {
			commands.ModalSubmitHandlers[event.Data.CustomID](event)
		}),

		bot.WithEventListenerFunc(database.MessageCreateListener),
		bot.WithEventListenerFunc(database.MessageUpdateListener),
		bot.WithEventListenerFunc(database.MessageReactAddListener),
		bot.WithEventListenerFunc(database.MessageReactRemoveListener),
		// bot.WithEventListenerFunc(func(event *events.ComponentInteractionCreate) {
		// 	commands.ComponentHandlers[strings.Split(event.Data.CustomID(), "_")[0]](event)
		// }),
	)

	if err != nil {
		log.Fatal(err)
	}
	client = c
	util.ConfigFile.DEBUG = *Debug
}

func main() {
	defer client.Close(context.TODO())
	var guilds []snowflake.ID
	if sn, err := snowflake.Parse(*GuildID); err == nil {
		guilds = append(guilds, sn)
	}

	if *PurgeCommands {
		if *GuildID != "" {
			cmds, err := client.Rest.GetGuildCommands(client.ApplicationID, guilds[0], false)
			if err != nil {
				log.Fatal(err)
			}
			for _, cmd := range cmds {
				err := client.Rest.DeleteGuildCommand(cmd.ApplicationID(), *cmd.GuildID(), cmd.ID())
				if err != nil {
					slog.Error(fmt.Sprintf("Cannot delete '%s' command: ", cmd.Name()), slog.Any("err", err))
				}
			}
			slog.Info("Done deleting guild commands")
		} else {
			cmds, err := client.Rest.GetGlobalCommands(client.ApplicationID, false)
			if err != nil {
				log.Fatal(err)
			}
			for _, cmd := range cmds {
				err := client.Rest.DeleteGlobalCommand(cmd.ApplicationID(), cmd.ID())
				if err != nil {
					slog.Error(fmt.Sprintf("Cannot delete '%s' command: ", cmd.Name()), slog.Any("err", err))
				}
			}
			slog.Info("Done deleting global commands")
		}
		return
	}

	slog.Info("Adding commands...")
	registeredCommands := make([]discord.ApplicationCommand, len(commands.ApplicationCommands))
	if *GuildID != "" {
		if r, err := client.Rest.SetGuildCommands(client.ApplicationID, guilds[0], commands.ApplicationCommands); err != nil {
			slog.Error("error while registering commands", slog.Any("err", err))
		} else {
			registeredCommands = r
		}
	} else {
		if r, err := client.Rest.SetGlobalCommands(client.ApplicationID, commands.ApplicationCommands); err != nil {
			slog.Error("error while registering commands", slog.Any("err", err))
		} else {
			registeredCommands = r
		}
	}
	// if err := handler.SyncCommands(client, commands.ApplicationCommands, guilds); err != nil {
	// 	log.Fatal("error while registering commands: ", err)
	// }

	if err := client.OpenGateway(context.TODO()); err != nil {
		log.Fatal("error while connecting to gateway: ", err)
	}
	slog.Info("Bot started")

	database.Init(client, GuildID)
	go routes.CreateRouter(client)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
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
			if *GuildID != "" {
				err := client.Rest.DeleteGuildCommand(v.ApplicationID(), *v.GuildID(), v.ID())
				if err != nil {
					slog.Error(fmt.Sprintf("Cannot delete '%s' command: ", v.Name()), slog.Any("err", err))
				}
			} else {
				err := client.Rest.DeleteGlobalCommand(v.ApplicationID(), v.ID())
				if err != nil {
					slog.Error(fmt.Sprintf("Cannot delete '%s' command: ", v.Name()), slog.Any("err", err))
				}
			}
		}
	}
}

func containsCommand(cmd *discordgo.ApplicationCommand, commands []*discordgo.ApplicationCommand) *discordgo.ApplicationCommand {
	for _, c := range commands {
		if cmd.Name == c.Name {
			return c
		}
	}
	return nil
}

// optionsEqual compares the Options slices of two ApplicationCommands.
func optionsEqual(cmd, registered *discordgo.ApplicationCommand) bool {
	if len(cmd.Options) != len(registered.Options) {
		return false
	}
	for i := range cmd.Options {
		if !optionEqual(cmd.Options[i], registered.Options[i]) {
			return false
		}
	}
	return true
}

// optionEqual compares two ApplicationCommandOption objects recursively.
func optionEqual(a, b *discordgo.ApplicationCommandOption) bool {
	if a.Name != b.Name ||
		a.Description != b.Description ||
		a.Type != b.Type ||
		a.Required != b.Required {
		return false
	}

	// Compare choices if available.
	if len(a.Choices) != len(b.Choices) {
		return false
	}
	for i := range a.Choices {
		if a.Choices[i].Name != b.Choices[i].Name ||
			a.Choices[i].Value != b.Choices[i].Value {
			return false
		}
	}

	// Compare sub-options recursively.
	if len(a.Options) != len(b.Options) {
		return false
	}
	for i := range a.Options {
		if !optionEqual(a.Options[i], b.Options[i]) {
			return false
		}
	}
	return true
}
