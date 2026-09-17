package util

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

const (
	DISCORD_EMOJI_URL       = "https://cdn.discordapp.com/emojis/%s.%s"
	DiscordEpoch      int64 = 1420070400000
)

// The mention alternative comes first so existing mentions are matched whole
// and skipped, rather than having their inner snowflake rewrapped.
var userIDPattern = regexp.MustCompile(`<@!?\d{17,20}>|\b\d{17,20}\b`)

// MentionifyIDs wraps bare Discord user IDs in a mention so they render as the
// user instead of a raw snowflake. Already formatted mentions are left as-is.
func MentionifyIDs(s string) string {
	return userIDPattern.ReplaceAllStringFunc(s, func(match string) string {
		if match[0] == '<' {
			return match
		}
		return fmt.Sprintf("<@%s>", match)
	})
}

// Elapsed timing time till function completion
func Elapsed(channel string) func() {
	start := time.Now()
	return func() {
		slog.Info("Loading channel complete", slog.String("channel", channel), slog.Duration("took", time.Since(start)))
	}
}

// SnowflakeToTimestamp converts a Discord snowflake ID to a timestamp
func SnowflakeToTimestamp(snowflakeID string) (time.Time, error) {
	id, err := strconv.ParseInt(snowflakeID, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	timestamp := (id >> 22) + DiscordEpoch
	return time.Unix(0, timestamp*int64(time.Millisecond)), nil
}

// FetchDiscordEmojiImage fetches the raw image bytes for a given emoji ID and animation status.
func FetchDiscordEmojiImage(emojiID string, isAnimated bool) (string, error) {
	ext := "png"
	if isAnimated {
		ext = "gif"
	}
	url := fmt.Sprintf(DISCORD_EMOJI_URL, emojiID, ext)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch emoji from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %w", err)
	}
	base64Data := base64.StdEncoding.EncodeToString(data)

	return base64Data, nil
}

func GetSeparator() discord.SeparatorComponent {
	divider := true
	spacing := discord.SeparatorSpacingSizeLarge

	return discord.SeparatorComponent{
		Divider: &divider,
		Spacing: spacing,
	}
}

func Pointer[T any](d T) *T {
	return &d
}

func UpdateInteractionResponse(event *events.ApplicationCommandInteractionCreate, components []discord.LayoutComponent) {
	_, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Flags:      Pointer(discord.MessageFlagIsComponentsV2),
		Components: &components,
	})
	if err != nil {
		slog.Error("Error updating interaction response", slog.Any("err", err))
	}
}

func UpdateComponentInteractionResponse(event *events.ComponentInteractionCreate, components []discord.LayoutComponent) {
	_, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Flags:      Pointer(discord.MessageFlagIsComponentsV2),
		Components: &components,
	})
	if err != nil {
		slog.Error("Error updating component interaction response", slog.Any("err", err))
	}
}

func ParseTimeArg(timeUnit string) (time.Duration, error) {
	// Regular expression to match a number followed by a unit
	re := regexp.MustCompile(`^(\d+)([smhd])$`)
	matches := re.FindStringSubmatch(timeUnit)
	if matches == nil {
		return 0, fmt.Errorf("invalid time format: %s", timeUnit)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid number: %v", err)
	}

	unit := matches[2]
	var duration time.Duration

	// Calculate duration based on the unit
	switch unit {
	case "s": // seconds
		duration = time.Duration(value) * time.Second
	case "m": // minutes
		duration = time.Duration(value) * time.Minute
	case "h": // hours
		duration = time.Duration(value) * time.Hour
	case "d": // days
		duration = time.Duration(value) * 24 * time.Hour
	default:
		return 0, fmt.Errorf("unknown time unit: %s", unit)
	}

	// Enforce maximum time limit (1 day)
	maxDuration := 24 * time.Hour
	if duration > maxDuration {
		return 0, fmt.Errorf("time cannot exceed 1 day (24h)")
	}

	return duration, nil
}
