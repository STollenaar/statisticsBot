package util

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Config struct {
	DISCORD_TOKEN string
	DATABASE_HOST string

	AWS_REGION         string
	AWS_PARAMETER_NAME string

	MONGO_USERNAME_PARAMETER string
	MONGO_PASSWORD_PARAMETER string
}

var ConfigFile *Config

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
		AWS_REGION:               os.Getenv("AWS_REGION"),
		AWS_PARAMETER_NAME:       os.Getenv("AWS_PARAMETER_NAME"),
		MONGO_USERNAME_PARAMETER: os.Getenv("MONGO_USERNAME_PARAMETER"),
		MONGO_PASSWORD_PARAMETER: os.Getenv("MONGO_PASSWORD_PARAMETER"),
	}

	if ConfigFile.DISCORD_TOKEN == "" && ConfigFile.AWS_PARAMETER_NAME == "" {
		log.Fatal("DISCORD_TOKEN or AWS_PARAMETER_NAME is not set")
	}
}

func CreateMongoAuth() options.Credential {
	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		log.Fatal(err)
	}
	cfg.Region = ConfigFile.AWS_REGION

	ssmClient := ssm.NewFromConfig(cfg)
	mongoUsername, _ := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name:           &ConfigFile.MONGO_USERNAME_PARAMETER,
		WithDecryption: true,
	})
	mongoPassword, _ := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name:           &ConfigFile.MONGO_PASSWORD_PARAMETER,
		WithDecryption: true,
	})
	return options.Credential{
		Username: *mongoUsername.Parameter.Value,
		Password: *mongoPassword.Parameter.Value,
	}
}

func GetDiscordToken() string {
	if ConfigFile.DISCORD_TOKEN != "" {
		return ConfigFile.DISCORD_TOKEN
	} else {
		cfg, err := config.LoadDefaultConfig(context.TODO())

		if err != nil {
			log.Fatal(err)
		}
		cfg.Region = ConfigFile.AWS_REGION

		ssmClient := ssm.NewFromConfig(cfg)
		out, err := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
			Name:           &ConfigFile.AWS_PARAMETER_NAME,
			WithDecryption: true,
		})
		if err != nil {
			log.Fatal(err)
		}
		return *out.Parameter.Value
	}
}
