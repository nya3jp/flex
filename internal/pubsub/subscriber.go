package pubsub

import (
	"context"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/pubsub"
)

type Message = pubsub.Message

type Subscriber struct {
	client       *pubsub.Client
	subscription *pubsub.Subscription
}

func NewSubscriber(ctx context.Context, subscriptionID string, maxOutstandingMessages int) (*Subscriber, error) {
	projectID, err := metadata.ProjectID()
	if err != nil {
		return nil, err
	}

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	subscription := client.Subscription(subscriptionID)
	subscription.ReceiveSettings.MaxOutstandingMessages = maxOutstandingMessages
	subscription.ReceiveSettings.Synchronous = true

	return &Subscriber{
		client:       client,
		subscription: subscription,
	}, nil
}

func (s *Subscriber) Close() error {
	return s.client.Close()
}

func (s *Subscriber) Receive(ctx context.Context, f func(ctx context.Context, msg *Message)) error {
	return s.subscription.Receive(ctx, f)
}
