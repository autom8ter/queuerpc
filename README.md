# queuerpc

a protoc plugin to generate type safe RabbitMQ RPC client and server code based on protobuf service definitions.

## Installation

    go install github.com/autom8ter/queuerpc/

## Usage

Given the Following protobuf service definition:

```protobuf
syntax = "proto3";

package echo.v1;
option go_package = "github.com/autom8ter/example/echo/v1";


message EchoRequest {
  string message = 1;
}

message EchoResponse {
  string message = 1;
}

service EchoService {
  // Echo returns the same message it receives.
  rpc Echo(EchoRequest) returns (EchoResponse) {}
}
```

a type safe client and server will be generated:

```go
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
func Serve(ctx context.Context, srv queuerpc.IServer, handler EchoServiceServer) error {
	return srv.Serve(ctx, func(ctx context.Context, msg *queuerpc.Message) *queuerpc.Message {
		meta := msg.Metadata
		switch msg.Method {
		case "Echo":
			var in EchoRequest
			if err := proto.Unmarshal(msg.Body, &in); err != nil {
				return &queuerpc.Message{
					Id:       msg.Id,
					Method:   msg.Method,
					Metadata: meta,
					Error:    err,
				}
			}
			out, err := handler.Echo(queuerpc.NewContextWithMetadata(ctx, meta), &in)
			if err != nil {
				return &queuerpc.Message{
					Id:       msg.Id,
					Method:   msg.Method,
					Metadata: meta,
					Error:    err,
				}
			}
			body, err := proto.Marshal(out)
			if err != nil {
				return &queuerpc.Message{
					Id:       msg.Id,
					Method:   msg.Method,
					Metadata: meta,
					Error:    err,
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

```

    

see [example](example) for a full example of a client and server
example code generated using [Makefile](Makefile)
