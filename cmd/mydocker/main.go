package main

import (
	"fmt"
	"os"

	"github.com/AbhishekGY/mydocker/pkg/container"
	"github.com/AbhishekGY/mydocker/pkg/namespace"
)

func main() {
	// If we're in container init mode, we need to directly handle the container init
	if os.Getenv("_CONTAINER_INIT") == "1" {
		fmt.Println("MAIN: Container init mode detected")

		// Parse for the original run command pattern: mydocker run --rootfs PATH COMMAND [ARGS...]
		var rootfs, command string
		var args []string

		// Find rootfs
		for i := 1; i < len(os.Args); i++ {
			if os.Args[i] == "--rootfs" && i+1 < len(os.Args) {
				rootfs = os.Args[i+1]
				break
			}
		}

		// Find command and args - command is first arg after rootfs path
		foundRootfs := false
		for i := 1; i < len(os.Args); i++ {
			if os.Args[i] == "--rootfs" {
				// Skip the rootfs flag and value
				i++
				foundRootfs = true
				continue
			}

			if foundRootfs {
				// First argument after rootfs is the command
				if command == "" {
					command = os.Args[i]
				} else {
					// Remaining arguments are command args
					args = append(args, os.Args[i])
				}
			}
		}

		fmt.Printf("MAIN: Extracted command: %s, args: %v, rootfs: %s\n", command, args, rootfs)

		// Call the ContainerInit function directly
		if err := namespace.ContainerInit(command, args, rootfs); err != nil {
			fmt.Printf("Container initialization failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Normal command parsing for the first execution
	if len(os.Args) < 2 {
		fmt.Println("Usage: mydocker [command] [args...]")
		fmt.Println("Commands:")
		fmt.Println("  run     Run a command in a new container")
		os.Exit(1)
	}

	// Check if the command is "run"
	switch os.Args[1] {
	case "run":
		run()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		fmt.Println("Usage: mydocker run <command> [args...]")
		os.Exit(1)
	}
}

func run() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mydocker run [--rootfs PATH] <command> [args...]")
		os.Exit(1)
	}

	var command string
	var args []string
	var rootfs string

	// Parse arguments
	argIndex := 2
	for argIndex < len(os.Args) {
		if os.Args[argIndex] == "--rootfs" && argIndex+1 < len(os.Args) {
			rootfs = os.Args[argIndex+1]
			argIndex += 2
		} else {
			break
		}
	}

	if argIndex >= len(os.Args) {
		fmt.Println("No command specified")
		fmt.Println("Usage: mydocker run [--rootfs PATH] <command> [args...]")
		os.Exit(1)
	}

	command = os.Args[argIndex]
	if argIndex+1 < len(os.Args) {
		args = os.Args[argIndex+1:]
	}

	fmt.Printf("Creating container for command: %s %v with rootfs: %s\n", command, args, rootfs)

	// Create and start the container
	cont := container.NewContainer(command, args, rootfs)
	if err := cont.Start(); err != nil {
		fmt.Printf("Error starting container: %v\n", err)
		os.Exit(1)
	}
}
