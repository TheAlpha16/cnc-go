package cnc

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/valkey-io/valkey-go"
)

type ValkeyTransport struct {
	client       valkey.Client
	channel      string
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.RWMutex
	isSubscribed bool
	connected    bool
	msgChan      chan Command
	closedChan   chan struct{}
	once         sync.Once
	options      Options
}

// Publish publishes a command to the valkey channel
func (v *ValkeyTransport) Publish(ctx context.Context, command Command) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.connected {
		return ErrTransportNotConnected
	}

	data, err := json.Marshal(command)
	if err != nil {
		return ErrInvalidCommand
	}

	cmd := v.client.B().Publish().Channel(v.channel).Message(string(data)).Build()
	err = v.client.Do(ctx, cmd).Error()
	if err != nil {
		return ErrPublishFailed
	}

	return nil
}

// Subscribe starts subscribing to the valkey channel
func (v *ValkeyTransport) Subscribe(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.isSubscribed {
		return nil // Already subscribed
	}

	if !v.connected {
		return ErrTransportNotConnected
	}

	// Start the subscription goroutine
	go v.subscriptionLoop()

	v.isSubscribed = true
	return nil
}

// subscriptionLoop handles the actual subscription and message processing
func (v *ValkeyTransport) subscriptionLoop() {
	defer func() {
		v.mu.Lock()
		v.isSubscribed = false
		close(v.msgChan)
		v.mu.Unlock()
	}()

	retryDelay := 100 * time.Millisecond
	maxRetryDelay := 30 * time.Second
	subscriber := v.client.B().Subscribe().Channel(v.channel).Build()

	for {
		if v.shouldStop() {
			return
		}

		// Create subscription and start receiving messages
		// This call will block until an error occurs or context is cancelled
		err := v.client.Receive(v.ctx, subscriber, v.handleMessage)

		if err != nil {
			// Check if we're shutting down
			if v.shouldStop() {
				return
			}

			// If Receive returns an error, we retry after a delay
			// This handles network disconnections, server restarts, etc.
			// retry with backoff
			time.Sleep(retryDelay)
			retryDelay *= 2
			if retryDelay > maxRetryDelay {
				retryDelay = maxRetryDelay
			}
			continue
		}

		if v.shouldStop() {
			return
		}

		// If we get here, v.client.Receive returned without error
		// Reset retry delay on successful connection
		retryDelay = 100 * time.Millisecond

		// Brief pause before reconnecting
		time.Sleep(100 * time.Millisecond)
	}
}

// handleMessage processes individual messages from the subscription
func (v *ValkeyTransport) handleMessage(msg valkey.PubSubMessage) {
	// Only process messages from our channel
	if msg.Channel != v.channel {
		return // This only exits the callback, not the subscription loop
	}

	// Parse the command
	var command Command
	if err := json.Unmarshal([]byte(msg.Message), &command); err != nil {
		if v.options.OnError != nil {
			v.options.OnError(v.ctx, Command{}, err)
		}
		return
	}

	// Send to message channel (non-blocking to prevent deadlock)
	select {
	case v.msgChan <- command:
		// Successfully sent message
	case <-v.closedChan:
		return
	case <-v.ctx.Done():
		return
	default:
		// Channel full, drop message to prevent blocking
		// In production, you might want to log this or implement backpressure
	}
}

// Messages returns a channel that receives commands from the transport
func (v *ValkeyTransport) Messages() <-chan Command {
	return v.msgChan
}

// Close shuts down the valkey transport and cleans up resources
func (v *ValkeyTransport) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.connected {
		return nil
	}

	// Use sync.Once to ensure cleanup happens only once
	v.once.Do(func() {
		// Signal close to all goroutines
		close(v.closedChan)

		// Cancel context to stop all operations
		v.cancel()

		// Close the client connection
		v.client.Close()
		v.connected = false
	})

	return nil
}

// IsConnected returns true if the transport is connected and ready
func (v *ValkeyTransport) IsConnected() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.connected
}

func (v *ValkeyTransport) shouldStop() bool {
	select {
	case <-v.closedChan:
		return true
	case <-v.ctx.Done():
		return true
	default:
		return false
	}
}

// NewValkeyClient creates a new valkey client with common configuration
func NewValkeyClient(address string, options ...valkey.ClientOption) (valkey.Client, error) {
	defaultOption := valkey.ClientOption{
		InitAddress: []string{address},
	}

	// Use provided options or default
	var clientOption valkey.ClientOption
	if len(options) > 0 {
		clientOption = options[0]
		if len(clientOption.InitAddress) == 0 {
			clientOption.InitAddress = []string{address}
		}
	} else {
		clientOption = defaultOption
	}

	client, err := valkey.NewClient(clientOption)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// NewValkeyTransport creates a new valkey transport instance
func NewValkeyTransport(client valkey.Client, channel string, opts ...Option) Transport {
	ctx, cancel := context.WithCancel(context.Background())

	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	return &ValkeyTransport{
		client:     client,
		channel:    channel,
		ctx:        ctx,
		cancel:     cancel,
		connected:  true,
		msgChan:    make(chan Command, options.MsgBufferSize), // Buffered channel for message queue
		closedChan: make(chan struct{}),
		options:    options,
	}
}
