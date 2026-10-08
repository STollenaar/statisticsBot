package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/statisticsbot/internal/commands"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/routes"
	"github.com/stollenaar/statisticsbot/internal/util"
)

var (
	client *bot.Client

	gatewayReady atomic.Bool

	// guildsReady is closed once disgo has loaded every guild into its cache
	// after login, so we don't read an empty cache immediately after connecting.
	guildsReady     = make(chan struct{})
	guildsReadyOnce sync.Once

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
				gateway.IntentGuilds|gateway.IntentGuildMessages|gateway.IntentGuildMembers|gateway.IntentMessageContent|gateway.IntentGuildMessageReactions,
			),
		),
		bot.WithCacheConfigOpts(
			cache.WithCaches(
				cache.FlagGuilds,
				cache.FlagChannels,
				cache.FlagMessages,
			),
		),
		bot.WithEventListenerFunc(func(event *events.ApplicationCommandInteractionCreate) {
			data := event.Data
			if event.Data.Type() == discord.ApplicationCommandTypeSlash {
				if fn, ok := commands.CommandHandlers[data.CommandName()]; ok {
					fn(event)
				}
			} else {
				if fn, ok := commands.MessageCommandHandlers[data.CommandName()]; ok {
					fn(event)
				}
			}
		}),
		bot.WithEventListenerFunc(func(event *events.ComponentInteractionCreate) {
			if fn, ok := commands.ComponentHandlers[strings.Split(event.Message.Interaction.Name, " ")[0]]; ok {
				fn(event)
			}
		}),
		bot.WithEventListenerFunc(func(event *events.ModalSubmitInteractionCreate) {
			if fn, ok := commands.ModalSubmitHandlers[event.Data.CustomID]; ok {
				fn(event)
			}
		}),

		bot.WithEventListenerFunc(func(event *events.GuildsReady) {
			guildsReadyOnce.Do(func() { close(guildsReady) })
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
		slog.Error("failed to create Discord client", slog.Any("err", err))
		os.Exit(1)
	}
	client = c
	util.ConfigFile.DEBUG = *Debug
}

// shutdownGrace is how long in-flight HTTP requests get to finish once a
// termination signal arrives. It is deliberately well inside Kubernetes' default
// 30s terminationGracePeriodSeconds, because the database still has to be
// flushed and the commands cleaned up after this returns — spending the whole
// budget draining would trade a cut-off request for an unflushed database.
const shutdownGrace = 15 * time.Second

// startHealthServer starts the liveness/readiness server in the background and
// returns it so it can be shut down with everything else.
func startHealthServer(port string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("ok"))
		if err != nil {
			slog.Error("Error writing ok", slog.Any("err", err.Error()))
		}
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if gatewayReady.Load() {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("ok"))
			if err != nil {
				slog.Error("Error writing ok", slog.Any("err", err.Error()))
			}
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, err := w.Write([]byte("not ready"))
			if err != nil {
				slog.Error("Error writing ready", slog.Any("err", err.Error()))
			}

		}
	})
	srv := &http.Server{Addr: ":" + port, Handler: mux}
	go func() {
		slog.Info("Health server listening", slog.String("port", port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Health server failed", slog.Any("err", err))
		}
	}()
	return srv
}

// shutdownHTTP stops the given servers accepting new connections and waits for
// in-flight requests, up to shutdownGrace shared across all of them.
func shutdownHTTP(servers ...*http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()

	var wg sync.WaitGroup
	for _, srv := range servers {
		if srv == nil {
			continue
		}
		wg.Add(1)
		go func(srv *http.Server) {
			defer wg.Done()
			if err := srv.Shutdown(ctx); err != nil {
				// Most likely the grace period expired with requests still
				// running; they are about to be cut off either way.
				slog.Error("server did not drain in time", slog.String("addr", srv.Addr), slog.Any("err", err))
				return
			}
			slog.Info("server stopped", slog.String("addr", srv.Addr))
		}(srv)
	}
	wg.Wait()
}

func main() {
	// Deferred LIFO: database.Exit runs before the gateway is closed, and both
	// run on every return path (including the PurgeCommands early return).
	// database.Exit is idempotent, so the explicit call during signal shutdown
	// below does not double-close.
	defer client.Close(context.TODO())
	defer database.Exit()

	var guilds []snowflake.ID
	if sn, err := snowflake.Parse(*GuildID); err == nil {
		guilds = append(guilds, sn)
	}

	if *PurgeCommands {
		if *GuildID != "" {
			cmds, err := client.Rest.GetGuildCommands(client.ApplicationID, guilds[0], false)
			if err != nil {
				slog.Error("failed to fetch guild commands", slog.Any("err", err))
				os.Exit(1)
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
				slog.Error("failed to fetch global commands", slog.Any("err", err))
				os.Exit(1)
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

	healthServer := startHealthServer(util.ConfigFile.HEALTH_PORT)

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
		slog.Error("error while connecting to gateway", slog.Any("err", err))
		os.Exit(1)
	}

	slog.Info("Bot started, waiting for guilds to load...")
	select {
	case <-guildsReady:
		slog.Info("Guilds loaded")
	case <-time.After(30 * time.Second):
		slog.Warn("Timed out waiting for guilds to load; continuing anyway")
	}

	gatewayReady.Store(true)

	database.Init(client, GuildID)
	apiServer := routes.CreateRouter(client)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	slog.Info("Shutting down...")

	// Stop serving before closing the database. A request still in flight would
	// otherwise find the connection closed underneath it, which is how an
	// in-progress backup used to die mid-upload on a rolling deploy.
	shutdownHTTP(apiServer, healthServer)

	// Then flush the database, before the (potentially slow) command cleanup
	// below risks running long enough for the platform to send SIGKILL.
	database.Exit()

	if *RemoveCommands {
		slog.Info("Removing commands...")
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
