package daemon

import (
	"fmt"
	"time"

	"github.com/AbhishekGY/mydocker/pkg/api"
	"github.com/AbhishekGY/mydocker/pkg/cgroups"
	"github.com/AbhishekGY/mydocker/pkg/state"
)

// CreateContainer creates a new container but does not start it yet
func (d *Daemon) CreateContainer(req api.ContainerCreateRequest) (string, error) {
	// Generate a unique container ID
	id := d.generateContainerID()

	// Create resource limits from request
	limits := cgroups.ResourceLimits{
		MemoryLimit:     req.Memory,
		MemorySwapLimit: req.MemorySwap,
		CpuShares:       req.CpuShares,
		CpuQuota:        req.CpuQuota,
		CpuPeriod:       req.CpuPeriod,
		PidsLimit:       req.PidsLimit,
	}

	// Create container state
	containerState := &state.ContainerState{
		ID:      id,
		PID:     0, // Not started yet
		Status:  "created",
		Command: req.Command,
		Rootfs:  req.Rootfs,
		Created: time.Now(),
		Limits:  limits,
	}

	// Add container to daemon state
	if err := d.addContainer(containerState); err != nil {
		return "", fmt.Errorf("failed to add container: %v", err)
	}

	fmt.Printf("Created container %s (status: created)\n", id)
	return id, nil
}

// ListContainers returns information about all containers
func (d *Daemon) ListContainers() []api.ContainerInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	containers := make([]api.ContainerInfo, 0, len(d.containers))
	for _, container := range d.containers {
		// Build command string
		commandStr := ""
		if len(container.Command) > 0 {
			commandStr = container.Command[0]
			if len(container.Command) > 1 {
				for _, arg := range container.Command[1:] {
					commandStr += " " + arg
				}
			}
		}

		info := api.ContainerInfo{
			ID:      container.ID,
			Image:   container.Rootfs,
			Command: commandStr,
			Status:  container.Status,
			Created: container.Created.Unix(),
			PID:     container.PID,
		}
		containers = append(containers, info)
	}

	return containers
}

// StopContainer stops a running container
func (d *Daemon) StopContainer(id string) error {
	// This is a placeholder implementation for Phase 1
	_, err := d.getContainer(id)
	if err != nil {
		return err
	}

	return fmt.Errorf("not implemented")
}
