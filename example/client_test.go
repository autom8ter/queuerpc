package main_test

import (
	"context"
	"testing"

	v1 "github.com/autom8ter/protoc-gen-rabbitmq/example/gen/proto/go"
	"github.com/autom8ter/protoc-gen-rabbitmq/rpc"
)

func Test(t *testing.T) {
	rpcClient, err := rpc.NewClient("amqp://guest:guest@localhost:5672/", "echo")
	if err != nil {
		t.Fatal(err)
	}
	if err := rpcClient.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	client := v1.NewEchoServiceClient(rpcClient)
	resp, err := client.Echo(context.Background(), &v1.EchoRequest{Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Message != "hello" {
		t.Fatal("expected hello")
	}
	t.Log(resp.Message)
}
