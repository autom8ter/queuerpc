package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/autom8ter/machine/v4"
	"github.com/autom8ter/queuerpc"
	"github.com/golang/protobuf/proto"
	"github.com/rabbitmq/amqp091-go"
)

var _ queuerpc.IServer = &Server{}

// Server is a server that handles rpc requests
type Server struct {
	session      *session
	inbox        string
	machine      machine.Machine
	errorHandler func(msg string, err error)
	onRequest    func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)
	onResponse   func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)
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
	s := &Server{
		session:      newSession(url, o.tls),
		inbox:        inbox,
		machine:      machine.New(),
		errorHandler: o.errorHandler,
		onRequest:    o.onRequest,
		onResponse:   o.onResponse,
	}
	return s, s.session.Connect(inbox)
}

// Serve starts the server. This is a blocking call. It will return an error when the context is canceled.
func (s *Server) Serve(handler queuerpc.Handlers) error {
	s.machine.Go(s.session.ctx, func(ctx context.Context) error {
		return s.session.Consume(func(delivery amqp091.Delivery) {
			if delivery.Body == nil {
				s.errorHandler("", ErrEmptyMessageReceived)
				return
			}
			s.machine.Go(ctx, func(ctx context.Context) error {
				var (
					msg = &queuerpc.Message{}
				)
				if err := proto.Unmarshal(delivery.Body, msg); err != nil {
					s.errorHandler(err.Error(), queuerpc.ErrUnmarshal)
					return nil
				}
				switch msg.Type {
				case queuerpc.Type_UNARY:
					if err := s.handleUnary(ctx, msg, handler); err != nil {
						s.errorHandler("error handling unary request", err)
						return nil
					}
				case queuerpc.Type_CLIENT_STREAM:
					if err := s.handleClientStream(ctx, msg, handler); err != nil {
						s.errorHandler("error handling client stream request", err)
						return nil
					}
				case queuerpc.Type_SERVER_STREAM:
					if err := s.handleServerStream(ctx, msg, handler); err != nil {
						s.errorHandler("error handling stream request", err)
						return nil
					}
				}
				return nil
			})
		})
	})
	return s.machine.Wait()
}

func (s *Server) handleClientStream(ctx context.Context, msg *queuerpc.Message, handlers queuerpc.Handlers) error {
	if handlers.ClientStreamHandler == nil {
		return fmt.Errorf("no client stream handler")
	}
	var err error
	if s.onRequest != nil {
		msg, err = s.onRequest(ctx, msg)
		if err != nil {
			s.errorHandler("error executing onRequest", err)
			return nil
		}
	}
	if err := handlers.ClientStreamHandler(ctx, msg); err != nil {
		s.errorHandler("error executing client stream handler", err)
		return nil
	}
	return nil
}

func (s *Server) handleServerStream(ctx context.Context, msg *queuerpc.Message, handlers queuerpc.Handlers) error {
	if handlers.ServerStreamHandler == nil {
		return fmt.Errorf("no server server stream handler")
	}
	var err error
	if s.onRequest != nil {
		msg, err = s.onRequest(ctx, msg)
		if err != nil {
			s.errorHandler("error executing onRequest", err)
			return nil
		}
	}
	ch, err := handlers.ServerStreamHandler(ctx, msg)
	if err != nil {
		s.errorHandler("error executing server stream handler", err)
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case resp, ok := <-ch:
			if !ok {
				return nil
			}
			resp.Id = msg.Id
			resp.Method = msg.Method
			resp.Type = msg.Type
			resp.Timestamp = time.Now().UnixMilli()
			if s.onResponse != nil {
				resp, err = s.onResponse(ctx, resp)
				if err != nil {
					s.errorHandler("error executing onResponse", err)
					return nil
				}
			}
			bits, err := proto.Marshal(resp)
			if err != nil {
				s.errorHandler(err.Error(), queuerpc.ErrMarshal)
				return nil
			}
			if err := s.session.Publish(ctx, amqp091.Publishing{
				Body: bits,
			}, msg.ReplyTo); err != nil {
				s.errorHandler(err.Error(), ErrPublishMessage)
			}
		}
	}
}

func (s *Server) handleUnary(ctx context.Context, msg *queuerpc.Message, handlers queuerpc.Handlers) error {
	if handlers.UnaryHandler == nil {
		return fmt.Errorf("no server unary handler")
	}
	var err error
	if s.onRequest != nil {
		msg, err = s.onRequest(ctx, msg)
		if err != nil {
			s.errorHandler("error executing onRequest", err)
			return nil
		}
	}
	resp := handlers.UnaryHandler(ctx, msg)
	resp.Id = msg.Id
	resp.Method = msg.Method
	resp.Type = msg.Type
	resp.Timestamp = time.Now().UnixMilli()
	if s.onResponse != nil {
		resp, err = s.onResponse(ctx, resp)
		if err != nil {
			s.errorHandler("error executing onResponse", err)
			return nil
		}
	}
	bits, err := proto.Marshal(resp)
	if err != nil {
		s.errorHandler(err.Error(), queuerpc.ErrMarshal)
		return nil
	}
	if err := s.session.Publish(ctx, amqp091.Publishing{
		CorrelationId: msg.Id,
		Body:          bits,
	}, msg.ReplyTo); err != nil {
		s.errorHandler(err.Error(), ErrPublishMessage)
	}
	return nil
}

func (s *Server) Close() error {
	return s.session.Close()
}
