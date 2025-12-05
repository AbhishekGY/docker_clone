package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/AbhishekGY/mydocker/pkg/container"
	"github.com/AbhishekGY/mydocker/pkg/state"
)

// Daemon represents the container daemon
type Daemon struct {
	socketPath string
	dataDir    string
	store      *state.Store
	containers map[string]*state.ContainerState
	runners    map[string]*container.Runner
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
		runners:    make(map[string]*container.Runner),
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
		// Check if container was running when daemon stopped
		if container.Status == "running" && container.PID > 0 {
			// Check if process still exists
			if err := syscall.Kill(container.PID, 0); err != nil {
				// Process is dead, update state
				fmt.Printf("Container %s was running but process %d is dead, marking as exited\n",
					container.ID, container.PID)
				container.Status = "exited"
				container.PID = 0
				// Save updated state
				if err := d.store.SaveContainer(container); err != nil {
					fmt.Printf("Warning: failed to update container state: %v\n", err)
				}
			} else {
				// Process is still alive - we could re-attach but for now just mark as exited
				// In a production system, we'd re-attach to the running process
				fmt.Printf("Container %s process %d is still running, marking as exited (re-attach not implemented)\n",
					container.ID, container.PID)
				container.Status = "exited"
				container.PID = 0
				if err := d.store.SaveContainer(container); err != nil {
					fmt.Printf("Warning: failed to update container state: %v\n", err)
				}
			}
		}

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

// updateContainer updates a container's state (thread-safe)
func (d *Daemon) updateContainer(containerState *state.ContainerState) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Update in-memory state
	d.containers[containerState.ID] = containerState

	// Persist to disk
	if err := d.store.SaveContainer(containerState); err != nil {
		return fmt.Errorf("failed to save container state: %v", err)
	}

	return nil
}

// getRunner retrieves a runner by container ID (thread-safe)
func (d *Daemon) getRunner(id string) (*container.Runner, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	runner, exists := d.runners[id]
	if !exists {
		return nil, fmt.Errorf("runner not found for container: %s", id)
	}

	return runner, nil
}

// addRunner adds a runner to the daemon's state (thread-safe)
func (d *Daemon) addRunner(id string, runner *container.Runner) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.runners[id] = runner
}

// removeRunner removes a runner from the daemon's state (thread-safe)
func (d *Daemon) removeRunner(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.runners, id)
}

// stopAllContainers stops all running containers
func (d *Daemon) stopAllContainers() {
	// Get all runner IDs
	d.mu.RLock()
	runnerIDs := make([]string, 0, len(d.runners))
	for id := range d.runners {
		runnerIDs = append(runnerIDs, id)
	}
	d.mu.RUnlock()

	// Stop all running containers
	for _, id := range runnerIDs {
		fmt.Printf("Stopping container %s...\n", id)
		runner, err := d.getRunner(id)
		if err != nil {
			continue
		}

		// Try graceful stop with timeout
		if err := runner.Stop(); err != nil {
			fmt.Printf("Warning: failed to send SIGTERM to container %s: %v\n", id, err)
		}

		// Wait with timeout
		if err := runner.WaitWithTimeout(5 * time.Second); err != nil {
			// Force kill if still running
			fmt.Printf("Container %s did not stop gracefully, killing...\n", id)
			runner.Kill()
		}

		// Cleanup
		runner.Cleanup()
	}
}
