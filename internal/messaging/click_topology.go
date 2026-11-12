package messaging

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ClickExchange           = "click_events"
	ClickRoutingKey         = "click"
	ClickQueue              = "click_events.count"
	ClickDeadLetterExchange = "click_events.dlx"
	ClickDeadLetterQueue    = "click_events.dead"

	clickDeliveryLimit = 20
)

// DeclareClickTopology declares the click exchanges and queues. The API and the
// worker both call it with the same arguments, so either can start first;
// RabbitMQ rejects a redeclaration whose arguments differ.
func DeclareClickTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(ClickDeadLetterExchange, amqp.ExchangeFanout, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare click dead-letter exchange: %w", err)
	}

	if _, err := ch.QueueDeclare(ClickDeadLetterQueue, true, false, false, false, amqp.Table{
		amqp.QueueTypeArg: amqp.QueueTypeQuorum,
	}); err != nil {
		return fmt.Errorf("declare click dead-letter queue: %w", err)
	}

	if err := ch.QueueBind(ClickDeadLetterQueue, "", ClickDeadLetterExchange, false, nil); err != nil {
		return fmt.Errorf("bind click dead-letter queue: %w", err)
	}

	if err := ch.ExchangeDeclare(ClickExchange, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare click exchange: %w", err)
	}

	// At-least-once dead-lettering requires the reject-publish overflow mode.
	if _, err := ch.QueueDeclare(ClickQueue, true, false, false, false, amqp.Table{
		amqp.QueueTypeArg:        amqp.QueueTypeQuorum,
		amqp.QueueOverflowArg:    amqp.QueueOverflowRejectPublish,
		"x-delivery-limit":       clickDeliveryLimit,
		"x-dead-letter-exchange": ClickDeadLetterExchange,
		"x-dead-letter-strategy": "at-least-once",
	}); err != nil {
		return fmt.Errorf("declare click queue: %w", err)
	}

	if err := ch.QueueBind(ClickQueue, ClickRoutingKey, ClickExchange, false, nil); err != nil {
		return fmt.Errorf("bind click queue: %w", err)
	}

	return nil
}
