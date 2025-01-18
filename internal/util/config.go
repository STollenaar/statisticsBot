package util

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/joho/godotenv"
	"github.com/stollenaar/aws-rotating-credentials-provider/credentials/filecreds"
)

type Config struct {
	DISCORD_TOKEN         string
	DATABASE_HOST         string
	DUCKDB_PATH           string
	SENTENCE_TRANSFORMERS string

	AWS_REGION         string
	AWS_PARAMETER_NAME string
	TERMINAL_REGEX     string

	SQS_REQUEST  string
	SQS_RESPONSE string

	EPS         float32
	MIN_SAMPLES int
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
		DISCORD_TOKEN:         os.Getenv("DISCORD_TOKEN"),
		DATABASE_HOST:         os.Getenv("DATABASE_HOST"),
		AWS_PARAMETER_NAME:    os.Getenv("AWS_PARAMETER_NAME"),
		SQS_REQUEST:           os.Getenv("SQS_REQUEST"),
		DUCKDB_PATH:           os.Getenv("DUCKDB_PATH"),
		SQS_RESPONSE:          os.Getenv("SQS_RESPONSE"),
		TERMINAL_REGEX:        os.Getenv("TERMINAL_REGEX"),
		SENTENCE_TRANSFORMERS: os.Getenv("SENTENCE_TRANSFORMERS"),
		EPS: 0.3,
		MIN_SAMPLES: 6,
	}
	if ConfigFile.TERMINAL_REGEX == "" {
		ConfigFile.TERMINAL_REGEX = `(\.|,|:|;|\?|!)$`
	}

	if t := os.Getenv("SUMMARIZE_EPS"); t != "" {
		temp, _ := strconv.ParseFloat(t, 32)
		ConfigFile.EPS = float32(temp)
	} 

	if t := os.Getenv("SUMMARIZE_CLUSTER_MIN_SAMPLES"); t != "" {
		temp, _ := strconv.Atoi(t)
		ConfigFile.MIN_SAMPLES = temp
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
