package container

import (
	"fmt"
	"os"
	"time"

	"github.com/AbhishekGY/mydocker/pkg/cgroups"
	"github.com/AbhishekGY/mydocker/pkg/filesystem"
	"github.com/AbhishekGY/mydocker/pkg/namespace"
)

// Container represents a containerized process
type Container struct {
	Command string
	Args    []string
	RootFS  string
	Limits  cgroups.ResourceLimits
	CGroup  *cgroups.Cgroup
}

// NewContainer creates a new container for the given command
func NewContainer(command string, args []string, rootfs string, limits cgroups.ResourceLimits) *Container {
	return &Container{
		Command: command,
		Args:    args,
		RootFS:  rootfs,
		Limits:  limits,
	}
}

// Start launches the container
func (c *Container) Start() error {
	// Create a rootfs if one wasn't provided
	if c.RootFS == "" {
		defaultRootfs := "/tmp/mydocker-rootfs"
		fmt.Printf("No rootfs specified, creating a default one at %s\n", defaultRootfs)

		// For now, just create an empty rootfs
		// In a real implementation, this would pull and extract a container image
		if err := os.MkdirAll(defaultRootfs, 0755); err != nil {
			return fmt.Errorf("failed to create default rootfs: %v", err)
		}

		c.RootFS = defaultRootfs
	}

	// Ensure the rootfs exists
	if _, err := os.Stat(c.RootFS); os.IsNotExist(err) {
		// Try to create a basic rootfs
		if err := filesystem.CreateRootfs("", c.RootFS); err != nil {
			return fmt.Errorf("failed to create rootfs: %v", err)
		}
	}

	// Generate a unique cgroup name
	cgroupName := fmt.Sprintf("container-%d", time.Now().UnixNano())

	// Create cgroup with controllers
	cg, err := cgroups.NewCgroup(cgroupName, []cgroups.Controller{
		cgroups.Cpu,
		cgroups.Memory,
		cgroups.Pids,
	})
	if err != nil {
		return fmt.Errorf("failed to create cgroup: %v", err)
	}

	// Create the cgroup directories
	if err := cg.Create(); err != nil {
		return fmt.Errorf("failed to create cgroup directories: %v", err)
	}

	// Store the cgroup reference
	c.CGroup = cg

	// Ensure cgroup cleanup on exit
	defer func() {
		if c.CGroup != nil {
			if err := c.CGroup.Delete(); err != nil {
				fmt.Printf("Warning: failed to cleanup cgroup: %v\n", err)
			} else {
				fmt.Println("Cgroup cleaned up successfully")
			}
		}
	}()

	// Run the command in the container with the rootfs and cgroup
	return namespace.RunContainer(c.Command, c.Args, c.RootFS, cg, c.Limits)
}
