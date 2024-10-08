package util

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/joho/godotenv"
	"github.com/stollenaar/aws-rotating-credentials-provider/credentials/filecreds"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Config struct {
	DISCORD_TOKEN string
	DATABASE_HOST string

	AWS_REGION         string
	AWS_PARAMETER_NAME string
	TERMINAL_REGEX     string

	MONGO_HOST_PARAMETER     string
	MONGO_USERNAME_PARAMETER string
	MONGO_PASSWORD_PARAMETER string

	SQS_REQUEST  string
	SQS_RESPONSE string

	DUNCE_CHANNEL     string
	AWS_DUNCE_CHANNEL string
	DUNCE_ROLE        string
	AWS_DUNCE_ROLE    string
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
		DATABASE_HOST:            os.Getenv("DATABASE_HOST"),
		AWS_PARAMETER_NAME:       os.Getenv("AWS_PARAMETER_NAME"),
		MONGO_HOST_PARAMETER:     os.Getenv("MONGO_HOST_PARAMETER"),
		MONGO_USERNAME_PARAMETER: os.Getenv("MONGO_USERNAME_PARAMETER"),
		MONGO_PASSWORD_PARAMETER: os.Getenv("MONGO_PASSWORD_PARAMETER"),
		SQS_REQUEST:              os.Getenv("SQS_REQUEST"),
		SQS_RESPONSE:             os.Getenv("SQS_RESPONSE"),
		TERMINAL_REGEX:           os.Getenv("TERMINAL_REGEX"),
		DUNCE_CHANNEL:            os.Getenv("DUNCE_CHANNEL"),
		AWS_DUNCE_ROLE:           os.Getenv("AWS_DUNCE_ROLE"),
		DUNCE_ROLE:               os.Getenv("DUNCE_ROLE"),
		AWS_DUNCE_CHANNEL:        os.Getenv("AWS_DUNCE_CHANNEL"),
	}
	if ConfigFile.TERMINAL_REGEX == "" {
		ConfigFile.TERMINAL_REGEX = `(\.|,|:|;|\?|!)$`
	}
}

func GetMongoHost() string {
	if ConfigFile.DATABASE_HOST == "" && ConfigFile.MONGO_HOST_PARAMETER == "" {
		log.Fatal("DATABASE_HOST or MONGO_HOST_PARAMETER is not set")
	}

	if ConfigFile.DATABASE_HOST != "" {
		return ConfigFile.DATABASE_HOST
	} else {
		out, err := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
			Name:           &ConfigFile.MONGO_HOST_PARAMETER,
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			log.Fatal(err)
		}
		return *out.Parameter.Value
	}
}

func CreateMongoAuth() options.Credential {
	if ConfigFile.MONGO_PASSWORD_PARAMETER == "" || ConfigFile.MONGO_USERNAME_PARAMETER == "" {
		log.Fatal("Mongo authentication parameters are not set")
	}

	mongoUsername, _ := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name:           &ConfigFile.MONGO_USERNAME_PARAMETER,
		WithDecryption: aws.Bool(true),
	})
	mongoPassword, _ := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name:           &ConfigFile.MONGO_PASSWORD_PARAMETER,
		WithDecryption: aws.Bool(true),
	})
	return options.Credential{
		Username: *mongoUsername.Parameter.Value,
		Password: *mongoPassword.Parameter.Value,
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

func (c *Config) GetDunceRole() (string, error) {
	if ConfigFile.DUNCE_ROLE == "" && ConfigFile.AWS_DUNCE_ROLE == "" {
		return "", errors.New("DISCORD_TOKEN or AWS_PARAMETER_NAME is not set")
	}

	if ConfigFile.DUNCE_ROLE != "" {
		return ConfigFile.DUNCE_ROLE, nil
	} else {
		return getAWSParameter(ConfigFile.AWS_DUNCE_ROLE)
	}
}

func (c *Config) GetDunceChannel() (string, error) {
	if ConfigFile.DUNCE_CHANNEL == "" && ConfigFile.AWS_DUNCE_CHANNEL == "" {
		return "", errors.New("DISCORD_TOKEN or AWS_PARAMETER_NAME is not set")
	}

	if ConfigFile.DUNCE_CHANNEL != "" {
		return ConfigFile.DUNCE_CHANNEL, nil
	} else {
		return getAWSParameter(ConfigFile.AWS_DUNCE_CHANNEL)
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
