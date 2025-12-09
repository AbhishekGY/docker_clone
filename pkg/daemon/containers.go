package daemon

import (
	"fmt"
	"time"

	"github.com/AbhishekGY/mydocker/pkg/api"
	"github.com/AbhishekGY/mydocker/pkg/cgroups"
	"github.com/AbhishekGY/mydocker/pkg/container"
	"github.com/AbhishekGY/mydocker/pkg/state"
)

// CreateContainer creates and starts a new container
func (d *Daemon) CreateContainer(req api.ContainerCreateRequest) (string, *container.Runner, error) {
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
		return "", nil, fmt.Errorf("failed to add container: %v", err)
	}

	fmt.Printf("Created container %s (status: created)\n", id)

	// Start the container immediately
	runner, err := d.StartContainerWithRunner(id, req.Detach)
	if err != nil {
		// If start fails, update state to reflect failure
		containerState.Status = "exited"
		d.updateContainer(containerState)
		return "", nil, fmt.Errorf("failed to start container: %v", err)
	}

	return id, runner, nil
}

// StartContainer starts a created container (with detach=true by default for backward compatibility)
func (d *Daemon) StartContainer(id string) error {
	_, err := d.StartContainerWithRunner(id, true)
	return err
}

// StartContainerWithRunner starts a created container and returns the runner
func (d *Daemon) StartContainerWithRunner(id string, detach bool) (*container.Runner, error) {
	// Get container state
	containerState, err := d.getContainer(id)
	if err != nil {
		return nil, err
	}

	// Check if container is already running
	if containerState.Status == "running" {
		return nil, fmt.Errorf("container is already running")
	}

	// Create the runner
	runner, err := container.NewRunner(id, containerState.Command, containerState.Rootfs, containerState.Limits, detach)
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %v", err)
	}

	// Start the container process
	if err := runner.Start(); err != nil {
		// Clean up cgroup on failure
		runner.Cleanup()
		return nil, fmt.Errorf("failed to start container process: %v", err)
	}

	// Update container state
	containerState.PID = runner.PID()
	containerState.Status = "running"
	if err := d.updateContainer(containerState); err != nil {
		// If we can't save state, kill the container
		runner.Kill()
		runner.Cleanup()
		return nil, fmt.Errorf("failed to update container state: %v", err)
	}

	// Store runner in daemon
	d.addRunner(id, runner)

	fmt.Printf("Started container %s with PID %d\n", id, runner.PID())

	// Launch goroutine to monitor container
	go d.monitorContainer(id, runner)

	return runner, nil
}

// monitorContainer monitors a running container and updates state when it exits
func (d *Daemon) monitorContainer(id string, runner *container.Runner) {
	// Wait for container to exit (blocks until exit)
	err := runner.Wait()

	fmt.Printf("Container %s exited", id)
	if err != nil {
		fmt.Printf(" with error: %v\n", err)
	} else {
		fmt.Println()
	}

	// Get container state
	containerState, err := d.getContainer(id)
	if err != nil {
		fmt.Printf("Error getting container state for %s: %v\n", id, err)
		return
	}

	// Update state to exited
	containerState.Status = "exited"
	containerState.PID = 0
	if err := d.updateContainer(containerState); err != nil {
		fmt.Printf("Error updating container state for %s: %v\n", id, err)
	}

	// Cleanup cgroup
	if err := runner.Cleanup(); err != nil {
		fmt.Printf("Error cleaning up container %s: %v\n", id, err)
	}

	// Remove runner from daemon
	d.removeRunner(id)
}

// StopContainer stops a running container
func (d *Daemon) StopContainer(id string) error {
	// Get container state
	containerState, err := d.getContainer(id)
	if err != nil {
		return err
	}

	// Check if container is running
	if containerState.Status != "running" {
		return fmt.Errorf("container is not running (status: %s)", containerState.Status)
	}

	// Get runner
	runner, err := d.getRunner(id)
	if err != nil {
		return fmt.Errorf("runner not found for container %s", id)
	}

	// Send SIGTERM
	fmt.Printf("Sending SIGTERM to container %s (PID %d)\n", id, runner.PID())
	if err := runner.Stop(); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %v", err)
	}

	// Wait with timeout (5 seconds)
	if err := runner.WaitWithTimeout(5 * time.Second); err != nil {
		// Still running after timeout, force kill
		fmt.Printf("Container %s did not stop gracefully, sending SIGKILL\n", id)
		if err := runner.Kill(); err != nil {
			return fmt.Errorf("failed to kill container: %v", err)
		}
	}

	// The monitorContainer goroutine will handle cleanup and state update
	return nil
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
