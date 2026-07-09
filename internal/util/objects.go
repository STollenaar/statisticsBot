package util

import (
	"database/sql"
	"time"
)

// CountGrouped Basic count group for the max command
type CountGrouped struct {
	Author string      `json:"Author"`
	Word   WordCounted `json:"Word"`
}

// MessageObject general messageobject for functions
type MessageObject struct {
	GuildID        string         `milvus:"name:guild_id" json:"guild_id"`
	ChannelID      string         `milvus:"name:channel_id" json:"channel_id"`
	MessageID      string         `milvus:"name:id" json:"id"`
	Author         string         `milvus:"name:author_id" json:"author_id"`
	Content        string         `milvus:"name:content" json:"content"`
	ReplyMessageID sql.NullString `json:"reply_message_id"`
	Date           time.Time      `milvus:"name:date" json:"date"`
	Version        int            `json:"version"`
}

type SummaryBody struct {
	Author  string `json:"author"`
	Message string `json:"message"`
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

type OllamaGenerateResponseChoiceMessage struct {
	Content string `json:"content"`
}

type OllamaGenerateResponseChoice struct {
	Message OllamaGenerateResponseChoiceMessage `json:"message"`
}

type OllamaGenerateResponse struct {
	Model              string                         `json:"model"`
	Created            time.Time                      `json:"created_at"`
	Choices            []OllamaGenerateResponseChoice `json:"choices"`
	Done               bool                           `json:"done"`
	Context            []int                          `json:"context"`
	TotalDuration      int                            `json:"total_duration"`
	LoadDuration       int                            `json:"load_duration"`
	PromptEvalCount    int                            `json:"prompt_eval_count"`
	PromptEvalDuration int                            `json:"prompt_eval_duration"`
	EvalCount          int                            `json:"eval_count"`
	EvalDuration       int                            `json:"eval_duration"`
}

type OllamaGenerateRequest struct {
	Model            string              `json:"model"`
	Messages         []map[string]string `json:"messages"`
	Temperature      float32             `json:"temperature"`
	MaxTokens        int                 `json:"max_tokens"`
	FrequencePenalty float32             `json:"frequency_penalty"`
	PresencePenalty  float32             `json:"presence_penalty"`
	ResponseFormat   any                 `json:"response_format"`
	Stream           bool                `json:"stream"`
}

// payload := map[string]any{
// 	"model":             model,
// 	"messages":          []map[string]string{{"role": "user", "content": prompt}},
// 	"temperature":       0.2,
// 	"max_tokens":        llmMaxTokens(),
// 	"stream":            false,
// 	"frequency_penalty": freqPenalty,
// 	"presence_penalty":  presPenalty,
// 	"response_format":   responseFormat(n),
// }
