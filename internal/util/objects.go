package util

import "time"

// CountGrouped Basic count group for the max command
type CountGrouped struct {
	Author string      `json:"Author"`
	Word   WordCounted `json:"Word"`
}

// MessageObject general messageobject for functions
type MessageObject struct {
	GuildID   string    `milvus:"name:guild_id" json:"guild_id"`
	ChannelID string    `milvus:"name:channel_id" json:"channel_id"`
	MessageID string    `milvus:"name:id" json:"id"`
	Author    string    `milvus:"name:author_id" json:"author_id"`
	Content   string    `milvus:"name:content" json:"content"`
	Date      time.Time `milvus:"name:date" json:"date"`
}

type WordCounted struct {
	Word  string `json:"Word"`
	Count int    `json:"Count"`
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

