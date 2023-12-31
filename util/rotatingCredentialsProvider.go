package util

import (
	"context"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spf13/viper"
)

const (
	// RotatingCredentialsName provides a name of Static provider
	RotatingCredentialsName = "RotatingCredentials"
)

type DefaultCredentials struct {
	Default Credentials `mapstructure:"default"`
}

type Credentials struct {
	AccessKeyID     string `mapstructure:"aws_access_key_id"`
	SecretAccessKey string `mapstructure:"aws_secret_access_key"`
	SessionToken    string `mapstructure:"aws_session_token"`
	Region          string `mapstructure:"region"`
	Output          string `mapstructure:"output"`
}

// RotatingCredentialsEmptyError is emitted when static credentials are empty.
type RotatingCredentialsEmptyError struct{}

func (*RotatingCredentialsEmptyError) Error() string {
	return "rotating credentials are empty"
}

// A RotatingCredentialsProvider is a set of credentials which are set, and will
// never expire.
type RotatingCredentialsProvider struct {
	FilePath string
}

// NewRotatingCredentialsProvider return a RotatingCredentialsProvider initialized with the AWS
// credentials passed in.
func NewRotatingCredentialsProvider(file string) RotatingCredentialsProvider {
	return RotatingCredentialsProvider{
		FilePath: file,
	}
}

// Retrieve returns the credentials or error if the credentials are invalid.
func (s RotatingCredentialsProvider) Retrieve(_ context.Context) (aws.Credentials, error) {
	// fmt.Println("Fetching Credentials")
	if s.FilePath == "" {
		return aws.Credentials{
			Source: RotatingCredentialsName,
		}, &RotatingCredentialsEmptyError{}
	}

	var creds DefaultCredentials
	viper.SetConfigName(path.Base(s.FilePath))
	viper.AddConfigPath(path.Dir(s.FilePath))
	viper.SetConfigType("ini")
	err := viper.ReadInConfig()
	if err != nil {
		return aws.Credentials{
			Source: RotatingCredentialsName,
		}, err
	}

	err = viper.Unmarshal(&creds)
	if err != nil {
		return aws.Credentials{
			Source: RotatingCredentialsName,
		}, err
	}

	newT := time.Now().Add(time.Duration(time.Minute*2))
	// fmt.Printf("Credentials will expire at: %v\n", newT.Format(time.RFC1123))
	return aws.Credentials{
		AccessKeyID:     creds.Default.AccessKeyID,
		SecretAccessKey: creds.Default.SecretAccessKey,
		SessionToken:    creds.Default.SessionToken,
		Source:          RotatingCredentialsName,

		CanExpire: true,
		Expires:   newT,
	}, nil
}
