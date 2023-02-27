package rpc

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"time"

	"github.com/autom8ter/machine/v4"
	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
)

// HandlerFunc is a function that handles a message.
// The message returned will be sent back to the client
// If an error is encountered, an error should be added to the Message
type HandlerFunc func(ctx context.Context, msg Message) Message

// Server is a server that handles rpc requests
type Server struct {
	session      func(ctx context.Context) (*session, error)
	outbox       string
	inbox        string
	machine      machine.Machine
	errorHandler func(msg string, err error)
	onRequest    func(ctx context.Context, msg Message) (Message, error)
	onResponse   func(ctx context.Context, msg Message) (Message, error)
}

// NewServer creates a new server
// The service name should be the same for the server and client
func NewServer(url string, service string, opts ...ServerOption) (*Server, error) {
	o := &serverOpts{}
	for _, opt := range opts {
		opt(o)
	}
	if o.errorHandler == nil {
		o.errorHandler = func(msg string, err error) {
			if err != nil {
				log.Println(msg, err.Error())
			}
		}
	}
	inbox := fmt.Sprintf("rpc.%s.server", service)
	outbox := fmt.Sprintf("rpc.%s.client", service)
	var (
		conn *amqp091.Connection
		err  error
	)
	if o.tls == nil {
		conn, err = amqp091.Dial(url)
		if err != nil {
			return nil, fmt.Errorf("failed to dial: %s", url)
		}
	} else {
		conn, err = amqp091.DialTLS(url, o.tls)
		if err != nil {
			return nil, fmt.Errorf("failed to dial tls: %s", url)
		}
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open channel: %s", err.Error())
	}
	if _, err := ch.QueueDeclare(
		inbox,
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	); err != nil {
		return nil, fmt.Errorf("failed to declare queue: %s", err.Error())
	}
	return &Server{
		session: func(ctx context.Context) (*session, error) {
			return &session{
				conn:    conn,
				channel: ch,
			}, nil
		},
		outbox:       outbox,
		inbox:        inbox,
		machine:      machine.New(),
		errorHandler: o.errorHandler,
		onRequest:    o.onRequest,
		onResponse:   o.onResponse,
	}, nil
}

// Serve starts the server. This is a blocking call. It will return an error when the context is canceled.
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
			case <-time.After(5 * time.Second):
				if sess.channel.IsClosed() {
					// TODO: reconnect
					s.errorHandler("", ErrChannelClosed)
					return nil
				}
			case d := <-deliveries:
				if d.Body == nil {
					// TODO: reconnect
					s.errorHandler("", ErrEmptyMessageReceived)
					return nil
				}
				s.machine.Go(ctx, func(ctx context.Context) error {
					var msg Message
					buf := bytes.NewBuffer(d.Body)
					if err := gob.NewDecoder(buf).Decode(&msg); err != nil {
						s.errorHandler(err.Error(), ErrDecodeMessage)
						return nil
					}
					if s.onRequest != nil {
						msg, err = s.onRequest(ctx, msg)
						if err != nil {
							s.errorHandler("error executing onRequest", err)
							return nil
						}
					}
					resp := handler(ctx, msg)
					if s.onResponse != nil {
						resp, err = s.onResponse(ctx, resp)
						if err != nil {
							s.errorHandler("error executing onResponse", err)
							return nil
						}
					}
					buf = bytes.NewBuffer(nil)
					if err := gob.NewEncoder(buf).Encode(resp); err != nil {
						s.errorHandler(err.Error(), ErrEncodeMessage)
						return nil
					}
					if err := sess.channel.PublishWithContext(ctx, "", s.outbox, false, false, amqp091.Publishing{
						MessageId:     uuid.NewString(),
						CorrelationId: resp.ID,
						Body:          buf.Bytes(),
					}); err != nil {
						// TODO: reconnect
						s.errorHandler(err.Error(), ErrPublishMessage)
					}
					return nil
				})
			}
		}
	})
	return s.machine.Wait()
}
