package rabbitmq_test

import (
	"context"
	"testing"

	"github.com/autom8ter/queuerpc/rabbitmq"
)

func TestClient(t *testing.T) {
	client, err := rabbitmq.NewClient("amqp://guest:guest@localhost:5672/", "echo")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Error(err)
	}
}
