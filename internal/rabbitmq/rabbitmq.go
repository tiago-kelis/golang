package rabbitmq

import (
	"fmt"

	"github.com/streadway/amqp"
)

type RabbitClient struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	url     string
}

func newConnection(url string) (*amqp.Connection, *amqp.Channel, error) {

	conn, err := amqp.Dial(url)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to connectiont to Rabbitmq: %v", err)
	}

	channel, err := conn.Channel()

	if err != nil {
		return nil, nil, fmt.Errorf("failed to open a channel: %v", err)
	}

	return conn, channel, nil

}

func NewRabbitClint(connectionUrl string) (*RabbitClient, error) {

	conn, channel, err := newConnection(connectionUrl)

	if err != nil {

		return nil, err
	}

	return &RabbitClient{
		conn:    conn,
		channel: channel,
		url:     connectionUrl,
	}, nil

}

func (client *RabbitClient) ConsumeMessages(exchange, routingkey, queueName string) (<-chan amqp.Delivery, error) {

	err := client.channel.ExchangeDeclare(
		exchange,
		"direct",
		true,
		true,
		false,
		false,
		nil,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to declare exchange: %v", err)
	}

	queue, err := client.channel.QueueDeclare(
		queueName,
		true,
		true,
		false,
		false,
		nil,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to declare queue")
	}

	err = client.channel.QueueBind(queue.Name, routingkey, exchange, false, nil)

	if err != nil {
		return nil, fmt.Errorf("Failed to bind queue: %v", err)
	}

	msgs, err := client.channel.Consume(
		queueName,
		"goapp",
		false,
		false,
		false,
		false,
		nil,
	)

	if err != nil {

		return nil, fmt.Errorf("Failed to consume messages from queue: %v", err)

	}

	return msgs, nil

}

func (client *RabbitClient) PublishMessage(exchange, routingkey, queueName string, message []byte) error {

	err := client.channel.ExchangeDeclare(
		exchange,
		"direct",
		true,
		true,
		false,
		false,
		nil,
	)

	if err != nil {
		return fmt.Errorf("failed to declare exchange: %v", err)
	}

	queue, err := client.channel.QueueDeclare(
		queueName,
		true,
		true,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue")
	}

	err = client.channel.QueueBind(queue.Name, routingkey, exchange, false, nil)

	if err != nil {
		return fmt.Errorf("Failed to bind queue: %v", err)
	}

	err = client.channel.Publish(
		exchange, routingkey, false, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        message,
		},
	)

	if err != nil {
		return fmt.Errorf("Failed to publish message: %v", err)
	}

	return nil

}

func (client *RabbitClient) Close() {
	client.conn.Close()
	client.channel.Close()
}
