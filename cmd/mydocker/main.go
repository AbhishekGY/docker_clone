package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/AbhishekGY/mydocker/pkg/api"
)

const defaultSocketPath = "/var/run/mydocker.sock"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Get subcommand
	subcommand := os.Args[1]

	switch subcommand {
	case "run":
		runCommand()
	case "ps":
		psCommand()
	case "stop":
		stopCommand()
	default:
		fmt.Printf("Unknown command: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: mydocker [command] [args...]")
	fmt.Println("Commands:")
	fmt.Println("  run     Create and run a new container")
	fmt.Println("  ps      List containers")
	fmt.Println("  stop    Stop a running container")
	fmt.Println("\nResource limit flags for 'run' command:")
	fmt.Println("  --memory BYTES         Memory limit in bytes (e.g., 536870912 for 512MB)")
	fmt.Println("  --memory-swap BYTES    Memory + Swap limit in bytes")
	fmt.Println("  --cpu-shares NUM       CPU shares (relative weight, default 1024)")
	fmt.Println("  --cpu-quota MICROS     CPU quota in microseconds (-1 for unlimited)")
	fmt.Println("  --cpu-period MICROS    CPU period in microseconds (default 100000)")
	fmt.Println("  --pids-limit NUM       Maximum number of PIDs/processes")
	fmt.Println("  --rootfs PATH          Path to the rootfs directory (required)")
	fmt.Println("  -d, --detach           Run container in detached mode (background)")
	fmt.Println("\nExamples:")
	fmt.Println("  mydocker run --rootfs /tmp/mydocker-rootfs /bin/sh")
	fmt.Println("  mydocker run -d --memory 536870912 --rootfs /tmp/mydocker-rootfs /bin/sleep 300")
	fmt.Println("  mydocker ps")
	fmt.Println("  mydocker stop <container-id>")
}

func runCommand() {
	// Create a new FlagSet for the run command
	runFlags := flag.NewFlagSet("run", flag.ExitOnError)

	// Define resource limit flags
	memory := runFlags.Uint64("memory", 0, "Memory limit in bytes")
	memorySwap := runFlags.Uint64("memory-swap", 0, "Memory + Swap limit in bytes")
	cpuShares := runFlags.Uint64("cpu-shares", 1024, "CPU shares (relative weight)")
	cpuQuota := runFlags.Int64("cpu-quota", -1, "CPU quota in microseconds")
	cpuPeriod := runFlags.Uint64("cpu-period", 100000, "CPU period in microseconds")
	pidsLimit := runFlags.Int64("pids-limit", 0, "Maximum number of PIDs/processes")
	rootfs := runFlags.String("rootfs", "", "Path to the rootfs directory")
	detach := runFlags.Bool("d", false, "Run container in detached mode (background)")
	runFlags.Bool("detach", false, "Run container in detached mode (background)")

	// Parse flags (skip "mydocker" and "run")
	if err := runFlags.Parse(os.Args[2:]); err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Get the remaining arguments (command and args)
	remainingArgs := runFlags.Args()
	if len(remainingArgs) < 1 {
		fmt.Println("Error: No command specified")
		fmt.Println("Usage: mydocker run [flags] <command> [args...]")
		runFlags.PrintDefaults()
		os.Exit(1)
	}

	if *rootfs == "" {
		fmt.Println("Error: --rootfs flag is required")
		os.Exit(1)
	}

	// Create client
	client := api.NewClient(defaultSocketPath)

	// Build request
	req := api.ContainerCreateRequest{
		Image:      *rootfs, // Using rootfs as image for now
		Command:    remainingArgs,
		Rootfs:     *rootfs,
		Memory:     *memory,
		MemorySwap: *memorySwap,
		CpuShares:  *cpuShares,
		CpuQuota:   *cpuQuota,
		CpuPeriod:  *cpuPeriod,
		PidsLimit:  *pidsLimit,
		Detach:     *detach,
	}

	// Create container
	id, err := client.CreateContainer(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating container: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(id)
}

func psCommand() {
	// Create client
	client := api.NewClient(defaultSocketPath)

	// List containers
	containers, err := client.ListContainers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing containers: %v\n", err)
		os.Exit(1)
	}

	// Print containers in a table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CONTAINER ID\tIMAGE\tCOMMAND\tSTATUS\tCREATED\tPID")

	for _, container := range containers {
		// Format created time
		created := time.Unix(container.Created, 0)
		createdStr := formatTimeSince(created)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n",
			container.ID,
			container.Image,
			container.Command,
			container.Status,
			createdStr,
			container.PID,
		)
	}

	w.Flush()
}

func stopCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Error: Container ID required")
		fmt.Println("Usage: mydocker stop <container-id>")
		os.Exit(1)
	}

	containerID := os.Args[2]

	// Create client
	client := api.NewClient(defaultSocketPath)

	// Stop container
	err := client.StopContainer(containerID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping container: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Container %s stopped\n", containerID)
}

// formatTimeSince formats the time since a given time in a human-readable format
func formatTimeSince(t time.Time) string {
	duration := time.Since(t)

	seconds := int(duration.Seconds())
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	if days > 0 {
		return fmt.Sprintf("%d days ago", days)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hours ago", hours)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d minutes ago", minutes)
	}
	return fmt.Sprintf("%d seconds ago", seconds)
}
