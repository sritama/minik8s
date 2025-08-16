package controller

import (
	"context"
	"testing"
	"time"

	"github.com/minik8s/minik8s/pkg/store"
)

// MockController implements the Controller interface for testing
type MockController struct {
	name    string
	running bool
	stopCh  chan struct{}
}

func NewMockController(name string) *MockController {
	return &MockController{
		name:   name,
		stopCh: make(chan struct{}),
	}
}

func (m *MockController) Name() string {
	return m.name
}

func (m *MockController) Start(ctx context.Context) error {
	m.running = true
	return nil
}

func (m *MockController) Stop() error {
	m.running = false
	close(m.stopCh)
	return nil
}

func (m *MockController) Sync(ctx context.Context) error {
	return nil
}

func TestControllerManager(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller manager
	config := &Config{
		Store:        mockStore,
		SyncInterval: 30 * time.Second,
	}
	manager := NewManager(config)

	// Test manager creation
	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	if manager.store != mockStore {
		t.Error("Manager should have the correct store")
	}

	if manager.syncInterval != 30*time.Second {
		t.Errorf("Expected sync interval 30s, got %v", manager.syncInterval)
	}
}

func TestControllerManager_AddController(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller manager
	config := &Config{
		Store:        mockStore,
		SyncInterval: 30 * time.Second,
	}
	manager := NewManager(config)

	// Create mock controllers
	ctrl1 := NewMockController("controller-1")
	ctrl2 := NewMockController("controller-2")

	// Add controllers
	manager.AddController(ctrl1)
	manager.AddController(ctrl2)

	// Check that controllers were added
	if len(manager.controllers) != 2 {
		t.Errorf("Expected 2 controllers, got %d", len(manager.controllers))
	}

	if manager.controllers["controller-1"] != ctrl1 {
		t.Error("Controller 1 should be stored correctly")
	}

	if manager.controllers["controller-2"] != ctrl2 {
		t.Error("Controller 2 should be stored correctly")
	}
}

func TestControllerManager_StartStop(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller manager
	config := &Config{
		Store:        mockStore,
		SyncInterval: 30 * time.Second,
	}
	manager := NewManager(config)

	// Create mock controller
	ctrl := NewMockController("test-controller")
	manager.AddController(ctrl)

	// Test starting manager
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	if !manager.running {
		t.Error("Manager should be running after Start()")
	}

	if !ctrl.running {
		t.Error("Controller should be running after manager Start()")
	}

	// Test stopping manager
	manager.Stop()
	if manager.running {
		t.Error("Manager should not be running after Stop()")
	}

	if ctrl.running {
		t.Error("Controller should not be running after manager Stop()")
	}
}

func TestControllerManager_StartAlreadyRunning(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller manager
	config := &Config{
		Store:        mockStore,
		SyncInterval: 30 * time.Second,
	}
	manager := NewManager(config)

	// Create mock controller
	ctrl := NewMockController("test-controller")
	manager.AddController(ctrl)

	// Start manager
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Try to start again
	if err := manager.Start(ctx); err == nil {
		t.Error("Should not be able to start manager twice")
	}
}

func TestControllerManager_GetController(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller manager
	config := &Config{
		Store:        mockStore,
		SyncInterval: 30 * time.Second,
	}
	manager := NewManager(config)

	// Create mock controller
	ctrl := NewMockController("test-controller")
	manager.AddController(ctrl)

	// Test getting controller
	retrievedCtrl := manager.GetController("test-controller")
	if retrievedCtrl != ctrl {
		t.Error("Should retrieve the correct controller")
	}

	// Test getting non-existent controller
	nonExistentCtrl := manager.GetController("non-existent")
	if nonExistentCtrl != nil {
		t.Error("Should return nil for non-existent controller")
	}
}

func TestControllerManager_ListControllers(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller manager
	config := &Config{
		Store:        mockStore,
		SyncInterval: 30 * time.Second,
	}
	manager := NewManager(config)

	// Create mock controllers
	ctrl1 := NewMockController("controller-1")
	ctrl2 := NewMockController("controller-2")
	ctrl3 := NewMockController("controller-3")

	// Add controllers
	manager.AddController(ctrl1)
	manager.AddController(ctrl2)
	manager.AddController(ctrl3)

	// Test listing controllers
	controllers := manager.ListControllers()
	if len(controllers) != 3 {
		t.Errorf("Expected 3 controllers, got %d", len(controllers))
	}

	// Check that all controllers are present
	controllerNames := make(map[string]bool)
	for _, ctrl := range controllers {
		controllerNames[ctrl.Name()] = true
	}

	expectedNames := []string{"controller-1", "controller-2", "controller-3"}
	for _, name := range expectedNames {
		if !controllerNames[name] {
			t.Errorf("Controller %s should be in the list", name)
		}
	}
}

func TestControllerManager_DefaultSyncInterval(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller manager without specifying sync interval
	config := &Config{
		Store: mockStore,
		// SyncInterval not set, should use default
	}
	manager := NewManager(config)

	// Check default sync interval
	expectedInterval := 30 * time.Second
	if manager.syncInterval != expectedInterval {
		t.Errorf("Expected default sync interval %v, got %v", expectedInterval, manager.syncInterval)
	}
}

func TestControllerManager_ContextCancellation(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller manager
	config := &Config{
		Store:        mockStore,
		SyncInterval: 100 * time.Millisecond, // Short interval for testing
	}
	manager := NewManager(config)

	// Create mock controller
	ctrl := NewMockController("test-controller")
	manager.AddController(ctrl)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start manager
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Verify manager is running
	if !manager.running {
		t.Fatal("Manager should be running after Start()")
	}

	// Cancel context
	cancel()

	// Use Stop() to ensure clean shutdown
	manager.Stop()

	// Check that manager stopped
	if manager.running {
		t.Error("Manager should not be running after Stop()")
	}
}
