package main_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/autom8ter/queuerpc"
	v1 "github.com/autom8ter/queuerpc/example/gen/proto/go"
	"github.com/autom8ter/queuerpc/nats"
	"github.com/autom8ter/queuerpc/rabbitmq"
)

var provider = "nats"

func Test(t *testing.T) {
	var (
		rpcClient queuerpc.IClient
		err       error
	)
	if provider == "rabbitmq" {
		rpcClient, err = rabbitmq.NewClient("amqp://guest:guest@localhost:5672/", "echo",
			rabbitmq.WithClientOnRequest(func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error) {
				fmt.Println("sending request", msg.String())
				return msg, nil
			}),
		)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		rpcClient, err = nats.NewClient("", "echo",
			nats.WithClientOnRequest(func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error) {
				fmt.Println("sending request", msg.String())
				return msg, nil
			}),
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	client := v1.NewEchoServiceClient(rpcClient)
	t.Run("echo", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		resp, err := client.Echo(ctx, &v1.EchoRequest{Message: "hello"})
		if err != nil && err != context.Canceled {
			t.Fatal(err)
		}
		if resp.Message != "hello" {
			t.Fatal("expected hello")
		}
	})
	t.Run("echo server stream", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := client.EchoServerStream(ctx, &v1.EchoRequest{Message: "hello"}, func(resp *v1.EchoResponse) {
			t.Log(resp.Message)
		})
		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			t.Fatal(err)
		}
	})
}
