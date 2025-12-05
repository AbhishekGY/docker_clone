package container

import (
	"github.com/AbhishekGY/mydocker/pkg/cgroups"
)

// Container represents a containerized process
// This is a simple data holder for container configuration
type Container struct {
	Command string
	Args    []string
	RootFS  string
	Limits  cgroups.ResourceLimits
}

// NewContainer creates a new container configuration
func NewContainer(command string, args []string, rootfs string, limits cgroups.ResourceLimits) *Container {
	return &Container{
		Command: command,
		Args:    args,
		RootFS:  rootfs,
		Limits:  limits,
	}
}
