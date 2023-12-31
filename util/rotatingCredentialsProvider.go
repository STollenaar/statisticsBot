package util

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/pelletier/go-toml/v2"
)

const (
	// RotatingCredentialsName provides a name of Static provider
	RotatingCredentialsName = "RotatingCredentials"
)

type Credentials struct {
	AccessKeyID     string `toml:"aws_access_key_id"`
	SecretAccessKey string `toml:"aws_secret_access_key"`
	SessionToken    string `toml:"aws_session_token"`
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
	fmt.Println("Fetching Credentials")
	if s.FilePath == "" {
		return aws.Credentials{
			Source: RotatingCredentialsName,
		}, &RotatingCredentialsEmptyError{}
	}

	file, err := os.ReadFile(s.FilePath)
	var creds Credentials

	if err != nil {
		return aws.Credentials{
			Source: RotatingCredentialsName,
		}, err
	}
	err = toml.Unmarshal(file, &creds)
	if err != nil {
		return aws.Credentials{
			Source: RotatingCredentialsName,
		}, err
	}

	newT := time.Now().Add(time.Duration(time.Hour))
	fmt.Printf("Credentials will expire at: %v\n", newT.Format(time.RFC1123))
	return aws.Credentials{
		AccessKeyID:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		Source:          RotatingCredentialsName,

		CanExpire: true,
		Expires:   newT,
	}, nil
}
