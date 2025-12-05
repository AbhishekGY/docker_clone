package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AbhishekGY/mydocker/pkg/daemon"
)

func main() {
	// Check if running as root
	if os.Getuid() != 0 {
		fmt.Fprintln(os.Stderr, "Error: mydockerd must be run as root")
		os.Exit(1)
	}

	// Parse command-line flags
	socketPath := flag.String("socket", "/var/run/mydocker.sock", "Path to Unix socket")
	dataDir := flag.String("data-dir", "/var/lib/mydocker", "Path to data directory")
	flag.Parse()

	// Create daemon instance
	d, err := daemon.NewDaemon(*socketPath, *dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create daemon: %v\n", err)
		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start daemon in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := d.Start(); err != nil {
			errChan <- err
		}
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal: %v\n", sig)
		if err := d.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Error stopping daemon: %v\n", err)
			os.Exit(1)
		}
	case err := <-errChan:
		fmt.Fprintf(os.Stderr, "Daemon error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Daemon stopped")
}
