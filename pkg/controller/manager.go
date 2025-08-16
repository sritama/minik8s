package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/minik8s/minik8s/pkg/store"
)

// Manager manages all controllers
type Manager struct {
	mu sync.RWMutex

	// Configuration
	store store.Store

	// Controllers
	controllers map[string]Controller
	running     bool
	stopCh      chan struct{}

	// Configuration
	syncInterval time.Duration
}

// Controller defines the interface for all controllers
type Controller interface {
	// Name returns the name of the controller
	Name() string

	// Start starts the controller
	Start(ctx context.Context) error

	// Stop stops the controller
	Stop() error

	// Sync performs a single sync operation
	Sync(ctx context.Context) error
}

// Config holds the configuration for the controller manager
type Config struct {
	Store        store.Store
	SyncInterval time.Duration
}

// NewManager creates a new controller manager
func NewManager(config *Config) *Manager {
	if config.SyncInterval == 0 {
		config.SyncInterval = 30 * time.Second
	}

	return &Manager{
		store:        config.Store,
		controllers:  make(map[string]Controller),
		syncInterval: config.SyncInterval,
		stopCh:       make(chan struct{}),
	}
}

// AddController adds a controller to the manager
func (m *Manager) AddController(controller Controller) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.controllers[controller.Name()] = controller
}

// Start starts the controller manager
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("controller manager is already running")
	}

	// Start all controllers
	for _, controller := range m.controllers {
		if err := controller.Start(ctx); err != nil {
			return fmt.Errorf("failed to start controller %s: %w", controller.Name(), err)
		}
	}

	// Start background sync loop
	go m.syncLoop(ctx)

	m.running = true
	return nil
}

// Stop stops the controller manager
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	// Stop all controllers
	for _, controller := range m.controllers {
		controller.Stop()
	}

	close(m.stopCh)
	m.running = false
}

// syncLoop continuously syncs all controllers
func (m *Manager) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(m.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			// Check context again before syncing
			select {
			case <-ctx.Done():
				return
			default:
				if err := m.syncAll(ctx); err != nil {
					// Log error but continue
					fmt.Printf("Error syncing controllers: %v\n", err)
				}
			}
		}
	}
}

// syncAll syncs all controllers
func (m *Manager) syncAll(ctx context.Context) error {
	m.mu.RLock()
	controllers := make([]Controller, 0, len(m.controllers))
	for _, controller := range m.controllers {
		controllers = append(controllers, controller)
	}
	m.mu.RUnlock()

	// Sync each controller
	for _, controller := range controllers {
		if err := controller.Sync(ctx); err != nil {
			fmt.Printf("Error syncing controller %s: %v\n", controller.Name(), err)
		}
	}

	return nil
}

// GetController returns a controller by name
func (m *Manager) GetController(name string) Controller {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.controllers[name]
}

// ListControllers returns all controllers
func (m *Manager) ListControllers() []Controller {
	m.mu.RLock()
	defer m.mu.RUnlock()

	controllers := make([]Controller, 0, len(m.controllers))
	for _, controller := range m.controllers {
		controllers = append(controllers, controller)
	}

	return controllers
}
