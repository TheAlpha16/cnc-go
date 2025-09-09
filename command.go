package cnc

import "context"

type CommandName string

// Command represents a distributed command
type Command struct {
	Name       CommandName    `json:"type"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

// Handler defines the function signature for processing a command
type Handler func(ctx context.Context, command Command) error
