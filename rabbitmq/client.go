package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/autom8ter/queuerpc"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
)

var _ queuerpc.IClient = &Client{}

// Client is a client for making rpc requests through rabbitmq
type Client struct {
	session      *session
	mu           sync.RWMutex
	awaiting     map[string]chan *queuerpc.Message
	outbox       string
	inbox        string
	errorHandler func(msg string, err error)
	onRequest    func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)
	onResponse   func(ctx context.Context, msg *queuerpc.Message) (*queuerpc.Message, error)
}

// NewClient creates a new client for making rpc requests.
// The client will automatically connect to the rabbitmq server and begin processing messages
// The service name should be the same for the server and client
func NewClient(url string, service string, opts ...ClientOption) (*Client, error) {
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
	c := &Client{
		session:      newSession(url, o.tls, inbox),
		mu:           sync.RWMutex{},
		awaiting:     map[string]chan *queuerpc.Message{},
		inbox:        inbox,
		outbox:       outbox,
		errorHandler: o.errorHandler,
		onRequest:    o.onRequest,
		onResponse:   o.onResponse,
	}
	if err := c.session.Connect(); err != nil {
		return nil, err
	}
	c.connect()
	return c, nil
}

func (c *Client) connect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.session.machine.Go(c.session.ctx, func(ctx context.Context) error {
		return c.session.Consume(func(delivery amqp091.Delivery) {
			if delivery.Body == nil {
				c.errorHandler("", ErrEmptyMessageReceived)
				return
			}
			var m queuerpc.Message
			if err := proto.Unmarshal(delivery.Body, &m); err != nil {
				c.errorHandler(err.Error(), queuerpc.ErrUnmarshal)
				return
			}
			c.mu.RLock()
			if ch, ok := c.awaiting[m.GetId()]; ok {
				c.session.machine.Go(ctx, func(ctx context.Context) error {
					ch <- &m
					return nil
				})
			}
			c.mu.RUnlock()
		})
	})
}

// Request sends a request to the server and returns the response and an error if one was encountered
func (c *Client) Request(ctx context.Context, body *queuerpc.Message, opts ...queuerpc.RequestOption) (*queuerpc.Message, error) {
	requestOpt := &queuerpc.RequestOpts{
		Timeout: 60 * time.Second,
	}
	for _, opt := range opts {
		opt(requestOpt)
	}
	var err error
	if c.onRequest != nil {
		body, err = c.onRequest(ctx, body)
		if err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, requestOpt.Timeout)
	defer cancel()
	ch := make(chan *queuerpc.Message, 1)
	c.mu.Lock()
	if c.awaiting == nil {
		c.awaiting = make(map[string]chan *queuerpc.Message)
	}
	id := uuid.NewString()
	body.Id = id
	c.awaiting[id] = ch
	c.mu.Unlock()
	bits, err := proto.Marshal(body)
	if err != nil {
		return nil, err
	}
	pubMsg := amqp091.Publishing{
		CorrelationId: body.Id,
		Body:          bits,
		ReplyTo:       c.inbox,
		Expiration:    fmt.Sprintf("%d", requestOpt.Timeout.Milliseconds()),
	}
	if err := c.session.Publish(ctx, pubMsg, c.outbox); err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case msg := <-ch:
			c.mu.Lock()
			delete(c.awaiting, id)
			c.mu.Unlock()
			if c.onResponse != nil {
				msg, err = c.onResponse(ctx, msg)
				if err != nil {
					return msg, err
				}
			}
			return msg, nil
		}
	}
}

// Close closes the client and waits for all goroutines to exit
func (c *Client) Close() error {
	return c.session.Close()
}
