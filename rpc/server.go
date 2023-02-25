package rpc

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	"github.com/autom8ter/machine/v4"
	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
)

type HandlerFunc func(ctx context.Context, msg Message) Message

type Server struct {
	session func(ctx context.Context) (*session, error)
	outbox  string
	inbox   string
	machine machine.Machine
}

// NewServer creates a new server
func NewServer(url string, service string) (*Server, error) {
	inbox := fmt.Sprintf("rpc.%s.server", service)
	outbox := fmt.Sprintf("rpc.%s.client", service)
	conn, err := amqp091.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	if _, err := ch.QueueDeclare(
		inbox,
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	); err != nil {
		return nil, err
	}
	return &Server{
		machine: machine.New(),
		inbox:   inbox,
		outbox:  outbox,
		session: func(ctx context.Context) (*session, error) {
			return &session{
				conn:    conn,
				channel: ch,
			}, nil
		},
	}, nil
}

func (s *Server) Serve(ctx context.Context, handler HandlerFunc) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	sess, err := s.session(ctx)
	if err != nil {
		return err
	}
	deliveries, err := sess.channel.Consume(
		s.inbox,
		"",
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to consume: %w", err)
	}
	s.machine.Go(ctx, func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case d := <-deliveries:
				if d.Body == nil {
					// TODO: reconnect
					return fmt.Errorf("empty message received")
				}
				s.machine.Go(ctx, func(ctx context.Context) error {
					var msg Message
					buf := bytes.NewBuffer(d.Body)
					if err := gob.NewDecoder(buf).Decode(&msg); err != nil {
						return err
					}
					resp := handler(ctx, msg)
					buf = bytes.NewBuffer(nil)
					if err := gob.NewEncoder(buf).Encode(resp); err != nil {
						return err
					}
					if err := sess.channel.PublishWithContext(ctx, "", s.outbox, false, false, amqp091.Publishing{
						MessageId:     uuid.NewString(),
						CorrelationId: resp.ID,
						Body:          buf.Bytes(),
					}); err != nil {
						return err
					}
					return nil
				})
			}
		}
	})
	return s.machine.Wait()
}
