# cnc-go
A lightweight Go package for distributed command execution and coordination across services

---

> [!note]
> This version is written by Claude after explaining the requirements and providing some code that I used somewhere else
> I haven't tested it extensively, please use with caution
> I will update once I get the chance to test it thoroughly


## What it does

`cnc-go` lets you:

- ⚡ Register commands with custom handlers

- 📡 Trigger commands locally **or across a cluster**

- 🔌 Propagate events using pluggable Pub/Sub backends (Redis/Valkey included, extendable to NATS, Kafka, etc.)

- 🗄️ Ensure synchronized actions across pods/nodes

- 🛠️ Gracefully start, run, and shutdown CNC workers

---


## 🤔 Why?

In distributed systems, some actions — like invalidating caches, reloading configs, or resetting feature flags — must be executed **everywhere**.

Instead of building ad-hoc RPCs or reinventing messaging, **cnc-go provides a simple abstraction**:
publish a command once and have every node execute it reliably.

---

## 🚀 Features

- 🔌 **Pluggable Pub/Sub transport** (Redis included, extendable to NATS/Kafka/etc.)

- 🧩 **Simple API** for registering and triggering commands

- 🛡️ **Typed errors** for safe error handling (`ErrHandlerNotFound`, `ErrHandlerAlreadyExists`, …)

- 📦 **Clean, idiomatic Go design** (no external dependencies beyond the transport driver)

- ✅ Unit tested & extensible

---

## 📦 Installation

```bash
go get github.com/TheAlpha16/cnc-go
```

---

## 🚀 Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/TheAlpha16/cnc-go"
	"github.com/valkey-io/valkey-go"
)

func main() {
	// Create CNC instance with Valkey transport
	cncInstance, err := cnc.NewCNCWithValkeyAddress("localhost:6379", "my-commands", []valkey.ClientOption{})
	if err != nil {
		log.Fatalf("Failed to create CNC: %v", err)
	}
	defer cncInstance.Shutdown()

	// Register a simple command handler
	err = cncInstance.RegisterHandler("hello", func(ctx context.Context, cmd cnc.Command) error {
		name := "World"
		if n, ok := cmd.Parameters["name"].(string); ok {
			name = n
		}
		fmt.Printf("Hello, %s!\n", name)
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to register handler: %v", err)
	}

	// Start the CNC instance
	ctx := context.Background()
	if err := cncInstance.Start(ctx); err != nil {
		log.Fatalf("Failed to start CNC: %v", err)
	}

	fmt.Println("CNC started! Transport is now listening for commands...")

	// Trigger a command after a short delay
	time.Sleep(1 * time.Second)

	command := cnc.Command{
		Name: "hello",
		Parameters: map[string]any{
			"name": "CNC User",
		},
	}

	if err := cncInstance.TriggerCommand(ctx, command); err != nil {
		log.Printf("Failed to trigger command: %v", err)
	}

	// Keep the program running for a bit to see the result
	time.Sleep(3 * time.Second)
	fmt.Println("Quick start example completed!")
}
```

---

## 📖 API Reference

### Core Interfaces

#### CNC Interface
The main interface for command and control operations:

```go
type CNC interface {
    RegisterHandler(commandName CommandName, handler Handler) error
    TriggerCommand(ctx context.Context, command Command) error
    Start(ctx context.Context) error
    Shutdown() error
    IsRunning() bool
}
```

#### Transport Interface
Pluggable transport layer for message passing:

```go
type Transport interface {
    Publish(ctx context.Context, command Command) error
    Subscribe(ctx context.Context) error
    Messages() <-chan Command  // Returns channel of received commands
    Close() error
    IsConnected() bool
}
```

### Creating CNC Instances

#### With Valkey Client
```go
client, err := cnc.NewValkeyClient("localhost:6379")
if err != nil {
    log.Fatal(err)
}

cncInstance := cnc.NewCNCWithValkey(client, "command-channel")
```

#### With Valkey Address (Recommended)
```go
cncInstance, err := cnc.NewCNCWithValkeyAddress("localhost:6379", "command-channel")
if err != nil {
    log.Fatal(err)
}
```

#### With Custom Transport
```go
transport := NewMyCustomTransport()
cncInstance := cnc.NewCNC(transport)
```

### Command Structure

```go
type Command struct {
    Name       CommandName    `json:"type"`
    Parameters map[string]any `json:"parameters,omitempty"`
}

type Handler func(ctx context.Context, command Command) error
```

### Error Handling

The package provides typed errors for better error handling:

```go
var (
    ErrInvalidCommand        = errors.New("invalid command")
    ErrHandlerAlreadyExists  = errors.New("handler already exists")
    ErrHandlerNotFound       = errors.New("handler not found")
    ErrPublishFailed         = errors.New("failed to publish command")
    ErrSubscribeFailed       = errors.New("failed to subscribe to channel")
    ErrTransportNotConnected = errors.New("transport not connected")
    ErrTransportClosed       = errors.New("transport is closed")
    ErrCNCNotStarted         = errors.New("cnc instance not started")
    ErrCNCAlreadyStarted     = errors.New("cnc instance already started")
)
```

---

## 🔌 Implementing Custom Transports

To implement a custom transport (e.g., for NATS, Kafka), implement the `Transport` interface:

```go
type MyCustomTransport struct {
    msgChan chan Command
    // Your other transport fields
}

func (t *MyCustomTransport) Publish(ctx context.Context, command Command) error {
    // Implement publishing logic
    return nil
}

func (t *MyCustomTransport) Subscribe(ctx context.Context) error {
    // Start listening for messages and feed them to msgChan
    return nil
}

func (t *MyCustomTransport) Messages() <-chan Command {
    // Return the channel that receives commands
    return t.msgChan
}

func (t *MyCustomTransport) Close() error {
    // Implement cleanup logic
    close(t.msgChan)
    return nil
}

func (t *MyCustomTransport) IsConnected() bool {
    // Return connection status
    return true
}
```

---

## 🧪 Testing

Run the tests:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

---

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🏗️ Architecture

The package follows a clean separation of concerns:

- **Transport Layer**: Pure message passing (publish/subscribe) with channels
- **Registry**: Maps command names to handler functions
- **CNC Core**: Orchestrates transport and registry, manages lifecycle

```
Commands ──┐
           ▼
    ┌─────────────┐    ┌──────────────┐    ┌─────────────┐
    │  Transport  │───▶│  Channel     │───▶│  CNC Core   │
    │             │    │ <-chan Cmd   │    │             │
    └─────────────┘    └──────────────┘    └─────────────┘
                                                   │
                                                   ▼
                                           ┌─────────────┐
                                           │  Registry   │
                                           │  (Handlers) │
                                           └─────────────┘
```

## 🙋 Support

If you have any questions or need help:

- 📧 Open an issue on GitHub
- 💬 Start a discussion in the repository
- ⭐ Star the repo if you find it useful!
