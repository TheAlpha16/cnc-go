package cnc

import "errors"

var (
	ErrInvalidCommand        = errors.New("invalid command")
	ErrHandlerAlreadyExists  = errors.New("handler already exists")
	ErrHandlerNotFound       = errors.New("handler not found")
	ErrPublishFailed         = errors.New("failed to publish command")
	ErrSubscribeFailed       = errors.New("failed to subscribe to channel")
	ErrTransportNotConnected = errors.New("transport not connected")
	ErrCNCNotStarted         = errors.New("cnc instance not started")
	ErrCNCAlreadyStarted     = errors.New("cnc instance already started")
)
