package cnc

import (
	"context"
	"sync"

	"github.com/valkey-io/valkey-go"
)

// CNC is the main interface for command and control
type CNC interface {
	RegisterHandler(commandName CommandName, handler Handler) error
	TriggerCommand(ctx context.Context, command Command) error
	Start(ctx context.Context) error
	Shutdown() error
	IsRunning() bool
}

type cncImpl struct {
	registry  Registry
	transport Transport
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	started   bool
	mu        sync.RWMutex
	options   Options
}

// RegisterHandler registers a command handler with the registry
func (c *cncImpl) RegisterHandler(commandName CommandName, handler Handler) error {
	return c.registry.Register(commandName, handler)
}

// TriggerCommand publishes a command through the transport
func (c *cncImpl) TriggerCommand(ctx context.Context, command Command) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.started {
		return ErrCNCNotStarted
	}

	return c.transport.Publish(ctx, command)
}

// Start begins listening for commands from the transport
func (c *cncImpl) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return ErrCNCAlreadyStarted
	}

	if !c.transport.IsConnected() {
		return ErrTransportNotConnected
	}

	// Subscribe to the transport
	if err := c.transport.Subscribe(ctx); err != nil {
		return err
	}

	// Start message processing goroutine
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.processMessages()
	}()

	c.started = true
	return nil
}

// processMessages continuously processes incoming messages from transport
func (c *cncImpl) processMessages() {
	msgChan := c.transport.Messages()

	for {
		select {
		case command, ok := <-msgChan:
			if !ok {
				// Channel closed, stop processing
				return
			}

			// Execute the command using the registry in a separate goroutine
			// to avoid blocking message processing
			go func(cmd Command) {
				if err := c.registry.Execute(c.ctx, cmd); err != nil {
					if c.options.OnError != nil {
						c.options.OnError(c.ctx, cmd, err)
					}
				}
			}(command)

		case <-c.ctx.Done():
			// Context cancelled, stop processing
			return
		}
	}
}

// IsRunning returns true if the CNC instance is currently running
func (c *cncImpl) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.started
}

// Shutdown gracefully shuts down the CNC instance
func (c *cncImpl) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return nil
	}

	// Cancel context to stop all operations
	c.cancel()

	// Close the transport
	if err := c.transport.Close(); err != nil {
		return err
	}

	// Wait for all goroutines to finish
	c.wg.Wait()

	c.started = false
	return nil
}

// NewCNC creates a new CNC instance with the provided transport
func NewCNC(transport Transport, opts ...Option) CNC {
	ctx, cancel := context.WithCancel(context.Background())

	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	return &cncImpl{
		registry:  NewRegistry(),
		transport: transport,
		ctx:       ctx,
		cancel:    cancel,
		options:   options,
	}
}

// NewCNCWithValkey creates a new CNC instance with a Valkey transport
func NewCNCWithValkey(client valkey.Client, channel string, opts ...Option) CNC {
	transport := NewValkeyTransport(client, channel, opts...)
	return NewCNC(transport, opts...)
}

// NewCNCWithValkeyAddress creates a new CNC instance with a Valkey transport using an address
func NewCNCWithValkeyAddress(address, channel string, clientOpts []valkey.ClientOption, opts ...Option) (CNC, error) {
	client, err := NewValkeyClient(address, clientOpts...)
	if err != nil {
		return nil, err
	}

	transport := NewValkeyTransport(client, channel, opts...)
	return NewCNC(transport, opts...), nil
}
