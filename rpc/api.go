package rpc

import "context"

// IClient is the interface for an rpc client
type IClient interface {
	// Connect connects the client to the rabbitmq server and begins asynchronously processing messages
	Connect(ctx context.Context) error
	// Request sends a request to the server and returns the response
	Request(ctx context.Context, body Message, opts ...RequestOption) (*Message, error)
	// Close closes the client and waits for all pending requests to complete
	Close() error
}

// IServer is the interface for an rpc server
type IServer interface {
	// Serve starts the server and blocks until the context is canceled or the deadline is exceeded
	Serve(ctx context.Context, handler HandlerFunc) error
}
