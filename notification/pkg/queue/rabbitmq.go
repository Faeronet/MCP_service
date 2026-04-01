package queue

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func New(ctx context.Context, url string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("channel: %w", err)
	}
	// Declare queues we use
	for _, q := range []string{"ingestion_jobs", "attachment_jobs"} {
		_, _ = ch.QueueDeclare(q, true, false, false, false, nil)
	}
	return &Client{conn: conn, channel: ch}, nil
}

func (c *Client) Publish(ctx context.Context, queue string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return c.channel.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Body:         data,
	})
}

func (c *Client) Consume(queue string) (<-chan amqp.Delivery, error) {
	return c.channel.Consume(queue, "", false, false, false, false, nil)
}

func (c *Client) Close() error {
	if c.channel != nil {
		_ = c.channel.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
