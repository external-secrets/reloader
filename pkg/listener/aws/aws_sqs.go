package listener

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authAWS "github.com/external-secrets/reloader/pkg/auth/aws"
	modelAWS "github.com/external-secrets/reloader/pkg/models/aws"
)

type SQSClientInterface interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// AWSSQSListener handles AWS SQS notifications.
type AWSSQSListener struct {
	context   context.Context
	cancel    context.CancelFunc
	client    client.Client
	config    *modelAWS.AWSSQSConfig
	sqsClient SQSClientInterface
	logger    logr.Logger
}

// NewAWSSQSListener creates a new AWSSQSListener.
func NewAWSSQSListener(ctx context.Context, config *modelAWS.AWSSQSConfig, client client.Client, logger logr.Logger) (*AWSSQSListener, error) {
	// Load AWS config with appropriate authentication
	awsConfig, err := authAWS.CreateAWSSDKConfig(ctx, client, config.Auth, logger)
	if err != nil {
		logger.Error(err, "Failed to create AWS config")
		return nil, fmt.Errorf("failed to create AWS config: %w", err)
	}

	// Initialize SQS client
	sqsClient := sqs.NewFromConfig(awsConfig)
	ctx, cancel := context.WithCancel(ctx)
	logger.Info("Created new AWSSQSListener")
	return &AWSSQSListener{
		context:   ctx,
		cancel:    cancel,
		client:    client,
		config:    config,
		sqsClient: sqsClient,
		logger:    logger,
	}, nil
}

func (h *AWSSQSListener) SetSQSClient(sqsClient SQSClientInterface) error {
	h.sqsClient = sqsClient
	return nil
}

// Start begins polling the SQS queue for messages and yields them through a channel.
func (h *AWSSQSListener) Start() (<-chan []types.Message, <-chan error) {
	h.logger.Info("Starting AWS SQS Listener...")

	msgCh := make(chan []types.Message)
	errCh := make(chan error, 1)

	go func() {
		defer func() {
			h.logger.Info("Stopping AWS SQS Listener...")
			close(msgCh)
			close(errCh)
		}()

		for {
			select {
			case <-h.context.Done():
				return
			default:
				// Poll messages from SQS
				messages, err := h.PollMessages()
				if err != nil {
					h.logger.Error(err, "Error polling messages")
					select {
					case errCh <- err:
					default:
					}
					select {
					case <-time.After(5 * time.Second):
					case <-h.context.Done():
						return
					}
				} else if len(messages) > 0 {
					select {
					case msgCh <- messages:
					case <-h.context.Done():
						return
					}
				}
			}
		}
	}()

	return msgCh, errCh
}

// pollMessages fetches messages from the SQS queue and returns them as an array.
func (h *AWSSQSListener) PollMessages() ([]types.Message, error) {
	h.logger.Info("Polling messages from SQS", "QueueURL", h.config.QueueURL)

	// Receive messages from SQS
	output, err := h.sqsClient.ReceiveMessage(h.context, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(h.config.QueueURL),
		MaxNumberOfMessages: h.config.MaxNumberOfMessages,
		WaitTimeSeconds:     h.config.WaitTimeSeconds,
		VisibilityTimeout:   h.config.VisibilityTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to receive messages from SQS: %w", err)
	}

	h.logger.Info("Received messages from SQS", "MessageCount", len(output.Messages))

	// Copy messages to a separate array before deleting them
	messages := make([]types.Message, len(output.Messages))
	copy(messages, output.Messages)

	// Delete messages after storing them
	for _, message := range output.Messages {
		_, err := h.sqsClient.DeleteMessage(h.context, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(h.config.QueueURL),
			ReceiptHandle: message.ReceiptHandle,
		})
		if err != nil {
			h.logger.Error(err, "Failed to delete message", "MessageID", *message.MessageId)
		}
	}

	return messages, nil
}

// Stop stops polling the SQS queue and ensures all channels are properly closed.
func (h *AWSSQSListener) Stop() error {
	h.logger.Info("Stopping AWS SQS Listener...")
	h.cancel()
	// Wait for goroutine to exit before returning
	select {
	case <-h.context.Done():
		h.logger.Info("AWS SQS Listener stopped gracefully.")
	case <-time.After(2 * time.Second):
		h.logger.Info("AWS SQS Listener took too long to stop, forcing exit.")
	}

	return nil
}
