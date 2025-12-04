package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/AbhishekGY/mydocker/pkg/cgroups"
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
		fmt.Println("\nResource limit flags:")
		fmt.Println("  --memory BYTES         Memory limit in bytes (e.g., 536870912 for 512MB)")
		fmt.Println("  --memory-swap BYTES    Memory + Swap limit in bytes")
		fmt.Println("  --cpu-shares NUM       CPU shares (relative weight, default 1024)")
		fmt.Println("  --cpu-quota MICROS     CPU quota in microseconds (-1 for unlimited)")
		fmt.Println("  --cpu-period MICROS    CPU period in microseconds (default 100000)")
		fmt.Println("  --pids-limit NUM       Maximum number of PIDs/processes")
		fmt.Println("\nExamples:")
		fmt.Println("  mydocker run --memory 536870912 --rootfs /tmp/mydocker-rootfs /bin/sh")
		fmt.Println("  mydocker run --cpu-quota 50000 --cpu-period 100000 --rootfs /tmp/mydocker-rootfs /bin/sh")
		fmt.Println("  mydocker run --memory 536870912 --pids-limit 100 --rootfs /tmp/mydocker-rootfs /bin/sh")
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
	// Create a new FlagSet for the run command
	runFlags := flag.NewFlagSet("run", flag.ExitOnError)

	// Define resource limit flags
	var (
		memory      = runFlags.Uint64("memory", 0, "Memory limit in bytes")
		memorySwap  = runFlags.Uint64("memory-swap", 0, "Memory + Swap limit in bytes")
		cpuShares   = runFlags.Uint64("cpu-shares", 1024, "CPU shares (relative weight)")
		cpuQuota    = runFlags.Int64("cpu-quota", -1, "CPU quota in microseconds")
		cpuPeriod   = runFlags.Uint64("cpu-period", 100000, "CPU period in microseconds")
		pidsLimit   = runFlags.Int64("pids-limit", 0, "Maximum number of PIDs/processes")
		rootfs      = runFlags.String("rootfs", "", "Path to the rootfs directory")
	)

	// Parse flags (skip "mydocker" and "run")
	if err := runFlags.Parse(os.Args[2:]); err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Get the remaining arguments (command and args)
	remainingArgs := runFlags.Args()
	if len(remainingArgs) < 1 {
		fmt.Println("No command specified")
		fmt.Println("Usage: mydocker run [flags] <command> [args...]")
		runFlags.PrintDefaults()
		os.Exit(1)
	}

	command := remainingArgs[0]
	var args []string
	if len(remainingArgs) > 1 {
		args = remainingArgs[1:]
	}

	// Validate resource limits
	if *memory > 0 && *memory < 1048576 { // Less than 1MB
		fmt.Println("Warning: Memory limit is very low (< 1MB)")
	}
	if *cpuQuota > 0 && *cpuPeriod == 0 {
		fmt.Println("Error: cpu-period must be > 0 when cpu-quota is set")
		os.Exit(1)
	}
	if *pidsLimit < 0 {
		fmt.Println("Error: pids-limit must be >= 0")
		os.Exit(1)
	}

	// Create ResourceLimits struct
	limits := cgroups.ResourceLimits{
		MemoryLimit:     *memory,
		MemorySwapLimit: *memorySwap,
		CpuShares:       *cpuShares,
		CpuQuota:        *cpuQuota,
		CpuPeriod:       *cpuPeriod,
		PidsLimit:       *pidsLimit,
	}

	// Print resource limits if any are set
	if *memory > 0 || *cpuQuota != -1 || *pidsLimit > 0 {
		fmt.Println("Resource limits:")
		if *memory > 0 {
			fmt.Printf("  Memory: %d bytes (%.2f MB)\n", *memory, float64(*memory)/(1024*1024))
		}
		if *memorySwap > 0 {
			fmt.Printf("  Memory+Swap: %d bytes (%.2f MB)\n", *memorySwap, float64(*memorySwap)/(1024*1024))
		}
		if *cpuQuota != -1 {
			fmt.Printf("  CPU Quota: %d microseconds\n", *cpuQuota)
			fmt.Printf("  CPU Period: %d microseconds\n", *cpuPeriod)
		}
		if *cpuShares != 1024 {
			fmt.Printf("  CPU Shares: %d\n", *cpuShares)
		}
		if *pidsLimit > 0 {
			fmt.Printf("  PIDs Limit: %d\n", *pidsLimit)
		}
	}

	fmt.Printf("Creating container for command: %s %v with rootfs: %s\n", command, args, *rootfs)

	// Create and start the container
	cont := container.NewContainer(command, args, *rootfs, limits)
	if err := cont.Start(); err != nil {
		fmt.Printf("Error starting container: %v\n", err)
		os.Exit(1)
	}
}
