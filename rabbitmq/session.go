package rabbitmq

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/autom8ter/machine/v4"
	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
)

type session struct {
	url        string
	cfg        *tls.Config
	conn       atomic.Pointer[amqp091.Connection]
	channel    atomic.Pointer[amqp091.Channel]
	deliveries atomic.Pointer[<-chan amqp091.Delivery]
	machine    machine.Machine
	ctx        context.Context
	cancel     func()
}

func newSession(url string, cfg *tls.Config) *session {
	ctx, cancel := context.WithCancel(context.Background())
	return &session{
		url:     url,
		cfg:     cfg,
		machine: machine.New(),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (s *session) Connect(queue string, client bool) error {
	var hasConnected bool
	for i := 0; i < 60; i++ {
		if client {
			if err := s.connectClient(queue); err != nil {
				time.Sleep(1 * time.Second)
				continue
			}
			hasConnected = true
			break
		} else {
			if err := s.connectServer(queue); err != nil {
				time.Sleep(1 * time.Second)
				continue
			}
			hasConnected = true
			break
		}

	}
	if !hasConnected {
		return fmt.Errorf("failed to connect to rabbitmq: %s", s.url)
	}
	return nil
}

func (s *session) connectServer(queue string) error {
	var (
		conn *amqp091.Connection
		err  error
	)
	if s.cfg == nil {
		conn, err = amqp091.Dial(s.url)
		if err != nil {
			return fmt.Errorf("failed to dial: %s", s.url)
		}
	} else {
		conn, err = amqp091.DialTLS(s.url, s.cfg)
		if err != nil {
			return fmt.Errorf("failed to dial tls: %s", s.url)
		}
	}
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %s", s.url)
	}
	if err := ch.ExchangeDeclare(
		queue,
		"direct",
		false,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare exchange: %s", err.Error())
	}
	id := uuid.NewString()
	id = "server" + "-" + id[0:8]
	_, err = ch.QueueDeclare(
		id,
		false, // durable
		true,  // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %s", err.Error())
	}
	if err := ch.QueueBind(
		id,
		"",
		queue,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind queue: %s", err.Error())
	}
	deliveries, err := ch.Consume(
		queue,
		"",
		true,  // auto-ack
		true,  // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to consume: %w", err)
	}
	s.conn.Store(conn)
	s.channel.Store(ch)
	s.deliveries.Store(&deliveries)
	go func() {
		<-conn.NotifyClose(make(chan *amqp091.Error)) //Listen to NotifyClose
		s.Connect(queue, false)
	}()
	return nil
}

func (s *session) connectClient(queue string) error {
	var (
		conn *amqp091.Connection
		err  error
	)
	if s.cfg == nil {
		conn, err = amqp091.Dial(s.url)
		if err != nil {
			return fmt.Errorf("failed to dial: %s", s.url)
		}
	} else {
		conn, err = amqp091.DialTLS(s.url, s.cfg)
		if err != nil {
			return fmt.Errorf("failed to dial tls: %s", s.url)
		}
	}
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %s", s.url)
	}
	if err := ch.ExchangeDeclare(
		queue,
		"fanout",
		false,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare exchange: %s", err.Error())
	}
	id := uuid.NewString()
	id = "client" + "-" + id[0:8]
	_, err = ch.QueueDeclare(
		id,
		false, // durable
		true,  // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %s", err.Error())
	}
	if err := ch.QueueBind(
		id,
		"",
		queue,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind queue: %s", err.Error())
	}
	deliveries, err := ch.Consume(
		queue,
		"",
		true,  // auto-ack
		true,  // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to consume: %w", err)
	}
	s.conn.Store(conn)
	s.channel.Store(ch)
	s.deliveries.Store(&deliveries)
	go func() {
		<-conn.NotifyClose(make(chan *amqp091.Error)) //Listen to NotifyClose
		s.Connect(queue, true)
	}()
	return nil
}

func (s *session) Consume(fn func(delivery amqp091.Delivery)) error {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			deliveries := s.deliveries.Load()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case d, ok := <-*deliveries:
				if !ok {
					continue
				}
				s.machine.Go(ctx, func(ctx context.Context) error {
					fn(d)
					return nil
				})
			}
		}
	}
}

func (s *session) Publish(ctx context.Context, p amqp091.Publishing, queue string) error {
	if err := s.channel.Load().PublishWithContext(ctx, "", queue, false, false, p); err != nil {
		return fmt.Errorf("error in Publishing: %s", err)
	}
	return nil
}

func (s *session) Close() error {
	s.cancel()
	if err := s.machine.Wait(); err != nil {
		return err
	}
	return nil
}
