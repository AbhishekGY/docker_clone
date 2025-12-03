package container

import (
	"fmt"
	"os"

	"github.com/AbhishekGY/mydocker/pkg/filesystem"
	"github.com/AbhishekGY/mydocker/pkg/namespace"
)

// Container represents a containerized process
type Container struct {
	Command string
	Args    []string
	RootFS  string
}

// NewContainer creates a new container for the given command
func NewContainer(command string, args []string, rootfs string) *Container {
	return &Container{
		Command: command,
		Args:    args,
		RootFS:  rootfs,
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

	// Run the command in the container with the rootfs
	return namespace.RunContainer(c.Command, c.Args, c.RootFS)
}
