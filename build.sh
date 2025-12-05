#!/bin/bash
set -e

# Find Go executable
GO_CMD=$(which go 2>/dev/null || echo "")
if [ -z "$GO_CMD" ]; then
    # Check common Go installation locations
    if [ -x "/usr/local/go/bin/go" ]; then
        GO_CMD="/usr/local/go/bin/go"
    elif [ -x "$HOME/go/bin/go" ]; then
        GO_CMD="$HOME/go/bin/go"
    else
        echo "Error: Go executable not found in PATH"
        echo "Please install Go or add it to your PATH"
        echo "You can install Go from: https://golang.org/doc/install"
        exit 1
    fi
fi

echo "Using Go executable: $GO_CMD"

# Create bin directory if it doesn't exist
mkdir -p bin

# Build the container-init binary (must be built first, as it's needed by the daemon)
echo "Building container-init..."
$GO_CMD build -o bin/container-init cmd/container-init/main.go

# Build the mydockerd daemon
echo "Building mydockerd..."
$GO_CMD build -o bin/mydockerd cmd/mydockerd/main.go

# Build the mydocker client
echo "Building mydocker client..."
$GO_CMD build -o bin/mydocker cmd/mydocker/main.go

echo "Build completed. The binaries are in the bin/ directory."
echo ""
echo "IMPORTANT: This version requires root privileges to run."
echo "Start the daemon: sudo ./bin/mydockerd"
echo "Then run: ./bin/mydocker run --rootfs /tmp/mydocker-rootfs /bin/sh"

# Ensure rootfs exists
if [ ! -d "/tmp/mydocker-rootfs" ]; then
    echo ""
    echo "Creating rootfs directory..."
    ./setup_rootfs.sh
fi
