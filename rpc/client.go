package rpc

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sync"
	"time"

	"github.com/autom8ter/machine/v4"
	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
)

type RequestOpts struct {
	Timeout time.Duration
}

type RequestOption func(*RequestOpts)

func WithTimeout(t time.Duration) RequestOption {
	return func(o *RequestOpts) {
		o.Timeout = t
	}
}

type Client struct {
	session  func(ctx context.Context) (*session, error)
	mu       sync.RWMutex
	awaiting map[string]chan Message
	outbox   string
	inbox    string
	machine  machine.Machine
	ctx      context.Context
	cancel   func()
}

func NewClient(url string, service string) (*Client, error) {
	inbox := fmt.Sprintf("rpc.%s.client", service)
	outbox := fmt.Sprintf("rpc.%s.server", service)
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
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		session: func(ctx context.Context) (*session, error) {
			return &session{
				conn:    conn,
				channel: ch,
			}, nil
		},
		mu:       sync.RWMutex{},
		awaiting: map[string]chan Message{},
		inbox:    inbox,
		outbox:   outbox,
		machine:  machine.New(),
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

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
					return fmt.Errorf("channel closed")
				}
			case msg := <-deliveries:
				if msg.Body == nil {
					// TODO: reconnect
					return fmt.Errorf("empty message received")
				}
				buf := bytes.NewBuffer(msg.Body)
				var m Message
				if err := gob.NewDecoder(buf).Decode(&m); err != nil {
					c.mu.RUnlock()
					return err
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

func (c *Client) Request(ctx context.Context, body Message, opts ...RequestOption) (*Message, error) {
	requestOpt := &RequestOpts{
		Timeout: 60 * time.Second,
	}
	for _, opt := range opts {
		opt(requestOpt)
	}
	ctx, cancel := context.WithTimeout(ctx, requestOpt.Timeout)
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
		Expiration:    fmt.Sprintf("%d", requestOpt.Timeout.Milliseconds()),
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
			return &msg, nil
		}
	}
}

func (c *Client) Close() error {
	c.cancel()
	return c.machine.Wait()
}
