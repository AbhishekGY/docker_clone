package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/AbhishekGY/mydocker/pkg/state"
)

// Daemon represents the container daemon
type Daemon struct {
	socketPath string
	dataDir    string
	store      *state.Store
	containers map[string]*state.ContainerState
	mu         sync.RWMutex
}

// NewDaemon creates a new daemon instance
func NewDaemon(socketPath, dataDir string) (*Daemon, error) {
	// Initialize the state store
	store, err := state.NewStore(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create state store: %v", err)
	}

	d := &Daemon{
		socketPath: socketPath,
		dataDir:    dataDir,
		store:      store,
		containers: make(map[string]*state.ContainerState),
	}

	// Load existing containers from disk
	if err := d.loadContainers(); err != nil {
		return nil, fmt.Errorf("failed to load containers: %v", err)
	}

	return d, nil
}

// loadContainers loads all existing containers from the state store
func (d *Daemon) loadContainers() error {
	containers, err := d.store.ListContainers()
	if err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, container := range containers {
		d.containers[container.ID] = container
	}

	fmt.Printf("Loaded %d container(s) from disk\n", len(containers))
	return nil
}

// generateContainerID generates a random container ID
func (d *Daemon) generateContainerID() string {
	bytes := make([]byte, 6)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// getContainer retrieves a container by ID (thread-safe)
func (d *Daemon) getContainer(id string) (*state.ContainerState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	container, exists := d.containers[id]
	if !exists {
		return nil, fmt.Errorf("container not found: %s", id)
	}

	return container, nil
}

// addContainer adds a container to the daemon's state (thread-safe)
func (d *Daemon) addContainer(containerState *state.ContainerState) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Add to in-memory map
	d.containers[containerState.ID] = containerState

	// Persist to disk
	if err := d.store.SaveContainer(containerState); err != nil {
		// Rollback in-memory state on error
		delete(d.containers, containerState.ID)
		return fmt.Errorf("failed to save container state: %v", err)
	}

	return nil
}

// removeContainer removes a container from the daemon's state (thread-safe)
func (d *Daemon) removeContainer(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Remove from in-memory map
	delete(d.containers, id)

	// Remove from disk
	if err := d.store.DeleteContainer(id); err != nil {
		return fmt.Errorf("failed to delete container state: %v", err)
	}

	return nil
}
