// Command notifier is the Lambda that consumes SQS orphan-alert messages and posts them to Slack.
// Credentials come from Secrets Manager (matching slacked's design).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	goslack "github.com/slack-go/slack"

	"github.com/teddymalhan/aws-play/internal/notifier"
)

// slackCredentials is the JSON shape stored in Secrets Manager.
type slackCredentials struct {
	SlackBotToken  string `json:"SLACK_BOT_TOKEN"`
	SlackChannelID string `json:"SLACK_CHANNEL_ID"`
}

// slackClient adapts slack-go to the notifier.SlackPoster interface.
type slackClient struct{ api *goslack.Client }

func (c slackClient) Post(ctx context.Context, channelID string, blocks []goslack.Block) error {
	_, _, _, err := c.api.SendMessageContext(ctx, channelID, goslack.MsgOptionBlocks(blocks...))
	return err
}

func main() {
	ctx := context.Background()

	creds, err := loadCredentials(ctx)
	if err != nil {
		log.Fatalf("load slack credentials: %v", err)
	}

	handler := notifier.Handler{
		Poster:    slackClient{api: goslack.New(creds.SlackBotToken)},
		ChannelID: creds.SlackChannelID,
	}
	lambda.Start(handler.Handle)
}

func loadCredentials(ctx context.Context) (slackCredentials, error) {
	secretName := os.Getenv("SECRET_NAME")
	if secretName == "" {
		return slackCredentials{}, fmt.Errorf("SECRET_NAME env var not set")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return slackCredentials{}, fmt.Errorf("load aws config: %w", err)
	}

	client := secretsmanager.NewFromConfig(cfg, func(o *secretsmanager.Options) {
		// LocalStack: terraform injects AWS_ENDPOINT_URL so the SDK targets the local gateway.
		if ep := os.Getenv("AWS_ENDPOINT_URL"); ep != "" {
			o.BaseEndpoint = aws.String(ep)
		}
	})

	out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: &secretName})
	if err != nil {
		return slackCredentials{}, fmt.Errorf("get secret %s: %w", secretName, err)
	}

	var creds slackCredentials
	if err := json.Unmarshal([]byte(aws.ToString(out.SecretString)), &creds); err != nil {
		return slackCredentials{}, fmt.Errorf("parse secret json: %w", err)
	}
	if creds.SlackBotToken == "" || creds.SlackChannelID == "" {
		return slackCredentials{}, fmt.Errorf("secret missing SLACK_BOT_TOKEN or SLACK_CHANNEL_ID")
	}
	return creds, nil
}
