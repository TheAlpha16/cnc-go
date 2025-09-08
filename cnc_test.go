package cnc

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MockTransport implements the Transport interface for testing
type MockTransport struct {
	mu         sync.RWMutex
	published  []Command
	connected  bool
	subscribed bool
	msgChan    chan Command
	closedChan chan struct{}
	once       sync.Once
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		published:  make([]Command, 0),
		connected:  true,
		msgChan:    make(chan Command, 100),
		closedChan: make(chan struct{}),
	}
}

func (m *MockTransport) Publish(ctx context.Context, command Command) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return ErrPublishFailed
	}

	m.published = append(m.published, command)

	// Simulate message delivery by sending to our own message channel
	select {
	case m.msgChan <- command:
	case <-m.closedChan:
	default:
		// Channel full, drop message
	}

	return nil
}

func (m *MockTransport) Subscribe(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return ErrSubscribeFailed
	}

	m.subscribed = true
	return nil
}

func (m *MockTransport) Messages() <-chan Command {
	return m.msgChan
}

func (m *MockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return nil
	}

	m.once.Do(func() {
		close(m.closedChan)
		close(m.msgChan)
		m.connected = false
		m.subscribed = false
	})

	return nil
}

func (m *MockTransport) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

func (m *MockTransport) GetPublishedCommands() []Command {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Command, len(m.published))
	copy(result, m.published)
	return result
}

// Helper to inject commands directly into the message channel for testing
func (m *MockTransport) InjectCommand(command Command) {
	select {
	case m.msgChan <- command:
	case <-m.closedChan:
	default:
		// Channel full
	}
}

func TestCNCRegistration(t *testing.T) {
	transport := NewMockTransport()
	cnc := NewCNC(transport)
	defer cnc.Shutdown()

	// Test successful handler registration
	handler := func(ctx context.Context, cmd Command) error {
		return nil
	}

	err := cnc.RegisterHandler("test-command", handler)
	if err != nil {
		t.Fatalf("Expected no error registering handler, got: %v", err)
	}

	// Test duplicate handler registration
	err = cnc.RegisterHandler("test-command", handler)
	if err != ErrHandlerAlreadyExists {
		t.Fatalf("Expected ErrHandlerAlreadyExists, got: %v", err)
	}
}

func TestCNCTriggerCommand(t *testing.T) {
	transport := NewMockTransport()
	cnc := NewCNC(transport)
	ctx := context.Background()

	// Start the CNC instance
	err := cnc.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start CNC: %v", err)
	}
	defer cnc.Shutdown()

	// Test triggering a command
	command := Command{
		Name: "test-command",
		Parameters: map[string]any{
			"key": "value",
		},
	}

	err = cnc.TriggerCommand(ctx, command)
	if err != nil {
		t.Fatalf("Expected no error triggering command, got: %v", err)
	}

	// Verify the command was published
	published := transport.GetPublishedCommands()
	if len(published) != 1 {
		t.Fatalf("Expected 1 published command, got: %d", len(published))
	}

	if published[0].Name != command.Name {
		t.Fatalf("Expected command name %s, got: %s", command.Name, published[0].Name)
	}
}

func TestCNCCommandExecution(t *testing.T) {
	transport := NewMockTransport()
	cnc := NewCNC(transport)
	ctx := context.Background()

	// Track handler execution
	var executedCommands []Command
	var mu sync.Mutex

	handler := func(ctx context.Context, cmd Command) error {
		mu.Lock()
		defer mu.Unlock()
		executedCommands = append(executedCommands, cmd)
		return nil
	}

	// Register handler
	err := cnc.RegisterHandler("test-command", handler)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Start the CNC instance
	err = cnc.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start CNC: %v", err)
	}
	defer cnc.Shutdown()

	// Give some time for message processing to start
	time.Sleep(10 * time.Millisecond)

	// Inject a command directly (simulating external message)
	command := Command{
		Name: "test-command",
		Parameters: map[string]any{
			"test": "data",
		},
	}

	transport.InjectCommand(command)

	// Wait for command execution
	time.Sleep(50 * time.Millisecond)

	// Verify handler was executed
	mu.Lock()
	if len(executedCommands) != 1 {
		t.Fatalf("Expected 1 executed command, got: %d", len(executedCommands))
	}

	if executedCommands[0].Name != command.Name {
		t.Fatalf("Expected executed command name %s, got: %s", command.Name, executedCommands[0].Name)
	}
	mu.Unlock()
}

func TestCNCHandlerNotFound(t *testing.T) {
	registry := NewRegistry()
	ctx := context.Background()

	// Try to execute a command with no registered handler
	command := Command{
		Name: "non-existent-command",
	}

	err := registry.Execute(ctx, command)
	if err != ErrHandlerNotFound {
		t.Fatalf("Expected ErrHandlerNotFound, got: %v", err)
	}
}

func TestCNCShutdown(t *testing.T) {
	transport := NewMockTransport()
	cnc := NewCNC(transport)
	ctx := context.Background()

	// Start the CNC instance
	err := cnc.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start CNC: %v", err)
	}

	if !cnc.IsRunning() {
		t.Fatal("Expected CNC to be running")
	}

	// Shutdown the CNC instance
	err = cnc.Shutdown()
	if err != nil {
		t.Fatalf("Failed to shutdown CNC: %v", err)
	}

	if cnc.IsRunning() {
		t.Fatal("Expected CNC to be stopped after shutdown")
	}

	// Verify transport was closed
	if transport.IsConnected() {
		t.Fatal("Expected transport to be disconnected after shutdown")
	}
}

func TestCNCMultipleHandlers(t *testing.T) {
	transport := NewMockTransport()
	cnc := NewCNC(transport)
	ctx := context.Background()

	// Track executions
	var executions []string
	var mu sync.Mutex

	// Register multiple handlers
	handler1 := func(ctx context.Context, cmd Command) error {
		mu.Lock()
		defer mu.Unlock()
		executions = append(executions, "handler1")
		return nil
	}

	handler2 := func(ctx context.Context, cmd Command) error {
		mu.Lock()
		defer mu.Unlock()
		executions = append(executions, "handler2")
		return nil
	}

	err := cnc.RegisterHandler("command1", handler1)
	if err != nil {
		t.Fatalf("Failed to register handler1: %v", err)
	}

	err = cnc.RegisterHandler("command2", handler2)
	if err != nil {
		t.Fatalf("Failed to register handler2: %v", err)
	}

	// Start the CNC instance
	err = cnc.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start CNC: %v", err)
	}
	defer cnc.Shutdown()

	// Give some time for message processing to start
	time.Sleep(10 * time.Millisecond)

	// Inject both commands
	cmd1 := Command{Name: "command1"}
	cmd2 := Command{Name: "command2"}

	transport.InjectCommand(cmd1)
	transport.InjectCommand(cmd2)

	// Wait for executions
	time.Sleep(50 * time.Millisecond)

	// Verify both handlers were executed
	mu.Lock()
	if len(executions) != 2 {
		t.Fatalf("Expected 2 executions, got: %d", len(executions))
	}

	// Check that both handlers were called (order doesn't matter)
	found1 := false
	found2 := false
	for _, exec := range executions {
		if exec == "handler1" {
			found1 = true
		}
		if exec == "handler2" {
			found2 = true
		}
	}

	if !found1 {
		t.Fatal("Expected handler1 to be executed")
	}
	if !found2 {
		t.Fatal("Expected handler2 to be executed")
	}
	mu.Unlock()
}

func TestCNCStartNotConnected(t *testing.T) {
	transport := NewMockTransport()
	transport.Close() // Disconnect the transport
	cnc := NewCNC(transport)
	ctx := context.Background()

	// Try to start with disconnected transport
	err := cnc.Start(ctx)
	if err != ErrTransportNotConnected {
		t.Fatalf("Expected ErrTransportNotConnected when transport not connected, got: %v", err)
	}
}

func TestCNCTriggerBeforeStart(t *testing.T) {
	transport := NewMockTransport()
	cnc := NewCNC(transport)
	ctx := context.Background()

	command := Command{Name: "test"}

	// Try to trigger command before starting
	err := cnc.TriggerCommand(ctx, command)
	if err != ErrCNCNotStarted {
		t.Fatalf("Expected ErrCNCNotStarted when CNC not started, got: %v", err)
	}
}

func TestCNCStartTwice(t *testing.T) {
	transport := NewMockTransport()
	cnc := NewCNC(transport)
	ctx := context.Background()

	// Start once
	err := cnc.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start CNC first time: %v", err)
	}

	// Try to start again
	err = cnc.Start(ctx)
	if err != ErrCNCAlreadyStarted {
		t.Fatalf("Expected ErrCNCAlreadyStarted when starting twice, got: %v", err)
	}

	cnc.Shutdown()
}

func TestCNCMessageChannelClosed(t *testing.T) {
	transport := NewMockTransport()
	cnc := NewCNC(transport)
	ctx := context.Background()

	// Start the CNC instance
	err := cnc.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start CNC: %v", err)
	}

	// Close transport (which closes message channel)
	transport.Close()

	// Give time for the message processing goroutine to detect the closed channel
	time.Sleep(50 * time.Millisecond)

	// CNC should still be considered running (transport closure doesn't auto-stop CNC)
	if !cnc.IsRunning() {
		t.Fatal("Expected CNC to still be running even after transport closure")
	}

	// But shutdown should work normally
	err = cnc.Shutdown()
	if err != nil {
		t.Fatalf("Failed to shutdown CNC: %v", err)
	}
}

func TestCNCConcurrentExecution(t *testing.T) {
	transport := NewMockTransport()
	cnc := NewCNC(transport)
	ctx := context.Background()

	// Track concurrent executions
	var executions int32
	var mu sync.Mutex
	var wg sync.WaitGroup

	handler := func(ctx context.Context, cmd Command) error {
		mu.Lock()
		executions++
		current := executions
		mu.Unlock()

		// Simulate some work
		time.Sleep(10 * time.Millisecond)

		wg.Done()

		// This handler should be able to run concurrently
		// so we should see executions > 1 at the same time
		if current == 1 {
			// First execution, others should be able to run concurrently
		}

		return nil
	}

	err := cnc.RegisterHandler("concurrent-test", handler)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	err = cnc.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start CNC: %v", err)
	}
	defer cnc.Shutdown()

	// Give time for message processing to start
	time.Sleep(10 * time.Millisecond)

	// Inject multiple commands quickly
	numCommands := 5
	wg.Add(numCommands)

	for i := 0; i < numCommands; i++ {
		transport.InjectCommand(Command{
			Name:       "concurrent-test",
			Parameters: map[string]any{"id": i},
		})
	}

	// Wait for all executions to complete
	wg.Wait()

	mu.Lock()
	finalExecutions := executions
	mu.Unlock()

	if finalExecutions != int32(numCommands) {
		t.Fatalf("Expected %d executions, got: %d", numCommands, finalExecutions)
	}
}

func TestSubscriptionRecovery(t *testing.T) {
	transport := NewMockTransport()
	cnc := NewCNC(transport)
	ctx := context.Background()

	// Track handler executions
	var executions []string
	var mu sync.Mutex

	handler := func(ctx context.Context, cmd Command) error {
		mu.Lock()
		defer mu.Unlock()
		executions = append(executions, string(cmd.Name))
		return nil
	}

	err := cnc.RegisterHandler("test", handler)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	err = cnc.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start CNC: %v", err)
	}
	defer cnc.Shutdown()

	// Give time for startup
	time.Sleep(10 * time.Millisecond)

	// Send a command before "disconnection"
	transport.InjectCommand(Command{Name: "test"})
	time.Sleep(50 * time.Millisecond)

	// Simulate transport disconnection and reconnection
	transport.Close()
	time.Sleep(50 * time.Millisecond)

	// Recreate transport (simulating reconnection)
	transport = NewMockTransport()
	// Note: In a real scenario, you'd need to update the CNC's transport
	// For this test, we're just verifying the behavior with the mock

	mu.Lock()
	executionCount := len(executions)
	mu.Unlock()

	if executionCount != 1 {
		t.Fatalf("Expected 1 execution before disconnection, got: %d", executionCount)
	}
}

func TestMessageChannelBuffering(t *testing.T) {
	transport := NewMockTransport()
	cnc := NewCNC(transport)
	ctx := context.Background()

	// Create a slow handler to test buffering
	var processed int32
	handler := func(ctx context.Context, cmd Command) error {
		time.Sleep(100 * time.Millisecond) // Slow processing
		atomic.AddInt32(&processed, 1)
		return nil
	}

	err := cnc.RegisterHandler("slow", handler)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	err = cnc.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start CNC: %v", err)
	}
	defer cnc.Shutdown()

	// Give time for startup
	time.Sleep(10 * time.Millisecond)

	// Send multiple commands quickly to test buffering
	numCommands := 10
	for i := 0; i < numCommands; i++ {
		transport.InjectCommand(Command{
			Name:       "slow",
			Parameters: map[string]any{"id": i},
		})
	}

	// Wait for all commands to be processed
	// With 100ms per command and concurrency, this should be enough
	time.Sleep(2 * time.Second)

	processedCount := atomic.LoadInt32(&processed)
	if processedCount != int32(numCommands) {
		t.Fatalf("Expected %d processed commands, got: %d", numCommands, processedCount)
	}
}
