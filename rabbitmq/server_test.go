package rabbitmq_test

import (
	"context"
	"testing"
	"time"

	"github.com/autom8ter/queuerpc"
	"github.com/autom8ter/queuerpc/rabbitmq"
)

func TestServer(t *testing.T) {
	srv, err := rabbitmq.NewServer("amqp://guest:guest@localhost:5672/", "echo")
	if err != nil {
		panic(err)
	}
	go func() {
		<-time.After(5 * time.Second)
		t.Log("closing server")
		if err := srv.Close(); err != nil {
			t.Error(err)
			return
		}
	}()
	err = srv.Serve(queuerpc.Handlers{
		UnaryHandler: func(ctx context.Context, msg *queuerpc.Message) *queuerpc.Message {
			return msg
		},
		ServerStreamHandler: nil,
	})
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Fatal(err)
	}
}
