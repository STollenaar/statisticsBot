package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/stollenaar/statisticsbot/internal/util"

	"github.com/bwmarrin/discordgo"

	_ "github.com/marcboeker/go-duckdb" // DuckDB Go driver
)

var (
	duckdbClient *sql.DB

	CustomEmojiCache = make(map[string]string)
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

	// Create the messages table
	_, err = duckdbClient.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id VARCHAR,
			guild_id VARCHAR,
			channel_id VARCHAR,
			author_id VARCHAR,
			reply_message_id VARCHAR,
			content VARCHAR,
			date TIMESTAMP,
			version INTEGER DEFAULT 1,
    		PRIMARY KEY (id, version)
		);
	`)
	if err != nil {
		log.Fatalf("Failed to create message table: %v", err)
	}

	// Create the reactions table
	_, err = duckdbClient.Exec(`
		CREATE TABLE IF NOT EXISTS reactions (
			id VARCHAR,
			guild_id VARCHAR,
			channel_id VARCHAR,
			author_id VARCHAR,
			reaction VARCHAR,
			date TIMESTAMP,
			PRIMARY KEY (id, reaction, author_id)
		);
		CREATE INDEX IF NOT EXISTS idx_message_reactions ON reactions (id);
	`)
	if err != nil {
		log.Fatalf("Failed to create reactions table: %v", err)
	}

	// Create guild emoji cache
	_, err = duckdbClient.Exec(`
		CREATE TABLE IF NOT EXISTS emojis (
			id VARCHAR,
			name VARCHAR,
			guild_id VARCHAR,
			image_data VARCHAR,
			PRIMARY KEY (guild_id, id)
		);
	`)
	if err != nil {
		log.Fatalf("Failed to create guild emoji table: %v", err)
	}

	// Create bot message cache
	_, err = duckdbClient.Exec(`
		CREATE TABLE IF NOT EXISTS bot_messages (
			id VARCHAR,
			guild_id VARCHAR,
			channel_id VARCHAR,
			author_id VARCHAR,
			reply_message_id VARCHAR,
			content VARCHAR,
			date TIMESTAMP,
			version INTEGER DEFAULT 1,
    		PRIMARY KEY (id, version)
		);
	`)
	if err != nil {
		log.Fatalf("Failed to create bot messages table: %v", err)
	}
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
          FROM messages
          WHERE channel_id = ?
          ORDER BY date DESC
          LIMIT 1;
	`

	// Execute the query'
	row := duckdbClient.QueryRow(query, channel.ID)

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
	timestamp, err := util.SnowflakeToTimestamp(message.ID)
	if err != nil {
		fmt.Printf("Error converting snowflake to timestamp: %s\n", err)
	}
	table := "messages"
	if isBot {
		table = "bot_messages"
	}

	var referencedMessage string
	columns := "id, guild_id, channel_id, author_id, content, date, version"
	values := "?, ?, ?, ?, ?, ?, 1"
	args := []any{message.ID, guildID, message.ChannelID, message.Author.ID, strings.Join(content, "\n"), timestamp}
	if message.MessageReference != nil {
		referencedMessage = message.MessageReference.MessageID
		columns = "id, guild_id, channel_id, author_id, reply_message_id, content, date, version"
		values = "?, ?, ?, ?, ?, ?, ?, 1"
		args = []any{message.ID, guildID, message.ChannelID, message.Author.ID, referencedMessage, strings.Join(content, "\n"), timestamp}
	}

	// Increment the version and insert the updated message
	_, err = duckdbClient.Exec(fmt.Sprintf(`INSERT INTO %s (%s) 
                                VALUES (%s)`, table, columns, values), args...)
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

	var referencedMessage string
	columns := "id, guild_id, channel_id, author_id, content, date, version"
	values := "?, ?, ?, ?, ?, ?, ?"
	args := []any{message.ID, guildID, message.ChannelID, message.Author.ID, contentStr, timestamp, maxVersion}
	if message.MessageReference != nil {
		referencedMessage = message.MessageReference.MessageID
		columns = "id, guild_id, channel_id, author_id, reply_message_id, content, date, version"
		values = "?, ?, ?, ?, ?, ?, ?, ?"
		args = []any{message.ID, guildID, message.ChannelID, message.Author.ID, referencedMessage, contentStr, timestamp, maxVersion}
	}

	// Increment the version and insert the updated message
	_, err = duckdbClient.Exec(fmt.Sprintf(`INSERT INTO %s (%s) 
                                VALUES (%s)`, table, columns, values), args...)

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
