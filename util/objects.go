package util

import "github.com/bwmarrin/discordgo"

// CountGrouped Basic count group for the max command
type CountGrouped struct {
	Author string      `bson:"_id" json:"Author"`
	Word   wordCounted `bson:"Word" json:"Word"`
}

// MessageObject general messageobject for functions
type MessageObject struct {
	GuildID   string              `bson:"GuildID" json:"GuildID"`
	ChannelID string              `bson:"ChannelID" json:"ChannelID"`
	MessageID string              `bson:"_id" json:"MessageID"`
	Author    string              `bson:"Author" json:"Author"`
	Content   []string            `bson:"Content" json:"Content"`
	Date      discordgo.Timestamp `bson:"Date" json:"Date"`
}

type wordCounted struct {
	Word  string `bson:"Word" json:"Word"`
	Count int    `bson:"wordCount" json:"Count"`
}
