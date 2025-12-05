package main

import (
	"fmt"
	"os"

	"github.com/AbhishekGY/mydocker/pkg/namespace"
)

// container-init is the init process that runs inside the container namespaces
// It sets up the container environment (mounts, pivot_root, etc.) and then
// execs the actual container command
func main() {
	// Get the rootfs path from environment
	rootfs := os.Getenv("CONTAINER_ROOTFS")
	if rootfs == "" {
		fmt.Fprintf(os.Stderr, "Error: CONTAINER_ROOTFS environment variable not set\n")
		os.Exit(1)
	}

	// Get the command to execute from arguments
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: no command specified\n")
		fmt.Fprintf(os.Stderr, "Usage: container-init <command> [args...]\n")
		os.Exit(1)
	}

	command := os.Args[1]
	args := []string{}
	if len(os.Args) > 2 {
		args = os.Args[2:]
	}

	// Set up the container environment and exec the command
	// This function will not return - it will replace this process with the container command
	if err := namespace.ContainerInit(rootfs, command, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing container: %v\n", err)
		os.Exit(1)
	}
}
