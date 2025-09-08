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
