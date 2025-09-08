package cnc

import "context"

// ErrorHandler is a user-provided callback for command execution errors
type ErrorHandler func(ctx context.Context, cmd Command, err error)
type Option func(*Options)

type Options struct {
	MsgBufferSize int
	OnError       ErrorHandler
}

func defaultOptions() Options {
	return Options{
		MsgBufferSize: 100,
		OnError: func(ctx context.Context, cmd Command, err error) {
			// Default: no-op
		},
	}
}

func WithMsgBufferSize(size int) Option {
	return func(o *Options) {
		if size > 0 {
			o.MsgBufferSize = size
		}
	}
}

func WithOnError(handler ErrorHandler) Option {
	return func(o *Options) {
		o.OnError = handler
	}
}
