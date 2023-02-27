package rabbitmq_test

import (
	"context"
	"testing"
	"time"

	"github.com/autom8ter/queuerpc"
	"github.com/autom8ter/queuerpc/rabbitmq"
)

func TestServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, err := rabbitmq.NewServer("amqp://guest:guest@localhost:5672/", "echo")
	if err != nil {
		panic(err)
	}
	err = srv.Serve(ctx, func(ctx context.Context, msg *queuerpc.Message) *queuerpc.Message {
		return msg
	})
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Fatal(err)
	}
}
