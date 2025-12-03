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

# Build the main mydocker binary
echo "Building mydocker..."
$GO_CMD build -o bin/mydocker cmd/mydocker/main.go

echo "Build completed. The mydocker binary is in the bin/ directory."
echo "You can run: ./bin/mydocker run --rootfs /tmp/mydocker-rootfs /bin/sh"

# Ensure rootfs exists
if [ ! -d "/tmp/mydocker-rootfs" ]; then
    echo "Creating rootfs directory..."
    ./setup_rootfs.sh
fi

# Make sure unprivileged user namespaces are enabled
if [ -f /proc/sys/kernel/unprivileged_userns_clone ]; then
    echo "Checking if unprivileged user namespaces are enabled..."
    if [ "$(cat /proc/sys/kernel/unprivileged_userns_clone)" -eq "0" ]; then
        echo "Warning: Unprivileged user namespaces are disabled."
        echo "You may need to enable them with:"
        echo "  sudo sysctl -w kernel.unprivileged_userns_clone=1"
    else
        echo "Unprivileged user namespaces are enabled."
    fi
fi