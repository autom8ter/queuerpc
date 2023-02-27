package rpc

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/autom8ter/machine/v4"
	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
)

// Client is a client for making rpc requests through rabbitmq
type Client struct {
	session      func(ctx context.Context) (*session, error)
	mu           sync.RWMutex
	awaiting     map[string]chan Message
	outbox       string
	inbox        string
	machine      machine.Machine
	ctx          context.Context
	cancel       func()
	errorHandler func(msg string, err error)
	onRequest    func(ctx context.Context, msg Message) (Message, error)
	onResponse   func(ctx context.Context, msg Message) (Message, error)
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
		return nil, fmt.Errorf("failed to create channel: %s", url)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		session: func(ctx context.Context) (*session, error) {
			return &session{
				conn:    conn,
				channel: ch,
			}, nil
		},
		mu:           sync.RWMutex{},
		awaiting:     map[string]chan Message{},
		inbox:        inbox,
		outbox:       outbox,
		machine:      machine.New(),
		ctx:          ctx,
		cancel:       cancel,
		errorHandler: o.errorHandler,
		onRequest:    o.onRequest,
		onResponse:   o.onResponse,
	}, nil
}

// Connect connects the client to the rabbitmq server and begins asynchronously processing messages
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	c.ctx = ctx
	c.cancel = cancel
	c.machine.Go(c.ctx, func(ctx context.Context) error {
		sess, err := c.session(ctx)
		if err != nil {
			return err
		}
		deliveries, err := sess.channel.Consume(
			c.inbox,
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
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				if sess.channel.IsClosed() {
					// TODO: reconnect
					c.errorHandler("", ErrChannelClosed)
					return nil
				}
			case msg := <-deliveries:
				if msg.Body == nil {
					// TODO: reconnect
					c.errorHandler("", ErrEmptyMessageReceived)
					return nil
				}
				buf := bytes.NewBuffer(msg.Body)
				var m Message
				if err := gob.NewDecoder(buf).Decode(&m); err != nil {
					c.errorHandler(err.Error(), ErrDecodeMessage)
					return nil
				}
				c.mu.RLock()
				if ch, ok := c.awaiting[m.ID]; ok {
					c.machine.Go(ctx, func(ctx context.Context) error {
						ch <- m
						return nil
					})
				}
				c.mu.RUnlock()
			}
		}
	})
	return nil
}

// Request sends a request to the server and returns the response and an error if one was encountered
func (c *Client) Request(ctx context.Context, body Message, opts ...RequestOption) (*Message, error) {
	requestOpt := &requestOpts{
		timeout: 60 * time.Second,
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
	ctx, cancel := context.WithTimeout(ctx, requestOpt.timeout)
	defer cancel()
	sess, err := c.session(ctx)
	if err != nil {
		return nil, err
	}

	ch := make(chan Message, 1)
	c.mu.Lock()
	if c.awaiting == nil {
		c.awaiting = make(map[string]chan Message)
	}
	id := uuid.NewString()
	body.ID = id
	c.awaiting[id] = ch
	c.mu.Unlock()
	buf := bytes.NewBuffer(nil)
	if err := gob.NewEncoder(buf).Encode(body); err != nil {
		return nil, err
	}
	pubMsg := amqp091.Publishing{
		CorrelationId: body.ID,
		Body:          buf.Bytes(),
		ReplyTo:       c.inbox,
		Expiration:    fmt.Sprintf("%d", requestOpt.timeout.Milliseconds()),
	}
	if err := sess.channel.PublishWithContext(ctx, "", c.outbox, false, false, pubMsg); err != nil {
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
					return &msg, err
				}
			}
			return &msg, nil
		}
	}
}

// Close closes the client and waits for all goroutines to exit
func (c *Client) Close() error {
	c.cancel()
	return c.machine.Wait()
}
