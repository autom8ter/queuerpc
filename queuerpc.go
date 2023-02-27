package queuerpc

import (
	"context"
	"fmt"
	"time"
)

// IClient is the interface for an rpc client
type IClient interface {
	// Request sends a request to the server and returns the response
	Request(ctx context.Context, body *Message, opts ...RequestOption) (*Message, error)
	// Close closes the client and waits for all pending requests to complete
	Close() error
}

// IServer is the interface for an rpc server
type IServer interface {
	// Serve starts the server and blocks until the context is canceled or the deadline is exceeded
	Serve(handler HandlerFunc) error
	// Close closes the server and waits for all pending requests to complete
	Close() error
}

// HandlerFunc is a function that handles a message.
// The message returned will be sent back to the client
// If an error is encountered, an error should be added to the Message
type HandlerFunc func(ctx context.Context, msg *Message) *Message

var (
	// ErrUnsupportedMethod is returned when the method is not supported
	ErrUnsupportedMethod = &Error{Code: 1, Message: "unsupported method"}
	ErrUnmarshal         = &Error{Code: 2, Message: "failed to unmarshal request"}
	ErrMarshal           = &Error{Code: 3, Message: "failed to marshal response"}
	ErrUnknown           = &Error{Code: 4, Message: "unknown error"}
)

type ctxKey struct{}

var metadataKey = ctxKey{}

// MetadataFromContext returns the metadata from the context
func MetadataFromContext(ctx context.Context) map[string]string {
	if md, ok := ctx.Value(metadataKey).(map[string]string); ok {
		return md
	}
	return map[string]string{}
}

// NewContextWithMetadata returns a new context with the metadata
func NewContextWithMetadata(ctx context.Context, md map[string]string) context.Context {
	return context.WithValue(ctx, metadataKey, md)
}

// RequestOpts are the options for a client side rpc request
type RequestOpts struct {
	Timeout time.Duration
}

// RequestOption is a function that configures a a client side rpc request
type RequestOption func(*RequestOpts)

// WithTimeout sets the timeout for the request
func WithTimeout(t time.Duration) RequestOption {
	return func(o *RequestOpts) {
		o.Timeout = t
	}
}

// Error returns the rpc error as a string
func (e *Error) Error() string {
	return fmt.Sprintf("rpc error: code = %d desc = %s", e.Code, e.Message)
}

// ErrorFrom returns an rpc error from an error
func ErrorFrom(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e
	}
	e := *ErrUnknown
	e.Message = err.Error()
	return &e
}
