package rpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/autom8ter/protoc-gen-rabbitmq/rpc"
)

func TestServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, err := rpc.NewServer("amqp://guest:guest@localhost:5672/", "echo")
	if err != nil {
		panic(err)
	}
	err = srv.Serve(ctx, func(ctx context.Context, msg rpc.Message) rpc.Message {
		return msg
	})
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Fatal(err)
	}
}
