package routes

import (
	"fmt"
	"net/http"
	"slices"
	"sync"

	"github.com/disgoorg/disgo/discord"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

func addFixEmojis(mux *http.ServeMux) {
	mux.HandleFunc("PUT /fixEmojis", addMissingEmojis)
}

func addMissingEmojis(w http.ResponseWriter, r *http.Request) {

	guilds := slices.Collect(client.Caches.Guilds())

	var waitGroup sync.WaitGroup
	var missedEmojis []*database.EmojiData
	var mu sync.Mutex
	for _, guild := range guilds {
		emojis := slices.Collect(client.Caches.Emojis(guild.ID))

		waitGroup.Add(1)
		go func(emojis []discord.Emoji, waitGroup *sync.WaitGroup) {
			defer waitGroup.Done()
			guildEmojis := doEmojis(emojis, guild.ID.String())
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
	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("done, added %d emojis", missed)})
}

func doEmojis(emojis []discord.Emoji, guildID string) (result []*database.EmojiData) {
	for _, emoji := range emojis {
		if database.CustomEmojiCache[emoji.Name] == "" {
			e, err := util.FetchDiscordEmojiImage(emoji.ID.String(), emoji.Animated)
			if err != nil {
				fmt.Printf("Error fetching emoji data: %v\n", err)
				continue
			}
			result = append(result, &database.EmojiData{
				ID:        emoji.ID.String(),
				Name:      emoji.Name,
				GuildID:   guildID,
				ImageData: e,
			})
		}
	}
	return
}
