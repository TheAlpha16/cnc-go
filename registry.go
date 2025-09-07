package cnc

import (
	"context"
	"sync"
)

type Registry interface {
	Register(name CommandName, h Handler) error
	Execute(ctx context.Context, cmd Command) error
}

type registryImpl struct {
	handlers map[CommandName]Handler
	mu       sync.RWMutex
}

func (r *registryImpl) Register(name CommandName, h Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[name]; exists {
		return ErrHandlerAlreadyExists
	}
	r.handlers[name] = h
	return nil
}

func (r *registryImpl) Execute(ctx context.Context, cmd Command) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[cmd.Name]
	if !ok {
		return ErrHandlerNotFound
	}
	return h(ctx, cmd)
}

func NewRegistry() Registry {
	return &registryImpl{handlers: make(map[CommandName]Handler)}
}
