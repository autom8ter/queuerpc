package main

import (
	"context"
	"fmt"

	"github.com/autom8ter/queuerpc"
	v1 "github.com/autom8ter/queuerpc/example/gen/proto/go"
	"github.com/autom8ter/queuerpc/nats"
	"github.com/autom8ter/queuerpc/rabbitmq"
)

type server struct{}

func (s *server) Echo(ctx context.Context, req *v1.EchoRequest) (*v1.EchoResponse, error) {
	return &v1.EchoResponse{Message: req.Message}, nil
}

func (s *server) EchoServerStream(ctx context.Context, req *v1.EchoRequest) (<-chan *v1.EchoResponse, error) {
	ch := make(chan *v1.EchoResponse)
	go func() {
		defer close(ch)
		for i := 0; i < 10; i++ {
			ch <- &v1.EchoResponse{Message: req.Message}
		}
	}()
	return ch, nil
}

var (
	provider = "nats"
)

func main() {
	if provider == "rabbitmq" {
		srv, err := rabbitmq.NewServer("amqp://guest:guest@localhost:5672/", "echo",
			rabbitmq.WithServerOnRequest(func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error) {
				fmt.Println("received request", msg.String())
				return msg, nil
			}),
			rabbitmq.WithServerOnResponse(func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error) {
				fmt.Println("sending response", msg.String())
				return msg, nil
			}),
		)
		if err != nil {
			panic(err)
		}
		if err := v1.Serve(srv, &server{}); err != nil {
			panic(err)
		}
	} else {
		srv, err := nats.NewServer("", "echo",
			nats.WithServerOnRequest(func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error) {
				fmt.Println("received request", msg.String())
				return msg, nil
			}),
			nats.WithServerOnResponse(func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error) {
				fmt.Println("sending response", msg.String())
				return msg, nil
			}),
		)
		if err != nil {
			panic(err)
		}
		if err := v1.Serve(srv, &server{}); err != nil {
			panic(err)
		}
	}
}
