package pubsub

import (
	"context"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/pubsub"
)

type Publisher struct {
	client *pubsub.Client
	topic  *pubsub.Topic
}

func NewPublisher(ctx context.Context, topicID string) (*Publisher, error) {
	projectID, err := metadata.ProjectID()
	if err != nil {
		return nil, err
	}

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	topic := client.Topic(topicID)

	return &Publisher{
		client: client,
		topic:  topic,
	}, nil
}

func (p *Publisher) Close() error {
	p.topic.Stop()
	return p.client.Close()
}

func (p *Publisher) Send(ctx context.Context) error {
	result := p.topic.Publish(ctx, &pubsub.Message{
		Data: []byte("{}"),
	})
	_, err := result.Get(ctx)
	return err
}
