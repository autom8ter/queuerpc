package rpc_test

import (
	"context"
	"testing"

	"github.com/autom8ter/protoc-gen-rabbitmq/rpc"
)

func TestClient(t *testing.T) {
	rpcClient, err := rpc.NewClient("amqp://guest:guest@localhost:5672/", "echo")
	if err != nil {
		t.Fatal(err)
	}
	if err := rpcClient.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
}
