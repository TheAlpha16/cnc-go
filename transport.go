package cnc

import "context"

// Transport defines the interface for message transport layer
// Focused purely on message passing without handler coupling
type Transport interface {
	// Publish sends a command to the transport layer
	Publish(ctx context.Context, command Command) error

	// Subscribe starts listening for messages on the transport
	Subscribe(ctx context.Context) error

	// Messages returns a channel that receives commands from the transport
	// This channel should be closed when the transport is closed
	Messages() <-chan Command

	// Close shuts down the transport and releases resources
	Close() error

	// IsConnected returns true if the transport is connected and ready
	IsConnected() bool
}
