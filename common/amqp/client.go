package amqp

import (
	"context"

	"github.com/juju/errors"
	"github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn    *amqp091.Connection
	channel *amqp091.Channel
}

func NewClient(uri string) (*Client, error) {
	c := &Client{}
	var err error

	config := amqp091.Config{Properties: amqp091.NewConnectionProperties()}
	c.conn, err = amqp091.DialConfig(uri, config)
	if err != nil {
		return nil, errors.Trace(err)
	}

	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, errors.Trace(err)
	}

	return c, nil
}

func (c *Client) SetQos(prefetchCount int) error {
	err := c.channel.Qos(prefetchCount, 0, false)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (c *Client) DeclareQueue(name string) error {
	_, err := c.channel.QueueDeclare(name, true, false, false, false, nil)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (c *Client) QueueBind(queue, exchange string) error {
	err := c.channel.QueueBind(queue, "", exchange, false, nil)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (c *Client) Consume(ctx context.Context, queueName string, autoAck bool) (<-chan amqp091.Delivery, error) {
	ch, err := c.channel.ConsumeWithContext(ctx, queueName, "", autoAck, false, false, false, nil)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return ch, nil
}
