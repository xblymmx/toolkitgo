package grpcpool

import (
	"sync"
	"google.golang.org/grpc"
	"time"
	"errors"
	"log"
	"context"
)

var (
	ErrClosed        = errors.New("grpc pool: client pool is closed")
	ErrTimeout       = errors.New("grpc pool: client pool timed out")
	ErrAlreadyClosed = errors.New("grpc pool: the connection was already closed")
	ErrFullPool      = errors.New("grpc pool: closing a ClientConn into a full pool")
)

type Factory func() (*grpc.ClientConn, error)

type Pool struct {
	sync.RWMutex
	clients     chan ClientConn
	factory     Factory
	idleTimeout time.Duration
}

type ClientConn struct {
	*grpc.ClientConn
	pool        *Pool
	timeSpawned time.Time
	unhealthy   bool
}

func New(factory Factory, init, capacity int, timeout time.Duration) (*Pool, error) {
	if init < 0 {
		init = 0
	}
	if capacity < 0 {
		capacity = 1
	}
	if init > capacity {
		init = capacity
	}

	p := &Pool{
		clients:     make(chan ClientConn, capacity),
		factory:     factory,
		idleTimeout: timeout,
	}
	for i := 0; i < init; i++ {
		c, err := factory()
		if err != nil {
			return nil, err
		}

		p.clients <- ClientConn{
			timeSpawned: time.Now(),
			pool:        p,
			ClientConn:  c,
		}

	}

	// fill with empty grpc connection
	for i := init; i < capacity; i++ {
		p.clients <- ClientConn{
			pool: p,
		}
	}

	return p, nil
}

func (p *Pool) getClients() chan ClientConn {
	p.RLock()
	defer p.Unlock()
	return p.clients
}

func (p *Pool) Close() {
	clients := p.getClients()
	p.clients = nil

	if clients == nil {
		return
	}

	close(clients)
	for c := range clients {
		if c.ClientConn == nil {
			continue
		}

		err := c.ClientConn.Close()
		if err != nil {
			log.Println("error on grpc connection close:", err)
		}
	}
}

func (p *Pool) IsClosed() bool {
	return p == nil || p.getClients() == nil
}

func (p *Pool) Get(ctx context.Context) (*ClientConn, error) {
	clients := p.getClients()
	if clients == nil {
		return nil, ErrClosed
	}

	wrapper := ClientConn{
		pool: p,
	}

	select {
	case wrapper = <-clients:
		// continue
	case ctx.Done():
		return nil, ErrTimeout
	}

	if wrapper.ClientConn != nil && p.idleTimeout > 0 &&
		wrapper.timeSpawned.Add(p.idleTimeout).Before(time.Now()) {
		wrapper.ClientConn.Close()
		wrapper.ClientConn = nil
	}

	var err error
	if wrapper.ClientConn == nil {
		wrapper.ClientConn, err = p.factory()
		if err != nil {
			// error happens, put a client in the channel
			clients <- ClientConn{
				pool: p,
			}
		}

	}

	return &wrapper, nil
}

func (c *ClientConn) Unhealthy() {
	c.unhealthy = true
}

func (c *ClientConn) Close() error {
	if c == nil {
		return nil
	}
	if c.ClientConn == nil {
		return ErrAlreadyClosed
	}
	if c.pool.IsClosed() {
		return ErrClosed
	}

	wrapper := ClientConn{
		pool:        c.pool,
		timeSpawned: c.timeSpawned,
		unhealthy:   c.unhealthy,
		ClientConn:  c.ClientConn,
	}
	if c.unhealthy {
		wrapper.ClientConn = nil
	}

	select {
	case c.pool.clients <- wrapper:
		// all good
	default:
		return ErrFullPool
	}

	c.ClientConn = nil // closed
	return nil
}

func (p *Pool) Capacity() int {
	if p.IsClosed() {
		return 0
	}
	return cap(p.clients)
}

func (p *Pool) Available() int {
	if p.IsClosed() {
		return 0
	}
	return len(p.clients)
}
