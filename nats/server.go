package nats

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/autom8ter/queuerpc"
	"github.com/golang/protobuf/proto"
	"github.com/nats-io/nats.go"
)

// Server is a server for handling rpc requests through nats
type Server struct {
	conn         *nats.Conn
	inbox        string
	errorHandler func(msg string, err error)
	onRequest    func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)
	onResponse   func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewServer creates a new server for handling rpc requests.
func NewServer(url string, service string, opts ...ServerOption) (*Server, error) {
	if url == "" {
		url = nats.DefaultURL
	}
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
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		conn:         conn,
		inbox:        fmt.Sprintf("rpc.%s.server", service),
		ctx:          ctx,
		cancel:       cancel,
		errorHandler: o.errorHandler,
		onRequest:    o.onRequest,
		onResponse:   o.onResponse,
	}, nil
}

// Serve starts the server and listens for requests
func (s Server) Serve(handler queuerpc.Handlers) error {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	fmt.Println(s.inbox)
	sub, err := s.conn.Subscribe(s.inbox, func(msg *nats.Msg) {
		msg.Ack()
		if msg.Data == nil {
			s.errorHandler("", queuerpc.ErrEmptyMessageReceived)
			return
		}
		var (
			qmsg = &queuerpc.Message{}
		)
		if err := proto.Unmarshal(msg.Data, qmsg); err != nil {
			s.errorHandler(err.Error(), queuerpc.ErrUnmarshal)
			return
		}

		switch qmsg.Type {
		case queuerpc.Type_UNARY:
			if err := s.handleUnary(ctx, msg, qmsg, handler); err != nil {
				s.errorHandler("error handling unary request", err)
				return
			}
		case queuerpc.Type_SERVER_STREAM:
			if err := s.handleServerStream(ctx, qmsg, handler); err != nil {
				s.errorHandler("error handling stream request", err)
				return
			}
		}
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	<-ctx.Done()
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
			if err := s.conn.Publish(msg.ReplyTo, bits); err != nil {
				s.errorHandler(err.Error(), queuerpc.ErrPublishMessage)
			}
		}
	}
}

func (s *Server) handleUnary(ctx context.Context, nmsg *nats.Msg, msg *queuerpc.Message, handlers queuerpc.Handlers) error {
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
	return nmsg.Respond(bits)
}

// Close closes the server
func (s Server) Close() error {
	s.cancel()
	s.conn.Close()
	return nil
}
