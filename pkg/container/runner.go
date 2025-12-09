package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/AbhishekGY/mydocker/pkg/cgroups"
	"github.com/AbhishekGY/mydocker/pkg/namespace"
)

// Runner manages the lifecycle of a running container
type Runner struct {
	ID      string
	Command []string
	Rootfs  string
	Cgroup  *cgroups.Cgroup
	Cmd     *exec.Cmd
}

// NewRunner creates a new container runner and sets up its cgroup
func NewRunner(id string, command []string, rootfs string, limits cgroups.ResourceLimits) (*Runner, error) {
	// Validate inputs
	if len(command) == 0 {
		return nil, fmt.Errorf("command cannot be empty")
	}
	if rootfs == "" {
		return nil, fmt.Errorf("rootfs cannot be empty")
	}

	// Check if rootfs exists
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		return nil, fmt.Errorf("rootfs directory doesn't exist: %s", rootfs)
	}

	// TODO: Cgroups disabled for now - will implement later
	// For now, just create the runner without cgroup

	return &Runner{
		ID:      id,
		Command: command,
		Rootfs:  rootfs,
		Cgroup:  nil, // No cgroup for now
	}, nil
}

// Start prepares and starts the container process in the background
func (r *Runner) Start() error {
	// Find the path to container-init binary
	// It should be in the same directory as the mydockerd binary
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}
	execDir := filepath.Dir(execPath)
	initPath := filepath.Join(execDir, "container-init")

	// Check if container-init exists
	if _, err := os.Stat(initPath); os.IsNotExist(err) {
		return fmt.Errorf("container-init binary not found at %s", initPath)
	}

	// Prepare the command to run container-init
	// container-init will set up the container environment and exec the actual command
	args := append([]string{initPath}, r.Command...)
	r.Cmd = exec.Command(args[0], args[1:]...)

	// Pass the rootfs path via environment variable
	r.Cmd.Env = append(os.Environ(), fmt.Sprintf("CONTAINER_ROOTFS=%s", r.Rootfs))

	// Set up stdin/stdout/stderr
	r.Cmd.Stdin = nil        // Containers run in background, no stdin
	r.Cmd.Stdout = os.Stdout // For now, log to daemon's stdout
	r.Cmd.Stderr = os.Stderr // For now, log to daemon's stderr

	// Configure namespaces
	namespace.PrepareNamespaces(r.Cmd)

	// Start the process in the background
	if err := r.Cmd.Start(); err != nil {
		return fmt.Errorf("failed to start container process: %v", err)
	}

	// TODO: Cgroup support disabled - skip AddProcess for now

	return nil
}

// Wait blocks until the container process exits
func (r *Runner) Wait() error {
	if r.Cmd == nil || r.Cmd.Process == nil {
		return fmt.Errorf("container not started")
	}
	return r.Cmd.Wait()
}

// Stop sends SIGTERM to the container process
func (r *Runner) Stop() error {
	if r.Cmd == nil || r.Cmd.Process == nil {
		return fmt.Errorf("container not started")
	}
	return r.Cmd.Process.Signal(syscall.SIGTERM)
}

// Kill sends SIGKILL to the container process
func (r *Runner) Kill() error {
	if r.Cmd == nil || r.Cmd.Process == nil {
		return fmt.Errorf("container not started")
	}
	return r.Cmd.Process.Kill()
}

// PID returns the process ID of the container
func (r *Runner) PID() int {
	if r.Cmd == nil || r.Cmd.Process == nil {
		return 0
	}
	return r.Cmd.Process.Pid
}

// Cleanup removes the cgroup for this container
func (r *Runner) Cleanup() error {
	// TODO: Cgroup cleanup disabled for now
	return nil
}

// WaitWithTimeout waits for the container to exit with a timeout
// Returns nil if process exits within timeout, error otherwise
func (r *Runner) WaitWithTimeout(timeout time.Duration) error {
	if r.Cmd == nil || r.Cmd.Process == nil {
		return fmt.Errorf("container not started")
	}

	done := make(chan error, 1)
	go func() {
		done <- r.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for container to exit")
	}
}
