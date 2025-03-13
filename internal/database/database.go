package database

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/stollenaar/statisticsbot/internal/util"

	"github.com/bwmarrin/discordgo"

	_ "github.com/marcboeker/go-duckdb" // DuckDB Go driver
)

const (
	collectionName = "statisticsbot"
)

var (
	duckdbClient *sql.DB
)

// Define the request and response structures
type TextRequest struct {
	Text string `json:"text"`
}

func init() {
	initDuckDB()
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
			content VARCHAR,
			date TIMESTAMP,
			version INTEGER DEFAULT 1,
    		PRIMARY KEY (id, version)
		);
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
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
	messages, _ := Bot.ChannelMessages(channel.ID, int(100), "", lastMessage.MessageID, "")
	// Sort messages by their Timestamp field in ascending order (oldest to newest)
	sort.Slice(messages, func(i, j int) bool {
		// Compare the timestamps of the messages
		return messages[i].Timestamp.Before((*messages[j]).Timestamp)
	})
	// messages = util.FilterDiscordMessages(messages, func(message *discordgo.Message) bool {
	// 	messageTime := message.Timestamp

	// 	return messageTime.After(lastMessage.Date)
	// })

	// Constructing operations for first 100
	for _, message := range messages {
		operations++
		ConstructCreateMessageObject(message, channel.GuildID)
	}

	// Loading more messages if got 100 message the first time
	if len(messages) == 100 {
		lastMessageCollected := messages[len(messages)-1]
		// Loading more messages, 100 at a time
		for lastMessageCollected != nil {
			moreMes, _ := Bot.ChannelMessages(channel.ID, int(100), "", lastMessageCollected.ID, "")
			sort.Slice(moreMes, func(i, j int) bool {
				// Compare the timestamps of the messages
				return moreMes[i].Timestamp.Before((*moreMes[j]).Timestamp)
			})
		
			// moreMes = util.FilterDiscordMessages(moreMes, func(message *discordgo.Message) bool {
			// 	messageTime := message.Timestamp

			// 	return messageTime.After(lastMessage.Date)
			// })

			for _, message := range moreMes {
				operations++
				ConstructCreateMessageObject(message, channel.GuildID)
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
func ConstructCreateMessageObject(message *discordgo.Message, guildID string) {

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

	_, err = duckdbClient.Exec(`INSERT INTO messages VALUES (?,?,?,?,?,?,1)`, message.ID, message.GuildID, message.ChannelID, message.Author.ID, strings.Join(content, "\n"), timestamp)
	if err != nil {
		fmt.Printf("Error inserting into duckdb: %s\n", err)
	}
}

func constructUpdateMessageObject(message *discordgo.Message, guildID string) {
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
	contentStr := strings.Join(content, " ")

	// Increment the version and insert the updated message
	_, err := duckdbClient.Exec(`INSERT INTO messages (id, guild_id, channel_id, author_id, content, date, version) 
                                SELECT ?, ?, ?, ?, ?, ?, MAX(version) + 1 FROM messages WHERE id = ? AND guild_id = ?`,
		message.ID, guildID, message.ChannelID, message.Author.ID, contentStr, message.Timestamp, message.ID, guildID)

	if err != nil {
		fmt.Printf("Error inserting updated message into DuckDB: %s\n", err)
	}
}

// Get a result from the database using a filter
func QueryDuckDB(query string, params []interface{}) (results *sql.Rows, err error) {

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
		WITH tokenized_messages AS (
			SELECT 
				author_id,
				guild_id,
				LOWER(unnest(string_split(regexp_replace(content, '[^a-zA-Z0-9'' ]', '', 'g'), ' '))) AS word
			FROM messages
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
