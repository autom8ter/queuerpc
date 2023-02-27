package rabbitmq

import (
	"github.com/autom8ter/queuerpc"
)

var (
	// ErrEmptyMessageReceived is returned when the message is empty
	ErrEmptyMessageReceived = &queuerpc.Error{Code: 9001, Message: "empty message received"}
	// ErrChannelClosed is returned when the channel is closed
	ErrChannelClosed = &queuerpc.Error{Code: 9002, Message: "channel closed"}
	// ErrPublishMessage is returned when the message cannot be published
	ErrPublishMessage = &queuerpc.Error{Code: 9005, Message: "failed to publish message"}
)
