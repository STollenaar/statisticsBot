package database

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/stollenaar/statisticsbot/internal/util"

	"github.com/bwmarrin/discordgo"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	_ "github.com/marcboeker/go-duckdb" // DuckDB Go driver
)

const (
	collectionName = "statisticsbot"
)

var (
	milvusClient client.Client
	duckdbClient *sql.DB
)

// Define the request and response structures
type TextRequest struct {
	Text string `json:"text"`
}

type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
	MoodEmbedding []float32 `json:"moodEmbedding"`
}

func init() {
	initMilvus()
	initDuckDB()
}

func Exit() {
	milvusClient.Close()
	duckdbClient.Close()
}

func initMilvus() {
	var err error
	//...other snippet ...
	milvusClient, err = client.NewGrpcClient(context.TODO(), util.ConfigFile.DATABASE_HOST)
	if err != nil {
		// handle error
		log.Fatal(err)
	}

	has, err := milvusClient.HasCollection(context.TODO(), collectionName)
	if err != nil {
		log.Fatal("failed to check whether collection exists:", err.Error())
	}
	if !has {
		// collection with same name exist, clean up mess
		err := milvusClient.CreateCollection(context.TODO(), &entity.Schema{
			CollectionName: collectionName,
			Description:    "Discord messages with embeddings",
			AutoID:         false,
			Fields: []*entity.Field{
				{
					Name:       "id",
					DataType:   entity.FieldTypeVarChar,
					PrimaryKey: true,
					AutoID:     false,
					TypeParams: map[string]string{
						entity.TypeParamMaxLength: "64",
					},
				},
				{
					Name:       "guild_id",
					DataType:   entity.FieldTypeVarChar,
					PrimaryKey: false,
					AutoID:     false,
					TypeParams: map[string]string{
						entity.TypeParamMaxLength: "64",
					},
				},
				{
					Name:       "channel_id",
					DataType:   entity.FieldTypeVarChar,
					PrimaryKey: false,
					AutoID:     false,
					TypeParams: map[string]string{
						entity.TypeParamMaxLength: "64",
					},
				},
				{
					Name:       "author_id",
					DataType:   entity.FieldTypeVarChar,
					PrimaryKey: false,
					AutoID:     false,
					TypeParams: map[string]string{
						entity.TypeParamMaxLength: "64",
					},
				},
				{
					Name:     "embedding",
					DataType: entity.FieldTypeFloatVector,
					TypeParams: map[string]string{
						entity.TypeParamDim: "384",
					},
				},
				{
					Name:     "mood_embedding",
					DataType: entity.FieldTypeFloatVector,
					TypeParams: map[string]string{
						entity.TypeParamDim: "768",
					},
				},
			},
		}, entity.DefaultShardNumber)

		if err != nil {
			log.Fatal("failed to create collection: ", err)
		}

	}

	// Check if an index exists for the specified field
	indexes, err := milvusClient.DescribeIndex(context.Background(), collectionName, "embedding")
	if err != nil {
		fmt.Println("No index'. Creating index...")
		// Now add index
		idx, err := entity.NewIndexIvfFlat(entity.L2, 2)
		if err != nil {
			log.Fatal("fail to create ivf flat index:", err.Error())
		}
		// Create the index
		err = milvusClient.CreateIndex(context.Background(), collectionName, "embedding", idx, false)
		if err != nil {
			log.Fatalf("Failed to create index: %v", err)
		}
	} else {
		fmt.Printf("Index already exists: %v\n", indexes)
	}

	// Check if an index exists for the specified field
	indexes, err = milvusClient.DescribeIndex(context.Background(), collectionName, "mood_embedding")
	if err != nil {
		fmt.Println("No index'. Creating index...")
		// Now add index
		idx, err := entity.NewIndexIvfFlat(entity.L2, 2)
		if err != nil {
			log.Fatal("fail to create ivf flat index:", err.Error())
		}
		// Create the index
		err = milvusClient.CreateIndex(context.Background(), collectionName, "mood_embedding", idx, false)
		if err != nil {
			log.Fatalf("Failed to create index: %v", err)
		}
	} else {
		fmt.Printf("Index already exists: %v\n", indexes)
	}

	err = milvusClient.LoadCollection(context.TODO(), collectionName, false)
	if err != nil {
		// handle error
		log.Fatal(err)
	}
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
		WITH ranked_messages AS (
			SELECT *,
				   ROW_NUMBER() OVER (PARTITION BY channel_id ORDER BY date DESC) AS rank
			FROM messages
			WHERE channel_id = ?
		)
		SELECT 
			id,
			date,
		FROM ranked_messages
		WHERE rank = 1;
	`

	// Execute the query
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
		messageTime := message.Timestamp

		return messageTime.After(lastMessage.Date)
	})

	// Constructing operations for first 100
	for _, message := range messages {
		operations++
		ConstructMessageObject(message, channel.GuildID)
	}

	// Loading more messages if got 100 message the first time
	if len(messages) == 100 {
		lastMessageCollected := messages[len(messages)-1]
		// Loading more messages, 100 at a time
		for lastMessageCollected != nil {
			moreMes, _ := Bot.ChannelMessages(channel.ID, int(100), lastMessageCollected.ID, "", "")
			moreMes = util.FilterDiscordMessages(moreMes, func(message *discordgo.Message) bool {
				messageTime := message.Timestamp

				return messageTime.After(lastMessage.Date)
			})

			for _, message := range moreMes {
				operations++
				ConstructMessageObject(message, channel.GuildID)
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
func ConstructMessageObject(message *discordgo.Message, guildID string) {

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

	embedding, err := getEmbedding(strings.Join(content, "\n"))
	if err != nil {
		log.Fatal(err)
	}

	idC := entity.NewColumnVarChar("id", []string{message.ID})
	guildC := entity.NewColumnVarChar("guild_id", []string{guildID})
	channelC := entity.NewColumnVarChar("channel_id", []string{message.ChannelID})
	authorC := entity.NewColumnVarChar("author_id", []string{message.Author.ID})
	embedC := entity.NewColumnFloatVector("embedding", 384, [][]float32{embedding.Embedding})
	moodEmbedC := entity.NewColumnFloatVector("mood_embedding", 768, [][]float32{embedding.MoodEmbedding})

	_, err = milvusClient.Insert(context.TODO(), collectionName, "", idC, guildC, channelC, authorC, embedC, moodEmbedC)
	if err != nil {
		log.Fatalf("Error inserting into milvus: %s\n", err)
	}
	_, err = duckdbClient.Exec(`INSERT INTO messages VALUES (?,?,?,?,?,?)`, message.ID, message.GuildID, message.ChannelID, message.Author.ID, message.Timestamp, message.Content)
	if err != nil {
		log.Fatalf("Error inserting into duckdb: %s\n", err)
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

func DeleteMilvus(query string) error {

	err := milvusClient.Delete(context.TODO(), collectionName, "", query)
	if err != nil {
		return err
	}
	return nil
}

func QueryMilvus(query string, outputFields []string) (*client.QueryIterator, error) {
	
	rs, err := milvusClient.QueryIterator(context.TODO(), client.NewQueryIteratorOption(collectionName).WithExpr(query).WithOutputFields(outputFields...))
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func StartTX() (*sql.Tx, error) {
	return duckdbClient.Begin()
}

func getEmbedding(in string) (EmbeddingResponse, error) {
	requestBody, _ := json.Marshal(TextRequest{Text: in})

	resp, err := http.Post(fmt.Sprintf("http://%s/embed", util.ConfigFile.SENTENCE_TRANSFORMERS), "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return EmbeddingResponse{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result EmbeddingResponse
	json.Unmarshal(body, &result)

	return result, nil
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