package util

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Contains check slice contains want string
func Contains(slice []string, want string) bool {
	for _, element := range slice {
		if element == want {
			return true
		}
	}
	return false
}

// DeleteEmpty deleting empty strings in string slice
func DeleteEmpty(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

// Elapsed timing time till function completion
func Elapsed(channel string) func() {
	start := time.Now()
	return func() {
		fmt.Printf("Loading %s took %v to complete\n", channel, time.Since(start))
	}
}

// FilterDiscordMessages filtering specific messages out of message slice
func FilterDiscordMessages(messages []*discordgo.Message, condition func(*discordgo.Message) bool) (result []*discordgo.Message) {
	for _, message := range messages {
		if condition(message) {
			result = append(result, message)
		}
	}
	return result
}

// FilterMessageObjects filtering specific messages out of message slice
func FilterMessageObjects(messages []*MessageObject, condition func(*MessageObject) bool) (result []*MessageObject) {
	for _, message := range messages {
		if condition(message) {
			result = append(result, message)
		}
	}
	return result
}

// FindMaxIndexElement finds the max count element index of the wordcounted slice
func FindMaxIndexElement(slice []CountGrouped) int {
	max := 0

	for index, element := range slice {
		if element.Word.Count > slice[max].Word.Count {
			max = index
		}
	}
	return max
}
