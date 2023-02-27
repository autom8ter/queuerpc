package rpc

import (
	"context"
	"encoding/gob"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

func init() {
	gob.Register(&Message{})
}

// Message is an rpc message sent over rabbitmq it is used as a client side request and server side response
type Message struct {
	// ID is the unique id of the message - this is set internally on each request
	ID string
	// Method is the method to call on the server
	Method string
	// Body is the body of the message
	Body []byte
	// Metadata is the metadata of the message
	Metadata map[string]any
	// Error is the error of the message if any
	Error error
}

var (

	// ErrUnsupportedMethod is returned when the method is not supported
	ErrUnsupportedMethod = fmt.Errorf("unsupported method")
	// ErrEmptyMessageReceived is returned when the message is empty
	ErrEmptyMessageReceived = fmt.Errorf("empty message received")
	// ErrChannelClosed is returned when the channel is closed
	ErrChannelClosed = fmt.Errorf("channel closed")
	// ErrDecodeMessage is returned when the message cannot be decoded
	ErrDecodeMessage = fmt.Errorf("failed to decode message")
	// ErrEncodeMessage is returned when the message cannot be encoded
	ErrEncodeMessage = fmt.Errorf("failed to encode message")
	// ErrPublishMessage is returned when the message cannot be published
	ErrPublishMessage = fmt.Errorf("failed to publish message")
)

type session struct {
	conn    *amqp091.Connection
	channel *amqp091.Channel
}

type ctxKey struct{}

var metadataKey = ctxKey{}

// MetadataFromContext returns the metadata from the context
func MetadataFromContext(ctx context.Context) map[string]any {
	if md, ok := ctx.Value(metadataKey).(map[string]any); ok {
		return md
	}
	return map[string]any{}
}

// NewContextWithMetadata returns a new context with the metadata
func NewContextWithMetadata(ctx context.Context, md map[string]any) context.Context {
	return context.WithValue(ctx, metadataKey, md)
}
