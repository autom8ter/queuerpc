package nats

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/autom8ter/queuerpc"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// Client is a client for making rpc requests through nats
type Client struct {
	conn         *nats.Conn
	outbox       string
	inbox        string
	errorHandler func(msg string, err error)
	onRequest    func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)
	onResponse   func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)
}

// NewClient creates a new client for making rpc requests.
func NewClient(url string, service string, opts ...ClientOption) (*Client, error) {
	if url == "" {
		url = nats.DefaultURL
	}
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	o := &clientOpts{}
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
	inbox := fmt.Sprintf("rpc.%s.client", service)
	outbox := fmt.Sprintf("rpc.%s.server", service)
	return &Client{
		conn:         conn,
		outbox:       outbox,
		inbox:        inbox,
		errorHandler: o.errorHandler,
		onRequest:    o.onRequest,
		onResponse:   o.onResponse,
	}, nil
}

// Request makes a request to the server and returns the response
func (c Client) Request(ctx context.Context, request *queuerpc.Message, opts ...queuerpc.RequestOption) (*queuerpc.Message, error) {
	id := uuid.NewString()
	request.Id = id
	request.Timestamp = time.Now().UnixMilli()
	request.ReplyTo = c.inbox
	request.Type = queuerpc.Type_UNARY
	requestOpt := &queuerpc.RequestOpts{
		Timeout: 60 * time.Second,
	}
	for _, opt := range opts {
		opt(requestOpt)
	}
	var err error
	if c.onRequest != nil {
		request, err = c.onRequest(ctx, request)
		if err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, requestOpt.Timeout)
	defer cancel()
	bits, err := proto.Marshal(request)
	if err != nil {
		return nil, err
	}
	resp, err := c.conn.RequestWithContext(ctx, c.outbox, bits)
	if err != nil {
		return nil, err
	}
	response := &queuerpc.Message{}
	if err := proto.Unmarshal(resp.Data, response); err != nil {
		return nil, err
	}
	if c.onResponse != nil {
		response, err = c.onResponse(ctx, response)
		if err != nil {
			return nil, err
		}
	}
	return response, nil
}

func (c Client) ServerStream(ctx context.Context, request *queuerpc.Message, fn func(*queuerpc.Message)) error {
	id := uuid.NewString()
	request.Id = id
	request.Timestamp = time.Now().UnixMilli()
	request.ReplyTo = c.inbox
	request.Type = queuerpc.Type_SERVER_STREAM
	bits, err := proto.Marshal(request)
	if err != nil {
		return err
	}
	sub, err := c.conn.QueueSubscribe(c.inbox, request.Id, func(msg *nats.Msg) {
		msg.Ack()
		response := &queuerpc.Message{}
		if err := proto.Unmarshal(msg.Data, response); err != nil {
			c.errorHandler("failed to unmarshal response", err)
			return
		}
		fn(response)
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	if err := c.conn.PublishRequest(c.outbox, c.inbox, bits); err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}

func (c Client) Close() error {
	c.conn.Close()
	return nil
}
