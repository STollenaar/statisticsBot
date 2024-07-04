package util

import "time"

// CountGrouped Basic count group for the max command
type CountGrouped struct {
	Author string      `bson:"_id" json:"Author"`
	Word   wordCounted `bson:"Word" json:"Word"`
}

// MessageObject general messageobject for functions
type MessageObject struct {
	GuildID   string    `bson:"GuildID" json:"GuildID"`
	ChannelID string    `bson:"ChannelID" json:"ChannelID"`
	MessageID string    `bson:"_id" json:"MessageID"`
	Author    string    `bson:"Author" json:"Author"`
	Content   []string  `bson:"Content" json:"Content"`
	Date      time.Time `bson:"Date" json:"Date"`
}

type wordCounted struct {
	Word  string `bson:"Word" json:"Word"`
	Count int    `bson:"wordCount" json:"Count"`
}

type SQSObject struct {
	Type          string `json:"type"`
	Command       string `json:"command"`
	Data          string `json:"data"`
	ChannelID     string `json:"channelID"`
	GuildID       string `json:"guildID"`
	Token         string `json:"token"`
	ApplicationID string `json:"applicationID"`
}
