package v1

import "context"
import "github.com/autom8ter/queuerpc"
import "github.com/golang/protobuf/proto"

// EchoServiceServer is a type safe RabbitMQ rpc server
type EchoServiceServer interface {
	// Echo returns the same message it receives.
	Echo(ctx context.Context, in *EchoRequest) (*EchoResponse, error)
	// EchoServerStream streams the same message it initially receives.
	EchoServerStream(ctx context.Context, in *EchoRequest) (<-chan *EchoResponse, error)
}

// Serve starts the server and blocks until the context is canceled or the deadline is exceeded
func Serve(srv queuerpc.IServer, handler EchoServiceServer) error {
	return srv.Serve(queuerpc.Handlers{
		UnaryHandler: func(ctx context.Context, msg *queuerpc.Message) *queuerpc.Message {
			meta := msg.Metadata
			switch msg.Method {
			case "Echo":
				var in EchoRequest
				if err := proto.Unmarshal(msg.Body, &in); err != nil {
					return &queuerpc.Message{
						Id:       msg.Id,
						Method:   msg.Method,
						Metadata: meta,
						Error:    queuerpc.ErrUnmarshal,
					}
				}
				out, err := handler.Echo(queuerpc.NewContextWithMetadata(ctx, meta), &in)
				if err != nil {
					return &queuerpc.Message{
						Id:       msg.Id,
						Method:   msg.Method,
						Metadata: meta,
						Error:    queuerpc.ErrorFrom(err),
					}
				}
				body, err := proto.Marshal(out)
				if err != nil {
					return &queuerpc.Message{
						Id:       msg.Id,
						Method:   msg.Method,
						Metadata: meta,
						Error:    queuerpc.ErrMarshal,
					}
				}
				return &queuerpc.Message{
					Id:       msg.Id,
					Method:   msg.Method,
					Metadata: meta,
					Body:     body,
				}
			}
			return &queuerpc.Message{
				Id:       msg.Id,
				Method:   msg.Method,
				Metadata: meta,
				Error:    queuerpc.ErrUnsupportedMethod,
			}
		},
		ServerStreamHandler: func(ctx context.Context, msg *queuerpc.Message) (<-chan *queuerpc.Message, error) {
			meta := msg.Metadata
			ch := make(chan *queuerpc.Message)
			switch msg.Method {
			default:
				return nil, queuerpc.ErrUnsupportedMethod
			case "EchoServerStream":
				var in EchoRequest
				if err := proto.Unmarshal(msg.Body, &in); err != nil {
					return nil, queuerpc.ErrUnmarshal
				}
				out, err := handler.EchoServerStream(queuerpc.NewContextWithMetadata(ctx, meta), &in)
				if err != nil {
					return nil, err
				}
				go func() {
					defer close(ch)
					for {
						select {
						case <-ctx.Done():
							return
						case out, ok := <-out:
							if !ok {
								return
							}
							body, _ := proto.Marshal(out)
							ch <- &queuerpc.Message{
								Id:       msg.Id,
								Method:   msg.Method,
								Metadata: meta,
								Body:     body,
							}
						}
					}
				}()
				return ch, nil
			}
		},
	})
}

// EchoServiceClient is a type safe RabbitMQ rpc client
type EchoServiceClient struct {
	client queuerpc.IClient
}

// NewEchoServiceClient returns a new EchoServiceClientwith the given rpc client
func NewEchoServiceClient(client queuerpc.IClient) *EchoServiceClient {
	return &EchoServiceClient{client: client}
}

// Echo returns the same message it receives.
func (c *EchoServiceClient) Echo(ctx context.Context, in *EchoRequest) (*EchoResponse, error) {
	meta := queuerpc.MetadataFromContext(ctx)
	var out EchoResponse
	body, err := proto.Marshal(in)
	if err != nil {
		return nil, err
	}
	msg, err := c.client.Request(ctx, &queuerpc.Message{Method: "Echo", Body: body, Metadata: meta})
	if err != nil {
		return nil, err
	}
	if msg.Error != nil {
		return nil, msg.Error
	}
	if err := proto.Unmarshal(msg.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EchoServerStream streams the same message it initially receives.
func (c *EchoServiceClient) EchoServerStream(ctx context.Context, in *EchoRequest, handler func(*EchoResponse)) error {
	meta := queuerpc.MetadataFromContext(ctx)
	body, err := proto.Marshal(in)
	if err != nil {
		return err
	}
	return c.client.ServerStream(ctx, &queuerpc.Message{Method: "EchoServerStream", Body: body, Metadata: meta}, func(msg *queuerpc.Message) {
		var out EchoResponse
		if err := proto.Unmarshal(msg.Body, &out); err != nil {
			return
		}
		handler(&out)
	})
}
