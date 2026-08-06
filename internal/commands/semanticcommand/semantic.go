package semanticcommand

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"github.com/google/uuid"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/embeddings"
	"github.com/stollenaar/statisticsbot/internal/util"
)

const (
	// resultsPerPage is how many matches are shown at once.
	resultsPerPage = 5
	// defaultPoolSize / maxPoolSize bound how many ranked matches are fetched
	// and then paginated over.
	defaultPoolSize  = 20
	maxPoolSize      = 50
	maxContentLength = 300
	// sessionTTL is how long a search's results stay navigable before its
	// pagination buttons expire.
	sessionTTL = 15 * time.Minute
)

var (
	SemanticCmd = SemanticCommand{
		Name:        "semantic",
		Description: "semantic search command",
	}

	// sessions caches each search's ranked results so pagination buttons can
	// slice into them without re-running the search. Keyed by a token embedded
	// in the button custom IDs.
	sessions   = make(map[string]*searchSession)
	sessionsMu sync.Mutex
)

type SemanticCommand struct {
	Name        string
	Description string
}

type searchSession struct {
	query   string
	guildID string
	results []database.SemanticSearchResult
	created time.Time
}

func (s SemanticCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	err := event.DeferCreateMessage(util.ConfigFile.SetEphemeral() == discord.MessageFlagEphemeral)
	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}

	sub := event.SlashCommandInteractionData()
	query := sub.Options["query"].String()

	poolSize := defaultPoolSize
	if opt, ok := sub.Options["limit"]; ok {
		if n := opt.Int(); n > 0 {
			poolSize = n
		}
	}
	if poolSize > maxPoolSize {
		poolSize = maxPoolSize
	}

	// Embed the query with the same model used for stored messages.
	vec, err := embeddings.Embed(query)
	if err != nil {
		slog.Error("semantic embedding error", slog.Any("err", err))
		s.editError(event, "error happened while embedding the query")
		return
	}

	results, err := database.SearchSimilarMessages(event.GuildID().String(), vec, embeddings.ModelName(), poolSize)
	if err != nil {
		slog.Error("semantic search error", slog.Any("err", err))
		s.editError(event, "error happened while searching for messages")
		return
	}

	if len(results) == 0 {
		s.editError(event, "no matching messages found (has the history been embedded yet?)")
		return
	}

	token := uuid.New().String()
	sess := &searchSession{
		query:   query,
		guildID: event.GuildID().String(),
		results: results,
		created: time.Now(),
	}
	storeSession(token, sess)

	embed, components := renderResults(token, sess, 1)
	updateResponse(event.Client().Rest, event.ApplicationID(), event.Token(), embed, components)
}

// ComponentHandler drives the pagination buttons. Custom ID format:
// semantic_page_<token>_<page>
func (s SemanticCommand) ComponentHandler(event *events.ComponentInteractionCreate) {
	if err := event.DeferUpdateMessage(); err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}

	parts := strings.SplitN(event.Data.CustomID(), "_", 4)
	if len(parts) < 4 || parts[1] != "page" {
		return
	}
	token := parts[2]
	page, err := strconv.Atoi(parts[3])
	if err != nil {
		return
	}

	sess, ok := getSession(token)
	if !ok {
		expired := "this search has expired, please run `/semantic` again"
		_, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Content:    &expired,
			Embeds:     &[]discord.Embed{},
			Components: &[]discord.LayoutComponent{},
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err))
		}
		return
	}

	embed, components := renderResults(token, sess, page)
	updateResponse(event.Client().Rest, event.ApplicationID(), event.Token(), embed, components)
}

// renderResults builds the embed and pagination buttons for a single page of a
// cached search. Pages are 1-indexed and clamped to the valid range.
func renderResults(token string, sess *searchSession, page int) (discord.Embed, []discord.LayoutComponent) {
	totalPages := (len(sess.results) + resultsPerPage - 1) / resultsPerPage
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * resultsPerPage
	end := start + resultsPerPage
	if end > len(sess.results) {
		end = len(sess.results)
	}

	embed := discord.Embed{
		Title: fmt.Sprintf("Semantic search: %q", sess.query),
		Footer: &discord.EmbedFooter{
			Text: fmt.Sprintf("Page %d/%d — %d results", page, totalPages, len(sess.results)),
		},
	}
	for _, r := range sess.results[start:end] {
		link := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", sess.guildID, r.ChannelID, r.MessageID)

		author := r.AuthorID
		if id, parseErr := snowflake.Parse(r.AuthorID); parseErr == nil {
			author = discord.UserMention(id)
		}

		embed.Fields = append(embed.Fields, discord.EmbedField{
			Name: fmt.Sprintf("<t:%d>", r.Date.UTC().Unix()),
			Value: fmt.Sprintf("%s\n%s — [jump](%s)",
				truncate(r.Content, maxContentLength), author, link),
		})
	}

	var components []discord.LayoutComponent
	if totalPages > 1 {
		components = append(components, discord.ActionRowComponent{
			Components: []discord.InteractiveComponent{
				discord.ButtonComponent{
					Style:    discord.ButtonStyleSecondary,
					Label:    "← Previous",
					CustomID: fmt.Sprintf("semantic_page_%s_%d", token, page-1),
					Disabled: page <= 1,
				},
				discord.ButtonComponent{
					Style:    discord.ButtonStyleSecondary,
					Label:    "Next →",
					CustomID: fmt.Sprintf("semantic_page_%s_%d", token, page+1),
					Disabled: page >= totalPages,
				},
			},
		})
	}
	return embed, components
}

// updateResponse edits the interaction's original response with the given embed
// and components, suppressing mention pings.
func updateResponse(client rest.Rest, appID snowflake.ID, token string, embed discord.Embed, components []discord.LayoutComponent) {
	_, err := client.UpdateInteractionResponse(appID, token, discord.MessageUpdate{
		Embeds:          &[]discord.Embed{embed},
		Components:      &components,
		AllowedMentions: &discord.AllowedMentions{},
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
}

// editError replaces the deferred response with a plain error message.
func (s SemanticCommand) editError(event *events.ApplicationCommandInteractionCreate, msg string) {
	_, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Content: &msg,
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
}

func (s SemanticCommand) CreateCommandArguments() []discord.ApplicationCommandOption {
	return []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "query",
			Description: "what to search for",
			Required:    true,
		},
		discord.ApplicationCommandOptionInt{
			Name:        "limit",
			Description: fmt.Sprintf("how many matches to fetch and page through (max %d)", maxPoolSize),
			Required:    false,
		},
	}
}

// storeSession caches a search's results and prunes any expired ones.
func storeSession(token string, sess *searchSession) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	pruneLocked()
	sessions[token] = sess
}

// getSession returns a cached search by token, pruning expired ones first.
func getSession(token string) (*searchSession, bool) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	pruneLocked()
	sess, ok := sessions[token]
	return sess, ok
}

// pruneLocked drops sessions older than sessionTTL. Callers must hold sessionsMu.
func pruneLocked() {
	cutoff := time.Now().Add(-sessionTTL)
	for k, v := range sessions {
		if v.created.Before(cutoff) {
			delete(sessions, k)
		}
	}
}

// truncate shortens s to at most n runes, appending an ellipsis when cut.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
