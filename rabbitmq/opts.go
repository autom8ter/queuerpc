package rabbitmq

import (
	"context"
	"crypto/tls"

	"github.com/autom8ter/queuerpc"
)

type clientOpts struct {
	tls          *tls.Config
	errorHandler func(msg string, err error)
	onRequest    func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)
	onResponse   func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)
}

// ClientOption is a function that configures a client
type ClientOption func(opts *clientOpts)

// WithClientTLS sets the tls config for the client
func WithClientTLS(cfg *tls.Config) ClientOption {
	return func(opts *clientOpts) {
		opts.tls = cfg
	}
}

// WithClientErrorHandler sets the async error handler for the client
func WithClientErrorHandler(f func(msg string, err error)) ClientOption {
	return func(opts *clientOpts) {
		opts.errorHandler = f
	}
}

// WithClientOnRequest sets the onRequest handler for the client
// onRequest is called before a request is sent to the server and can be used to modify the request or add things like logging/validation
func WithClientOnRequest(f func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)) ClientOption {
	return func(opts *clientOpts) {
		opts.onRequest = f
	}
}

// WithClientOnResponse sets the onResponse handler for the client
// this is called when the client receives a response from the server and can be used to modify the response or add things like logging/validation
func WithClientOnResponse(f func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)) ClientOption {
	return func(opts *clientOpts) {
		opts.onResponse = f
	}
}

// serverOpts are options for the server
type serverOpts struct {
	tls          *tls.Config
	errorHandler func(msg string, err error)
	onRequest    func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)
	onResponse   func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)
}

// ServerOption is a function that configures a server
type ServerOption func(opts *serverOpts)

// WithServerTLS sets the tls config for the server
func WithServerTLS(cfg *tls.Config) ServerOption {
	return func(opts *serverOpts) {
		opts.tls = cfg
	}
}

// WithServerErrorHandler sets the async error handler for the server
func WithServerErrorHandler(f func(msg string, err error)) ServerOption {
	return func(opts *serverOpts) {
		opts.errorHandler = f
	}
}

// WithServerOnRequest sets the onRequest handler for the server
// onRequest is called when the server receives a request and can be used to modify the request or add things like logging/validation
func WithServerOnRequest(f func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)) ServerOption {
	return func(opts *serverOpts) {
		opts.onRequest = f
	}
}

// WithServerOnResponse sets the onResponse handler for the server
// this is called after the server has processed the request and can be used to modify the response or add things like logging/validation
func WithServerOnResponse(f func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)) ServerOption {
	return func(opts *serverOpts) {
		opts.onResponse = f
	}
}
