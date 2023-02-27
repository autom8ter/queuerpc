package main_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/autom8ter/queuerpc"
	v1 "github.com/autom8ter/queuerpc/example/gen/proto/go"
	"github.com/autom8ter/queuerpc/rabbitmq"
)

func Test(t *testing.T) {
	rpcClient, err := rabbitmq.NewClient("amqp://guest:guest@localhost:5672/", "echo",
		rabbitmq.WithClientOnRequest(func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error) {
			fmt.Println("sending request", msg.String())
			return msg, nil
		}))
	if err != nil {
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
