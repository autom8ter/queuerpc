# protoc-gen-rabbitmq

a protoc plugin to generate type safe RabbitMQ RPC client and server code based on protobuf service definitions.

## Installation

    go install github.com/autom8ter/protoc-gen-rabbitmq

## Usage

Given the Following protobuf service definition:

    syntax = "proto3";
    package example;
    option go_package = "github.com/autom8ter/protoc-gen-rabbitmq/example";
    service Example {
      rpc Echo(EchoRequest) returns (EchoResponse) {}
    }
    message EchoRequest {
      string message = 1;
    }
    message EchoResponse {
      string message = 1;
    }

a type safe client and server will be generated:

```go
package v1

import "context"
import "github.com/autom8ter/protoc-gen-rabbitmq/rpc"
import "github.com/golang/protobuf/proto"

// EchoServiceServer is a RabbitMQ service
type EchoServiceServer interface {
	Echo(ctx context.Context, in *EchoRequest) (*EchoResponse, error)
}

func Serve(ctx context.Context, srv *rpc.Server, handler EchoServiceServer) error {
	return srv.Serve(ctx, func(ctx context.Context, msg rpc.Message) rpc.Message {
		meta := msg.Metadata
		switch msg.Method {
		case "Echo":
			var in EchoRequest
			if err := proto.Unmarshal(msg.Body, &in); err != nil {
				return rpc.Message{
					ID:       msg.ID,
					Method:   msg.Method,
					Metadata: meta,
					Error:    err,
				}
			}
			out, err := handler.Echo(ctx, &in)
			if err != nil {
				return rpc.Message{
					ID:       msg.ID,
					Method:   msg.Method,
					Metadata: meta,
					Error:    err,
				}
			}
			body, err := proto.Marshal(out)
			if err != nil {
				return rpc.Message{
					ID:       msg.ID,
					Method:   msg.Method,
					Metadata: meta,
					Error:    err,
				}
			}
			return rpc.Message{
				ID:       msg.ID,
				Method:   msg.Method,
				Metadata: meta,
				Body:     body,
			}
		}
		return rpc.Message{
			ID:       msg.ID,
			Method:   msg.Method,
			Metadata: meta,
			Error:    rpc.ErrUnsupportedMethod,
		}
	})
}

// EchoServiceClient is a RabbitMQ client
type EchoServiceClient struct {
	client *rpc.Client
}

func NewEchoServiceClient(client *rpc.Client) *EchoServiceClient {
	return &EchoServiceClient{client: client}
}
func (c *EchoServiceClient) Echo(ctx context.Context, in *EchoRequest) (*EchoResponse, error) {
	var out EchoResponse
	body, err := proto.Marshal(in)
	if err != nil {
		return nil, err
	}
	msg, err := c.client.Request(ctx, rpc.Message{Method: "Echo", Body: body})
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
