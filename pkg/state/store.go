package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AbhishekGY/mydocker/pkg/cgroups"
)

// Store manages persistent storage of container state
type Store struct {
	dataDir string
}

// ContainerState represents the persistent state of a container
type ContainerState struct {
	ID      string                  `json:"id"`
	PID     int                     `json:"pid"`
	Status  string                  `json:"status"`
	Command []string                `json:"command"`
	Rootfs  string                  `json:"rootfs"`
	Created time.Time               `json:"created"`
	Limits  cgroups.ResourceLimits  `json:"limits"`
}

// NewStore creates a new state store
func NewStore(dataDir string) (*Store, error) {
	// Create the data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	return &Store{
		dataDir: dataDir,
	}, nil
}

// SaveContainer saves a container's state to disk
func (s *Store) SaveContainer(state *ContainerState) error {
	filename := filepath.Join(s.dataDir, fmt.Sprintf("%s.json", state.ID))

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal container state: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write container state: %v", err)
	}

	return nil
}

// LoadContainer loads a container's state from disk
func (s *Store) LoadContainer(id string) (*ContainerState, error) {
	filename := filepath.Join(s.dataDir, fmt.Sprintf("%s.json", id))

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read container state: %v", err)
	}

	var state ContainerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal container state: %v", err)
	}

	return &state, nil
}

// ListContainers returns all container states stored on disk
func (s *Store) ListContainers() ([]*ContainerState, error) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read data directory: %v", err)
	}

	var containers []*ContainerState
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		state, err := s.LoadContainer(id)
		if err != nil {
			// Log error but continue with other containers
			fmt.Printf("Warning: failed to load container %s: %v\n", id, err)
			continue
		}

		containers = append(containers, state)
	}

	return containers, nil
}

// DeleteContainer removes a container's state from disk
func (s *Store) DeleteContainer(id string) error {
	filename := filepath.Join(s.dataDir, fmt.Sprintf("%s.json", id))

	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete container state: %v", err)
	}

	return nil
}
