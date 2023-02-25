package rpc

import "github.com/rabbitmq/amqp091-go"

type session struct {
	conn    *amqp091.Connection
	channel *amqp091.Channel
}
