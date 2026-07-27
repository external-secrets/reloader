package sqs

import (
	"context"
	"encoding/json"
	"fmt"

	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/go-logr/logr"

	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	awsListener "github.com/external-secrets/reloader/pkg/listener/aws"
)

// Constants for authentication methods.
const (
	AuthMethodStatic = "static"
	AuthMethodIRSA   = "irsa"
)

type SecretMessage struct {
	Detail SecretMessageDetail `json:"detail"`
}

type SecretMessageDetail struct {
	EventTime         string            `json:"eventTime"`
	RequestParameters RequestParameters `json:"requestParameters"`
}

type RequestParameters struct {
	SecretId string `json:"secretId"`
}

// AWSSQSListener handles AWS SQS notifications.
type AWSSQSListener struct {
	context   context.Context
	listener  *awsListener.AWSSQSListener
	eventChan chan events.SecretRotationEvent
	logger    logr.Logger
}

// Start begins polling the SQS queue for messages.
func (h *AWSSQSListener) Start() error {
	msgCh, errCh := h.listener.Start()
	go func() {
		for {
			select {
			case messages, ok := <-msgCh:
				if !ok {
					h.logger.Info("Message channel closed, stopping listener...")
					return
				}
				for _, message := range messages {
					if err := h.processMessage(message); err != nil {
						h.logger.Error(err, "Failed to process message")
						continue
					}
				}
			case err, ok := <-errCh:
				if !ok {
					h.logger.Info("Error channel closed, stopping listener...")
					return
				}
				h.logger.Error(err, "Error receiving SQS messages")
			}
		}
	}()
	return nil
}

// processMessage processes an SQS message and publishes the result to the eventChan.
func (h *AWSSQSListener) processMessage(message sqstypes.Message) error {
	if message.Body == nil {
		h.logger.Error(fmt.Errorf("empty body"), "Received message with empty body")
		return fmt.Errorf("received message with empty body")
	}
	h.logger.Info("Processing message", "MessageBody", *message.Body)
	// Unmarshal the message body into a events.SecretRotationEvent

	event, err := parseEvent([]byte(*message.Body))
	if err != nil {
		h.logger.Error(err, "Failed to parse message body")
		return fmt.Errorf("failed to parse message body")
	}

	// Publish the event to the eventChan
	select {
	case h.eventChan <- *event:
		h.logger.Info("Published event to eventChan", "Event", event)
		return nil
	case <-h.context.Done():
		return h.context.Err()
	}
}

// Stop stops polling the SQS queue.
func (h *AWSSQSListener) Stop() error {
	h.logger.Info("Stopping AWS SQS Listener...")
	return h.listener.Stop()
}

func parseEvent(jsonData []byte) (*events.SecretRotationEvent, error) {
	var event SecretMessage
	err := json.Unmarshal(jsonData, &event)
	if err != nil {
		return nil, err
	}

	secretEvent := &events.SecretRotationEvent{
		SecretIdentifier:  event.Detail.RequestParameters.SecretId,
		RotationTimestamp: event.Detail.EventTime,
		TriggerSource:     schema.AWS_SQS,
	}

	return secretEvent, nil
}
