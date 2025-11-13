package database

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/stollenaar/statisticsbot/internal/util"

	"github.com/bwmarrin/discordgo"

	_ "github.com/marcboeker/go-duckdb/v2" // DuckDB Go driver
)

var (
	duckdbClient *sql.DB

	CustomEmojiCache = make(map[string]string)

	//go:embed changelog/*.sql
	changeLogFiles embed.FS
)

// Define the request and response structures
type TextRequest struct {
	Text string `json:"text"`
}

type MessageReact struct {
	ID        string
	GuildID   string
	ChannelID string
	Author    string
	Reaction  string
}

type EmojiData struct {
	ID        string
	GuildID   string
	Name      string
	ImageData string
}

func init() {
	initDuckDB()
	loadCache()
}

func Exit() {
	duckdbClient.Close()
}

func initDuckDB() {
	var err error

	duckdbClient, err = sql.Open("duckdb", fmt.Sprintf("%s/statsbot.db", util.ConfigFile.DUCKDB_PATH)) // Create or connect to messages.db

	if err != nil {
		log.Fatal(err)
	}

	// Ensure changelog table exists
	_, err = duckdbClient.Exec(`
	CREATE TABLE IF NOT EXISTS database_changelog (
		id INTEGER PRIMARY KEY,
		name VARCHAR NOT NULL,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		checksum VARCHAR,
		success BOOLEAN DEFAULT TRUE
	);
	`)

	if err != nil {
		log.Fatalf("failed to create changelog table: %v", err)
	}

	if err := runMigrations(); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	log.Println("All migrations applied successfully.")
}

func runMigrations() error {
	entries, err := changeLogFiles.ReadDir("changelog")
	if err != nil {
		return fmt.Errorf("failed to read embedded changelogs: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}

	sort.Strings(files)

	for i, file := range files {
		id := i + 1

		contents, err := changeLogFiles.ReadFile(filepath.Join("changelog", file))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		checksum := sha256.Sum256(contents)
		checksumHex := hex.EncodeToString(checksum[:])

		var appliedChecksum string
		err = duckdbClient.QueryRow("SELECT checksum FROM database_changelog WHERE id = ?", id).Scan(&appliedChecksum)
		if err == nil {
			if appliedChecksum != checksumHex {
				return fmt.Errorf("checksum mismatch for migration %s (id=%d). File has changed", file, id)
			}
			log.Printf("Skipping already applied migration %s", file)
			continue
		}

		// Run changelogs in a transaction
		tx, err := duckdbClient.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin tx: %w", err)
		}

		_, err = tx.Exec(string(contents))
		if err != nil {
			_ = tx.Rollback()
			_, _ = duckdbClient.Exec(`
				INSERT INTO database_changelog (id, name, applied_at, checksum, success) VALUES (?, ?, ?, ?, false)
			`, id, file, time.Now(), checksumHex)
			return fmt.Errorf("failed to apply migration %s: %w", file, err)
		}

		err = tx.Commit()
		if err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", file, err)
		}

		_, err = duckdbClient.Exec(`
			INSERT INTO database_changelog (id, name, applied_at, checksum, success)
			VALUES (?, ?, ?, ?, true)
		`, id, file, time.Now(), checksumHex)
		if err != nil {
			return fmt.Errorf("failed to record migration %s: %w", file, err)
		}

		log.Printf("Applied migration %s", file)
	}

	return nil
}

func loadCache() {
	rs, err := duckdbClient.Query(`
		SELECT name,image_data AS image FROM emojis;
	`)
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	for rs.Next() {
		var name, image string
		err = rs.Scan(&name, &image)
		if err != nil {
			fmt.Printf("Error parsing: %v\n", err)
			continue
		}
		CustomEmojiCache[name] = image
	}
}

// Init doing the initialization of all the messages
func Init(bot *discordgo.Session, GuildID *string) {
	guilds, err := bot.UserGuilds(100, "", "", false)
	if err != nil {
		fmt.Println(err)
	}

	// TODO: Probably reformat this
	if GuildID != nil {
		for _, v := range guilds {
			if v.ID == *GuildID {
				guilds = []*discordgo.UserGuild{}
				guilds = append(guilds, v)
				break
			}
		}
	}

	var waitGroup sync.WaitGroup
	for _, guild := range guilds {
		channels, err := bot.GuildChannels(guild.ID)
		if err != nil {
			fmt.Println("Error loading channels ", err)
			return
		}

		// Async checking the channels of guild for new messages
		waitGroup.Add(1)
		go func(bot *discordgo.Session, channels []*discordgo.Channel, waitGroup *sync.WaitGroup) {
			defer waitGroup.Done()
			initChannels(bot, channels, waitGroup)
		}(bot, channels, &waitGroup)
	}

	// Waiting for all async calls to complete
	waitGroup.Wait()
	fmt.Println("Done loading guilds")
}

// initChannels loading all the channels of the guild
func initChannels(bot *discordgo.Session, channels []*discordgo.Channel, waitGroup *sync.WaitGroup) {
	for _, channel := range channels {
		fmt.Printf("Checking %s \n", channel.Name)
		// Check if channel is a guild text channel and not a voice or DM channel
		if channel.Type != discordgo.ChannelTypeGuildText {
			continue
		}

		// Async loading of the messages in that channnel
		waitGroup.Add(1)
		go func(bot *discordgo.Session, channel *discordgo.Channel) {
			defer waitGroup.Done()
			loadMessages(bot, channel)
		}(bot, channel)
	}
}

// getLastMessage gets the last message in provided channel from the database
func getLastMessage(channel *discordgo.Channel) (lastMessage util.MessageObject) {

	// Query to find the most recent message per channel
	query := `
		SELECT id, date
		FROM (
			SELECT id, date
			FROM bot_messages
			WHERE channel_id = ?
		
			UNION ALL
		
			SELECT id, date
			FROM messages
			WHERE channel_id = ?
		) AS all_msgs
		ORDER BY date DESC
		LIMIT 1;
	`

	// Execute the query'
	row := duckdbClient.QueryRow(query, channel.ID, channel.ID)

	var (
		id   string
		date time.Time
	)

	err := row.Scan(&id, &date)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("No messages found for channel_id: %s\n", channel.ID)
		} else {
			log.Fatalf("Query failed: %v", err)
		}
		return
	}
	lastMessage.Date = date
	lastMessage.MessageID = id

	return
}

// loadMessages loading messages from the channel
func loadMessages(Bot *discordgo.Session, channel *discordgo.Channel) {
	fmt.Println("Loading ", channel.Name)
	defer util.Elapsed(channel.Name)() // timing how long it took to collect the messages
	// collection := client.Database("statistics_bot").Collection(channel.GuildID)
	var operations int

	// Getting last message and first 100
	lastMessage := getLastMessage(channel)
	messages, _ := Bot.ChannelMessages(channel.ID, int(100), "", "", "")
	messages = util.FilterDiscordMessages(messages, func(message *discordgo.Message) bool {
		messageTime, _ := util.SnowflakeToTimestamp(message.ID)

		return messageTime.After(lastMessage.Date)
	})

	// Constructing operations for first 100
	for _, message := range messages {
		operations++
		ConstructCreateMessageObject(message, channel.GuildID, message.Author.Bot)
		for _, reaction := range message.Reactions {
			if reaction.Emoji.User == nil {
				continue
			}
			ConstructMessageReactObject(MessageReact{
				ID:        message.ID,
				GuildID:   channel.GuildID,
				ChannelID: message.ChannelID,
				Author:    reaction.Emoji.User.ID,
				Reaction:  reaction.Emoji.Name,
			}, false)
		}
	}

	// Loading more messages if got 100 message the first time
	if len(messages) == 100 {
		lastMessageCollected := messages[len(messages)-1]
		// Loading more messages, 100 at a time
		for lastMessageCollected != nil {
			moreMes, _ := Bot.ChannelMessages(channel.ID, int(100), lastMessageCollected.ID, "", "")
			moreMes = util.FilterDiscordMessages(moreMes, func(message *discordgo.Message) bool {
				messageTime, _ := util.SnowflakeToTimestamp(message.ID)

				return messageTime.After(lastMessage.Date)
			})

			for _, message := range moreMes {
				operations++
				ConstructCreateMessageObject(message, channel.GuildID, message.Author.Bot)
			}
			if len(moreMes) != 0 {
				lastMessageCollected = moreMes[len(moreMes)-1]
			} else {
				break
			}
		}
	}

	fmt.Printf("Done collecting messages for %s, found %d messages\n", channel.Name, operations)
}

// constructing the message object from the received discord message, ready for inserting into database
func ConstructCreateMessageObject(message *discordgo.Message, guildID string, isBot bool) {

	var content []string
	if message.Content == "" && len(message.Embeds) > 0 {
		for _, embed := range message.Embeds {
			if embed.Description != "" {
				content = append(content, embed.Description)
			}
			if len(embed.Fields) > 0 {
				for _, field := range embed.Fields {
					content = append(content, field.Name)
					content = append(content, field.Value)
				}
			}
			if footer := embed.Footer; footer != nil && footer.Text != "" {
				content = append(content, footer.Text)
			}
		}
	} else {
		content = []string{message.Content}
	}

	// Prepare the content as a single string (for simplicity, we join it)
	contentStr := strings.Join(content, "\n")
	timestamp, err := util.SnowflakeToTimestamp(message.ID)
	if err != nil {
		fmt.Printf("Error converting snowflake to timestamp: %s\n", err)
	}
	table := "messages"
	if isBot {
		table = "bot_messages"
	}

	columns := []string{"id", "guild_id", "channel_id", "author_id", "content", "date", "version"}
	values := []string{"?", "?", "?", "?", "?", "?", "1"}
	args := []any{message.ID, guildID, message.ChannelID, message.Author.ID, contentStr, timestamp}
	if message.MessageReference != nil {
		args = append(args, message.MessageReference.MessageID)
		columns = append(columns, "reply_message_id")
		values = append(values, "?")
	}

	if message.Interaction != nil {
		args = append(args, message.Interaction.User.ID)
		columns = append(columns, "interaction_author_id")
		values = append(values, "?")
	}

	// Increment the version and insert the updated message
	_, err = duckdbClient.Exec(fmt.Sprintf(`INSERT INTO %s (%s) 
                                VALUES (%s)`, table, strings.Join(columns, ","), strings.Join(values, ",")), args...)
	if err != nil {
		fmt.Printf("Error inserting into duckdb: %s\n", err)
	}
}

func constructUpdateMessageObject(message *discordgo.Message, guildID string, isBot bool) {
	var content []string
	if message.Content == "" && len(message.Embeds) > 0 {
		for _, embed := range message.Embeds {
			if embed.Description != "" {
				content = append(content, embed.Description)
			}
			if len(embed.Fields) > 0 {
				for _, field := range embed.Fields {
					content = append(content, field.Name)
					content = append(content, field.Value)
				}
			}
			if footer := embed.Footer; footer != nil && footer.Text != "" {
				content = append(content, footer.Text)
			}
		}
	} else {
		content = []string{message.Content}
	}

	// Prepare the content as a single string (for simplicity, we join it)
	contentStr := strings.Join(content, "\n")
	timestamp, err := util.SnowflakeToTimestamp(message.ID)
	if err != nil {
		fmt.Printf("Error converting snowflake to timestamp: %s\n", err)
	}

	table := "messages"
	if isBot {
		table = "bot_messages"
	}

	var maxVersion int
	err = duckdbClient.QueryRow(fmt.Sprintf(`
    SELECT COALESCE(MAX(version) + 1, 1) FROM %s WHERE id = ? AND guild_id = ?`, table), message.ID, guildID).Scan(&maxVersion)

	if err != nil {
		fmt.Println("Error fetching max version:", err)
	}

	columns := []string{"id", "guild_id", "channel_id", "author_id", "content", "date", "version"}
	values := []string{"?", "?", "?", "?", "?", "?", "?"}
	args := []any{message.ID, guildID, message.ChannelID, message.Author.ID, contentStr, timestamp, maxVersion}
	if message.MessageReference != nil {
		args = append(args, message.MessageReference.MessageID)
		columns = append(columns, "reply_message_id")
		values = append(values, "?")
	}

	if message.Interaction != nil {
		args = append(args, message.Interaction.User.ID)
		columns = append(columns, "interaction_author_id")
		values = append(values, "?")
	}

	// Increment the version and insert the updated message
	_, err = duckdbClient.Exec(fmt.Sprintf(`INSERT INTO %s (%s) 
                                VALUES (%s)`, table, strings.Join(columns, ","), strings.Join(values, ",")), args...)

	if err != nil {
		fmt.Printf("Error inserting updated message into DuckDB: %s\n", err)
	}
}

func ConstructMessageReactObject(message MessageReact, delete bool) {
	timestamp, err := util.SnowflakeToTimestamp(message.ID)
	if err != nil {
		fmt.Printf("Error converting snowflake to timestamp: %s\n", err)
	}
	if !delete {

		// insert the reaction to the message
		_, err = duckdbClient.Exec(`INSERT INTO reactions (id, guild_id, channel_id, author_id, reaction, date) 
                                VALUES (?, ?, ?, ?, ?, ?)`,
			message.ID, message.GuildID, message.ChannelID, message.Author, message.Reaction, timestamp)

		if err != nil {
			fmt.Printf("Error inserting reaction add into DuckDB: %s\n", err)
		}
	} else {
		// insert the reaction to the message
		_, err := duckdbClient.Exec(`DELETE FROM reactions WHERE id = ? AND author_id = ? AND reaction = ?`,
			message.ID, message.Author, message.Reaction)
		if err != nil {
			fmt.Printf("Error inserting reaction add into DuckDB: %s\n", err)
		}
	}
}

func ConstructEmojiObject(message EmojiData) {

	// insert the reaction to the message
	_, err := duckdbClient.Exec(`INSERT INTO emojis (id, guild_id, name, image_data) 
                                VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING;`,
		message.ID, message.GuildID, message.Name, message.ImageData)

	if err != nil {
		fmt.Printf("Error inserting reaction add into DuckDB: %s\n", err)
	}
	CustomEmojiCache[message.Name] = message.ImageData
}

// Get a result from the database using a filter
func QueryDuckDB(query string, params []interface{}) (results *sql.Rows, err error) {
	if util.ConfigFile.DEBUG {
		// Prepare the query with the parameters replaced (for debugging)
		interpolatedQuery := query
		for _, param := range params {
			// Handle specific types (e.g., string, time.Time, etc.) appropriately
			var paramStr string
			switch v := param.(type) {
			case string:
				paramStr = fmt.Sprintf("'%s'", v) // Surround strings with quotes
			case time.Time:
				paramStr = fmt.Sprintf("'%s'", v.Format("2006-01-02 15:04:05")) // Format time
			default:
				paramStr = fmt.Sprintf("%v", v) // Default to using %v for other types
			}
			// Replace the first `?` with the actual value
			interpolatedQuery = strings.Replace(interpolatedQuery, "?", paramStr, 1)
		}

		fmt.Println("Executing query:", interpolatedQuery)
	}

	return duckdbClient.Query(query, params...)
}

// Execute a query on the database
func ExecDuckDB(query string, params []interface{}) (results sql.Result, err error) {

	return duckdbClient.Exec(query, params...)
}

func StartTX() (*sql.Tx, error) {
	return duckdbClient.Begin()
}

func CountFilterOccurences(filter, word string, params []interface{}) (messageObjects []util.CountGrouped, err error) {
	query := `
		WITH latest_versions AS (
			SELECT m.*
			FROM messages m
			JOIN (
				SELECT id, MAX(version) AS latest_version
				FROM messages
				GROUP BY id
			) latest
				ON m.id = latest.id AND m.version = latest.latest_version
		),
		tokenized_messages AS (
			SELECT 
				author_id,
				guild_id,
				LOWER(unnest(string_split(regexp_replace(content, '[^a-zA-Z0-9'' ]', '', 'g'), ' '))) AS word
			FROM latest_versions
			%s
		)
		SELECT 
			guild_id,
			author_id,
			word,
			COUNT(*) AS word_count
		FROM tokenized_messages
		%s
		GROUP BY author_id, guild_id, word
		ORDER BY word_count DESC;
	`

	tokenFilter := `WHERE %s`
	wordFilter := `WHERE word != '' AND word = LOWER(?)`
	var q string
	if word != "" {
		q = fmt.Sprintf(query, fmt.Sprintf(tokenFilter, filter), wordFilter)
	} else {
		q = fmt.Sprintf(query, fmt.Sprintf(tokenFilter, filter), "")
	}

	messages, err := QueryDuckDB(q, append(params, word))
	if err != nil {
		return nil, err
	}

	for messages.Next() {
		var guild_id, author_id, word string
		var word_count int

		err = messages.Scan(&guild_id, &author_id, &word, &word_count)
		if err != nil {
			break
		}

		messageObject := util.CountGrouped{
			Author: author_id,
			Word: util.WordCounted{
				Word:  word,
				Count: word_count,
			},
		}
		messageObjects = append(messageObjects, messageObject)
	}
	return
}

func GetMessageBlock(messageID string) ([]util.MessageObject, error) {
	query := `
		WITH RECURSIVE latest AS (
			SELECT *
			FROM messages
			QUALIFY ROW_NUMBER() OVER (PARTITION BY id ORDER BY version DESC) = 1
		),
		-- walk upward: start from given message, follow reply_message_id to parents
		reply_chain AS (
			SELECT *
			FROM latest
			WHERE id = ?

			UNION ALL

			SELECT parent.*
			FROM latest parent
			JOIN reply_chain child ON child.reply_message_id = parent.id
		),
		-- pick the earliest message in the upward chain as root
		root AS (
			SELECT * FROM reply_chain ORDER BY date LIMIT 1
		),
		-- walk downward from root to get entire subtree of replies
		full_chain AS (
			SELECT r.*
			FROM root r

			UNION ALL

			SELECT child.*
			FROM latest child
			JOIN full_chain parent ON child.reply_message_id = parent.id
		),
		window_bounds AS (
			SELECT MIN(date) AS min_date, MAX(date) AS max_date
			FROM full_chain
		),
		context_messages AS (
			SELECT *
			FROM latest
			WHERE channel_id = (SELECT channel_id FROM latest WHERE id = ?)
			AND date BETWEEN
					(SELECT min_date - INTERVAL 5 MINUTE FROM window_bounds)
				AND (SELECT max_date + INTERVAL 5 MINUTE FROM window_bounds)
		)
		SELECT id, guild_id, channel_id, author_id, reply_message_id,
			content, date, version
		FROM context_messages
		ORDER BY date;
	`
	rows, err := duckdbClient.Query(query, messageID, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []util.MessageObject

	for rows.Next() {
		var m util.MessageObject
		err := rows.Scan(
			&m.MessageID, &m.GuildID, &m.ChannelID, &m.Author, &m.ReplyMessageID,
			&m.Content, &m.Date, &m.Version,
		)
		if err != nil {
			return nil, err
		}

		result = append(result, m)
	}

	return result, nil
}
