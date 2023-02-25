package main

import (
	"context"

	v1 "github.com/autom8ter/protoc-gen-rabbitmq/example/gen/proto/go"
	"github.com/autom8ter/protoc-gen-rabbitmq/rpc"
)

type server struct{}

func (s *server) Echo(ctx context.Context, req *v1.EchoRequest) (*v1.EchoResponse, error) {
	return &v1.EchoResponse{Message: req.Message}, nil
}

func main() {
	srv, err := rpc.NewServer("amqp://guest:guest@localhost:5672/", "echo")
	if err != nil {
		panic(err)
	}
	if err := v1.Serve(context.Background(), srv, &server{}); err != nil {
		panic(err)
	}
}
