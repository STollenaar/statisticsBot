package util

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/disgoorg/disgo/discord"
	"github.com/joho/godotenv"
	"github.com/stollenaar/aws-rotating-credentials-provider/credentials/filecreds"
)

type Config struct {
	DEBUG         bool
	DISCORD_TOKEN string
	DUCKDB_PATH   string
	ADMIN_USER_ID string

	HEALTH_PORT string

	AWS_REGION         string
	AWS_PARAMETER_NAME string
	TERMINAL_REGEX     string

	OLLAMA_URL       string
	OLLAMA_AUTH_TYPE string
	OLLAMA_MODEL     string

	AWS_OLLAMA_AUTH_USERNAME string
	OLLAMA_AUTH_USERNAME     string
	AWS_OLLAMA_AUTH_PASSWORD string
	OLLAMA_AUTH_PASSWORD     string

	OLLAMA_API_KEY     string
	AWS_OLLAMA_API_KEY string

	HTTP_AUTH_TOKEN     string
	AWS_HTTP_AUTH_TOKEN string

	B2_BUCKET   string
	B2_PREFIX   string
	B2_REGION   string
	B2_ENDPOINT string

	B2_KEY_ID              string
	AWS_B2_KEY_ID          string
	B2_APPLICATION_KEY     string
	AWS_B2_APPLICATION_KEY string

	// BACKUP_RECIPIENTS holds one or more age public keys, separated by commas
	// or whitespace, that uploaded backups are encrypted to.
	// AWS_BACKUP_RECIPIENTS instead names the SSM parameter holding them.
	//
	// The matching private key never exists in the cluster, so a compromised
	// pod — or anyone who gets the B2 credentials — can write backups it cannot
	// read. Listing several recipients encrypts to all of them, so a second key
	// can be introduced before the first is retired without re-encrypting
	// anything.
	//
	// Resolved per backup rather than cached at startup, like the Backblaze
	// credentials, so rotating the recipient is a parameter edit and nothing
	// else: no apply, no restart.
	BACKUP_RECIPIENTS     string
	AWS_BACKUP_RECIPIENTS string

	// BACKUP_ENCRYPTION set to "off" uploads plaintext. Backups carry every
	// message in the guild, so the insecure path has to be asked for out loud
	// rather than being what happens when a variable is missing.
	BACKUP_ENCRYPTION string
}

var (
	ConfigFile *Config
	ssmClient  *ssm.Client
)

func init() {
	ConfigFile = new(Config)
	_, err := os.Stat(".env")
	if err == nil {
		err = godotenv.Load(".env")
		if err != nil {
			slog.Error("Error loading environment variables", slog.Any("err", err))
			os.Exit(1)
		}
	}

	ConfigFile = &Config{
		AWS_REGION:               os.Getenv("AWS_REGION"),
		DISCORD_TOKEN:            os.Getenv("DISCORD_TOKEN"),
		DEBUG:                    envBool("DEBUG"),
		AWS_PARAMETER_NAME:       os.Getenv("AWS_PARAMETER_NAME"),
		DUCKDB_PATH:              os.Getenv("DUCKDB_PATH"),
		TERMINAL_REGEX:           os.Getenv("TERMINAL_REGEX"),
		OLLAMA_URL:               os.Getenv("OLLAMA_URL"),
		OLLAMA_MODEL:             os.Getenv("OLLAMA_MODEL"),
		OLLAMA_AUTH_TYPE:         os.Getenv("OLLAMA_AUTH_TYPE"),
		OLLAMA_AUTH_USERNAME:     os.Getenv("OLLAMA_AUTH_USERNAME"),
		OLLAMA_AUTH_PASSWORD:     os.Getenv("OLLAMA_AUTH_PASSWORD"),
		OLLAMA_API_KEY:           os.Getenv("OLLAMA_API_KEY"),
		AWS_OLLAMA_AUTH_USERNAME: os.Getenv("AWS_OLLAMA_AUTH_USERNAME"),
		AWS_OLLAMA_AUTH_PASSWORD: os.Getenv("AWS_OLLAMA_AUTH_PASSWORD"),
		AWS_OLLAMA_API_KEY:       os.Getenv("AWS_OLLAMA_API_KEY"),
		HTTP_AUTH_TOKEN:          os.Getenv("HTTP_AUTH_TOKEN"),
		AWS_HTTP_AUTH_TOKEN:      os.Getenv("AWS_HTTP_AUTH_TOKEN"),
		HEALTH_PORT:              os.Getenv("HEALTH_PORT"),
		ADMIN_USER_ID:            os.Getenv("ADMIN_USER_ID"),
		B2_BUCKET:                os.Getenv("B2_BUCKET"),
		B2_PREFIX:                os.Getenv("B2_PREFIX"),
		B2_REGION:                os.Getenv("B2_REGION"),
		B2_ENDPOINT:              os.Getenv("B2_ENDPOINT"),
		B2_KEY_ID:                os.Getenv("B2_KEY_ID"),
		AWS_B2_KEY_ID:            os.Getenv("AWS_B2_KEY_ID"),
		B2_APPLICATION_KEY:       os.Getenv("B2_APPLICATION_KEY"),
		AWS_B2_APPLICATION_KEY:   os.Getenv("AWS_B2_APPLICATION_KEY"),
		BACKUP_RECIPIENTS:        os.Getenv("BACKUP_RECIPIENTS"),
		AWS_BACKUP_RECIPIENTS:    os.Getenv("AWS_BACKUP_RECIPIENTS"),
		BACKUP_ENCRYPTION:        os.Getenv("BACKUP_ENCRYPTION"),
	}
	if ConfigFile.TERMINAL_REGEX == "" {
		ConfigFile.TERMINAL_REGEX = `(\.|,|:|;|\?|!)$`
	}

	if ConfigFile.OLLAMA_MODEL == "" {
		ConfigFile.OLLAMA_MODEL = "llama3.2:3b"
	}

	if ConfigFile.B2_PREFIX == "" {
		ConfigFile.B2_PREFIX = "statisticsbot"
	}

	if ConfigFile.B2_ENDPOINT == "" && ConfigFile.B2_REGION != "" {
		ConfigFile.B2_ENDPOINT = fmt.Sprintf("https://s3.%s.backblazeb2.com", ConfigFile.B2_REGION)
	}
}

func init() {

	if os.Getenv("AWS_SHARED_CREDENTIALS_FILE") != "" {
		provider := filecreds.NewFilecredentialsProvider(os.Getenv("AWS_SHARED_CREDENTIALS_FILE"))
		ssmClient = ssm.New(ssm.Options{
			Credentials: provider,
			Region:      ConfigFile.AWS_REGION,
		})
	} else {

		// Create a config with the credentials provider.
		cfg, err := config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(ConfigFile.AWS_REGION),
		)

		if err != nil {
			if _, isProfileNotExistError := err.(config.SharedConfigProfileNotExistError); isProfileNotExistError {
				cfg, err = config.LoadDefaultConfig(context.TODO(),
					config.WithRegion(ConfigFile.AWS_REGION),
				)
			}
			if err != nil {
				slog.Error("Error loading AWS config", slog.Any("err", err))
				os.Exit(1)
			}
		}

		ssmClient = ssm.NewFromConfig(cfg)
	}
}

func (c *Config) GetDiscordToken() string {
	if ConfigFile.DISCORD_TOKEN == "" && ConfigFile.AWS_PARAMETER_NAME == "" {
		slog.Error("DISCORD_TOKEN or AWS_PARAMETER_NAME is not set")
		os.Exit(1)
	}

	if ConfigFile.DISCORD_TOKEN != "" {
		return ConfigFile.DISCORD_TOKEN
	}
	out, err := getAWSParameter(ConfigFile.AWS_PARAMETER_NAME)
	if err != nil {
		slog.Error("Error fetching Discord token parameter", slog.Any("err", err))
		os.Exit(1)
	}
	return out
}

func GetOllamaUsername() (string, error) {
	if ConfigFile.OLLAMA_AUTH_USERNAME == "" && ConfigFile.AWS_OLLAMA_AUTH_USERNAME == "" {
		slog.Error("OLLAMA_AUTH_USERNAME or AWS_OLLAMA_AUTH_USERNAME is not set")
		os.Exit(1)
	}

	if ConfigFile.OLLAMA_AUTH_USERNAME != "" {
		return ConfigFile.OLLAMA_AUTH_USERNAME, nil
	}
	return getAWSParameter(ConfigFile.AWS_OLLAMA_AUTH_USERNAME)
}

func GetOllamaPassword() (string, error) {
	if ConfigFile.OLLAMA_AUTH_PASSWORD == "" && ConfigFile.AWS_OLLAMA_AUTH_PASSWORD == "" {
		slog.Error("OLLAMA_AUTH_PASSWORD or AWS_OLLAMA_AUTH_PASSWORD is not set")
		os.Exit(1)
	}

	if ConfigFile.OLLAMA_AUTH_PASSWORD != "" {
		return ConfigFile.OLLAMA_AUTH_PASSWORD, nil
	}

	return getAWSParameter(ConfigFile.AWS_OLLAMA_AUTH_PASSWORD)
}

func GetOllamaAPIKey() (string, error) {
	if ConfigFile.OLLAMA_API_KEY == "" && ConfigFile.AWS_OLLAMA_API_KEY == "" {
		slog.Error("OLLAMA_API_KEY or OLLAMA_API_KEY is not set")
		os.Exit(1)
	}

	if ConfigFile.OLLAMA_API_KEY != "" {
		return ConfigFile.OLLAMA_API_KEY, nil
	}

	return getAWSParameter(ConfigFile.AWS_OLLAMA_API_KEY)
}

// envBool reads a boolean environment variable. Anything unparseable counts as
// false, so a typo disables a feature rather than enabling it by accident.
func envBool(name string) bool {
	value, err := strconv.ParseBool(os.Getenv(name))
	return err == nil && value
}

// GetHTTPAuthToken returns the shared token the internal HTTP API requires.
//
// The three outcomes are distinct and the caller must treat them differently:
//   - ("", nil)    no token configured at all
//   - (token, nil) a token is configured and resolved
//   - ("", err)    a token was configured but could not be read
//
// The last case must not be mistaken for the first: a token was intended, so
// the API has to refuse requests rather than quietly serve them unauthenticated.
// Like the B2 getters this reports an error instead of exiting — a misconfigured
// admin API must not take the Discord side of the bot down with it.
func GetHTTPAuthToken() (string, error) {
	if ConfigFile.HTTP_AUTH_TOKEN != "" {
		return ConfigFile.HTTP_AUTH_TOKEN, nil
	}
	if ConfigFile.AWS_HTTP_AUTH_TOKEN == "" {
		return "", nil
	}
	return getAWSParameter(ConfigFile.AWS_HTTP_AUTH_TOKEN)
}

// GetB2KeyID returns the Backblaze application key ID. Unlike the Ollama
// getters this reports an error instead of exiting: backups are triggered by a
// CronJob against the running bot, and misconfigured backup credentials must
// not take the bot down with them.
func GetB2KeyID() (string, error) {
	if ConfigFile.B2_KEY_ID != "" {
		return ConfigFile.B2_KEY_ID, nil
	}
	if ConfigFile.AWS_B2_KEY_ID == "" {
		return "", fmt.Errorf("neither B2_KEY_ID nor AWS_B2_KEY_ID is set")
	}
	return getAWSParameter(ConfigFile.AWS_B2_KEY_ID)
}

// GetBackupRecipients returns the age public keys backups are encrypted to.
//
// Returns an empty string when neither variable is set, rather than an error:
// the backup package decides what an absent recipient means, and it has the
// context to say so usefully. A configured parameter that cannot be read *is*
// an error, so a transient SSM failure fails the backup instead of looking like
// no recipient was ever configured.
func GetBackupRecipients() (string, error) {
	if ConfigFile.BACKUP_RECIPIENTS != "" {
		return ConfigFile.BACKUP_RECIPIENTS, nil
	}
	if ConfigFile.AWS_BACKUP_RECIPIENTS == "" {
		return "", nil
	}
	return getAWSParameter(ConfigFile.AWS_BACKUP_RECIPIENTS)
}

// GetB2ApplicationKey returns the Backblaze application key.
func GetB2ApplicationKey() (string, error) {
	if ConfigFile.B2_APPLICATION_KEY != "" {
		return ConfigFile.B2_APPLICATION_KEY, nil
	}
	if ConfigFile.AWS_B2_APPLICATION_KEY == "" {
		return "", fmt.Errorf("neither B2_APPLICATION_KEY nor AWS_B2_APPLICATION_KEY is set")
	}
	return getAWSParameter(ConfigFile.AWS_B2_APPLICATION_KEY)
}

func getAWSParameter(parameterName string) (string, error) {
	out, err := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name:           aws.String(parameterName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		slog.Error("error fetching parameter", slog.String("parameter", parameterName), slog.Any("err", err))
		return "", err
	}
	return *out.Parameter.Value, err
}

func (c *Config) SetEphemeral() discord.MessageFlags {
	if c.DEBUG {
		return discord.MessageFlagEphemeral
	} else {
		return 0
	}
}
