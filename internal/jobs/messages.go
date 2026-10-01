package jobs

import (
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/statisticsbot/internal/database"
)

// messagePageSize is Discord's maximum page size for channel history.
const messagePageSize = 100

// Counter names reported by the message sync, alongside the generic
// done/total/failed that count channels.
const (
	CounterAdded     = "added"
	CounterReactions = "reactions"
)

var syncJob = &Job{name: "message sync", unit: "channels"}

// StartMessageSync walks every message-bearing channel's full history and stores
// anything the database does not already have, returning immediately. It reports
// false when a sync is already running.
func StartMessageSync(client *bot.Client) bool {
	if !syncJob.tryStart("loading") {
		return false
	}
	go runMessageSync(client)
	return true
}

// MessageSync returns the progress of the running or last-finished sync.
func MessageSync() Status {
	return syncJob.status()
}

// run scans every message-bearing channel in every cached guild.
//
// Channels are scanned concurrently, one goroutine each, which is how this has
// always worked; disgo's REST client handles the rate limiting.
func runMessageSync(client *bot.Client) {
	defer syncJob.recoverPanic()

	known, reactions, err := loadKnown()
	if err != nil {
		syncJob.fail(err)
		return
	}

	// Collect the channels up front so the job can report a total to page against.
	var channels []discord.GuildChannel
	for _, guild := range slices.Collect(client.Caches.Guilds()) {
		for _, channel := range slices.Collect(client.Caches.ChannelsForGuild(guild.ID)) {
			// Every channel that can hold messages, not just plain text channels.
			// See initChannels in internal/database/database.go.
			if _, ok := channel.(discord.GuildMessageChannel); !ok {
				continue
			}
			channels = append(channels, channel)
		}
	}
	syncJob.begin("scanning", len(channels))
	slog.Info("message sync started",
		slog.Int("channels", len(channels)), slog.Int("knownMessages", len(known)))

	var wg sync.WaitGroup
	for _, channel := range channels {
		wg.Add(1)
		go func(channel discord.GuildChannel) {
			defer wg.Done()
			added, reacted, err := syncChannel(client, channel, known, reactions)
			syncJob.add(CounterAdded, added)
			syncJob.add(CounterReactions, reacted)
			syncJob.record(err)
		}(channel)
	}
	wg.Wait()

	syncJob.finish(PhaseDone, nil)
	s := syncJob.status()
	slog.Info("message sync finished",
		slog.Int("added", s.Counter(CounterAdded)), slog.Int("reactions", s.Counter(CounterReactions)),
		slog.Int("channels", s.Done), slog.Int("errors", s.Failed),
		slog.String("took", s.Elapsed))
}

// loadKnown reads the ids the database already holds, as sets so each channel's
// scan is a map lookup instead of a rescan of a million-element slice.
func loadKnown() (messages map[string]struct{}, reactions map[string]struct{}, err error) {
	messages = make(map[string]struct{})
	reactions = make(map[string]struct{})

	rows, err := database.QueryDuckDB(`
		SELECT id FROM messages
		UNION ALL
		SELECT id FROM bot_messages`, nil)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, nil, err
		}
		messages[id] = struct{}{}
	}
	rows.Close()

	rows, err = database.QueryDuckDB(`SELECT id, author_id, reaction FROM reactions`, nil)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, authorID, reaction string
		if err := rows.Scan(&id, &authorID, &reaction); err != nil {
			return nil, nil, err
		}
		reactions[reactionKey(id, authorID, reaction)] = struct{}{}
	}
	return messages, reactions, rows.Err()
}

func reactionKey(id, authorID, reaction string) string {
	return fmt.Sprintf("%s_%s_%s", id, authorID, reaction)
}

// syncChannel walks one channel's history and stores whatever is missing. The
// known sets are read-only here, so sharing them across goroutines is safe.
func syncChannel(client *bot.Client, channel discord.GuildChannel, known, knownReactions map[string]struct{}) (added, reacted int, err error) {
	slog.Info("message sync: loading channel",
		slog.String("guild", channel.GuildID().String()), slog.String("channel", channel.Name()))

	var fetched int
	var before snowflake.ID
	for {
		batch, batchErr := client.Rest.GetMessages(channel.ID(), 0, before, 0, messagePageSize)
		if batchErr != nil {
			slog.Error("message sync: failed to fetch messages",
				slog.String("guild", channel.GuildID().String()),
				slog.String("channel", channel.Name()), slog.Any("err", batchErr))
			err = batchErr
			break
		}
		if len(batch) == 0 {
			break
		}
		fetched += len(batch)

		for _, message := range batch {
			if _, seen := known[message.ID.String()]; seen {
				continue
			}
			a, r := storeMessage(channel, message, knownReactions)
			added += a
			reacted += r
		}

		// The last element is the oldest message in the batch; page from it.
		before = batch[len(batch)-1].ID
		if len(batch) < messagePageSize {
			break
		}
	}

	slog.Info("message sync: done collecting messages",
		slog.String("guild", channel.GuildID().String()), slog.String("channel", channel.Name()),
		slog.Int("fetched", fetched), slog.Int("added", added))
	return added, reacted, err
}

// storeMessage applies the ingestion rules to one message Discord has and the
// database does not.
func storeMessage(channel discord.GuildChannel, message discord.Message, knownReactions map[string]struct{}) (added, reacted int) {
	for _, reaction := range message.Reactions {
		if reaction.Emoji.Creator == nil {
			continue
		}
		key := reactionKey(message.ID.String(), reaction.Emoji.Creator.ID.String(), reaction.Emoji.Name)
		if _, ok := knownReactions[key]; ok {
			continue
		}
		database.ConstructMessageReactObject(database.MessageReact{
			ID:        message.ID.String(),
			GuildID:   message.GuildID.String(),
			ChannelID: message.ChannelID.String(),
			Author:    reaction.Emoji.Creator.ID.String(),
			Reaction:  reaction.Emoji.Name,
		}, false)
		reacted++
	}

	if message.Flags == discord.MessageFlagLoading ||
		message.Type == discord.MessageTypeUserJoin ||
		message.Type == discord.MessageTypeChannelPinnedMessage ||
		message.Type == discord.MessageTypeGuildBoost ||
		message.Type == discord.MessageTypeGuildBoostTier1 ||
		message.Type == discord.MessageTypeGuildBoostTier2 ||
		message.Type == discord.MessageTypeGuildBoostTier3 ||
		message.Thread != nil ||
		message.Poll != nil ||
		message.StickerItems != nil {
		return added, reacted
	}

	// These three only exist to drop messages that carry no text at all — a bare
	// image post, a poll result, a reply whose referenced message REST did not
	// populate. deleteBadMessages guards the same checks the same way.
	if database.MessageContent(message) == "" {
		if message.Type == discord.MessageTypeDefault && message.ReferencedMessage == nil && message.MessageReference != nil {
			return added, reacted
		}
		if len(message.Embeds) > 0 && message.Embeds[0].Type == "poll_result" {
			return added, reacted
		}
		if len(message.Attachments) > 0 {
			return added, reacted
		}
	}

	database.ConstructCreateMessageObject(message, channel.GuildID().String(), message.Author.Bot)
	return added + 1, reacted
}
