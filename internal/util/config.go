package util

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/stollenaar/aws-rotating-credentials-provider/credentials/filecreds"
)

type Config struct {
	DEBUG         bool
	DISCORD_TOKEN string
	DUCKDB_PATH   string

	AWS_REGION         string
	AWS_PARAMETER_NAME string
	TERMINAL_REGEX     string

	SQS_REQUEST  string
	SQS_RESPONSE string

	OLLAMA_URL       string
	OLLAMA_AUTH_TYPE string

	AWS_OLLAMA_AUTH_USERNAME string
	OLLAMA_AUTH_USERNAME     string
	AWS_OLLAMA_AUTH_PASSWORD string
	OLLAMA_AUTH_PASSWORD     string
}

var (
	ConfigFile *Config
	ssmClient  *ssm.Client
)

func init() {
	ConfigFile = &Config{
		AWS_REGION: os.Getenv("AWS_REGION"),
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
				log.Fatal("Error loading AWS config:", err)
			}
		}

		ssmClient = ssm.NewFromConfig(cfg)
	}
}

func init() {
	ConfigFile = new(Config)
	_, err := os.Stat(".env")
	if err == nil {
		err = godotenv.Load(".env")
		if err != nil {
			log.Fatal("Error loading environment variables")
		}
	}

	ConfigFile = &Config{
		DISCORD_TOKEN:            os.Getenv("DISCORD_TOKEN"),
		AWS_PARAMETER_NAME:       os.Getenv("AWS_PARAMETER_NAME"),
		SQS_REQUEST:              os.Getenv("SQS_REQUEST"),
		DUCKDB_PATH:              os.Getenv("DUCKDB_PATH"),
		SQS_RESPONSE:             os.Getenv("SQS_RESPONSE"),
		TERMINAL_REGEX:           os.Getenv("TERMINAL_REGEX"),
		OLLAMA_URL:               os.Getenv("OLLAMA_URL"),
		OLLAMA_AUTH_TYPE:         os.Getenv("OLLAMA_AUTH_TYPE"),
		OLLAMA_AUTH_USERNAME:     os.Getenv("OLLAMA_AUTH_USERNAME"),
		OLLAMA_AUTH_PASSWORD:     os.Getenv("OLLAMA_AUTH_PASSWORD"),
		AWS_OLLAMA_AUTH_USERNAME: os.Getenv("AWS_OLLAMA_AUTH_USERNAME"),
		AWS_OLLAMA_AUTH_PASSWORD: os.Getenv("AWS_OLLAMA_AUTH_PASSWORD"),
	}
	if ConfigFile.TERMINAL_REGEX == "" {
		ConfigFile.TERMINAL_REGEX = `(\.|,|:|;|\?|!)$`
	}

}

func GetDiscordToken() string {
	if ConfigFile.DISCORD_TOKEN == "" && ConfigFile.AWS_PARAMETER_NAME == "" {
		log.Fatal("DISCORD_TOKEN or AWS_PARAMETER_NAME is not set")
	}

	if ConfigFile.DISCORD_TOKEN != "" {
		return ConfigFile.DISCORD_TOKEN
	} else {
		out, err := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
			Name:           &ConfigFile.AWS_PARAMETER_NAME,
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			log.Fatal(err)
		}
		return *out.Parameter.Value
	}
}

func GetOllamaUsername() (string, error) {
	if ConfigFile.OLLAMA_AUTH_USERNAME == "" && ConfigFile.AWS_OLLAMA_AUTH_USERNAME == "" {
		log.Fatal("OLLAMA_AUTH_USERNAME or AWS_OLLAMA_AUTH_USERNAME is not set")
	}

	if ConfigFile.OLLAMA_AUTH_USERNAME != "" {
		return ConfigFile.OLLAMA_AUTH_USERNAME, nil
	} else {
		out, err := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
			Name:           &ConfigFile.AWS_OLLAMA_AUTH_USERNAME,
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			return "", err
		}
		return *out.Parameter.Value, nil
	}
}

func GetOllamaPassword() (string, error) {
	if ConfigFile.OLLAMA_AUTH_PASSWORD == "" && ConfigFile.AWS_OLLAMA_AUTH_PASSWORD == "" {
		log.Fatal("OLLAMA_AUTH_PASSWORD or AWS_OLLAMA_AUTH_PASSWORD is not set")
	}

	if ConfigFile.OLLAMA_AUTH_PASSWORD != "" {
		return ConfigFile.OLLAMA_AUTH_PASSWORD, nil
	} else {
		out, err := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
			Name:           &ConfigFile.AWS_OLLAMA_AUTH_PASSWORD,
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			return "", err
		}
		return *out.Parameter.Value, nil
	}
}

func getAWSParameter(parameterName string) (string, error) {
	out, err := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name:           aws.String(parameterName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		fmt.Println(fmt.Errorf("error from fetching parameter %s. With error: %w", parameterName, err))
		return "", err
	}
	return *out.Parameter.Value, err
}

func (c *Config) SetEphemeral() discordgo.MessageFlags {
	if c.DEBUG {
		return discordgo.MessageFlagsEphemeral
	} else {
		return 0
	}
}
