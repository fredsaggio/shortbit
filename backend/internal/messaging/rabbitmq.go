package messaging

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const connectTimeout = 5 * time.Second

func Connect(url string, connectionName string) (*amqp.Connection, error) {
	properties := amqp.NewConnectionProperties()
	properties.SetClientConnectionName(connectionName)

	conn, err := amqp.DialConfig(url, amqp.Config{
		Dial:       amqp.DefaultDial(connectTimeout),
		Properties: properties,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	return conn, nil
}
