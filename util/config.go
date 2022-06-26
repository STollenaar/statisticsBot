package util

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/joho/godotenv"
)

type Config struct {
	DISCORD_TOKEN string
	DATABASE_HOST string

	AWS_REGION         string
	AWS_PARAMETER_NAME string
}

var ConfigFile *Config

func init() {
	ConfigFile = new(Config)
	if _, err := os.Stat("config.json"); err != nil {
		_, err := os.Stat(".env")
		if err == nil {
			err = godotenv.Load(".env")
			if err != nil {
				log.Fatal("Error loading environment variables")
			}
		}

		ConfigFile = &Config{
			DISCORD_TOKEN:      os.Getenv("DISCORD_TOKEN"),
			DATABASE_HOST:      os.Getenv("DATABASE_HOST"),
			AWS_REGION:         os.Getenv("AWS_REGION"),
			AWS_PARAMETER_NAME: os.Getenv("AWS_PARAMETER_NAME"),
		}
		data, _ := json.MarshalIndent(ConfigFile, "", "    ")
		os.WriteFile("config.json", data, 0644)
	} else {
		data, _ := os.ReadFile("config.json")
		err := json.Unmarshal(data, ConfigFile)
		if err != nil {
			log.Fatal(err)
		}
	}

	if ConfigFile.DISCORD_TOKEN == "" && ConfigFile.AWS_PARAMETER_NAME == "" {
		log.Fatal("DISCORD_TOKEN or AWS_PARAMETER_NAME is not set")
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
