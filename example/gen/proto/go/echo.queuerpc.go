package v1

import "context"
import "github.com/autom8ter/queuerpc"
import "github.com/golang/protobuf/proto"

// EchoServiceServer is a type safe RabbitMQ rpc server
type EchoServiceServer interface {
	// Echo returns the same message it receives.
	Echo(ctx context.Context, in *EchoRequest) (*EchoResponse, error)
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
		ClientStreamHandler: nil,
		ServerStreamHandler: nil,
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
