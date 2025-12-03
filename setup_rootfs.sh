#!/bin/bash
set -e

# Create a basic rootfs using BusyBox
ROOTFS="/tmp/mydocker-rootfs"

# Make sure we have busybox
if ! command -v busybox &> /dev/null; then
    echo "BusyBox not found. Installing it..."
    sudo apt-get update
    sudo apt-get install -y busybox-static
fi

# Create rootfs directory
echo "Creating rootfs at $ROOTFS"
sudo mkdir -p $ROOTFS/{bin,sbin,etc,proc,sys,dev,tmp,root,var/{log,run}}

# Copy busybox
echo "Setting up BusyBox"
BUSYBOX=$(which busybox)
sudo cp $BUSYBOX $ROOTFS/bin/

# Create busybox symlinks
cd $ROOTFS/bin
sudo ln -sf busybox sh
for cmd in $(sudo $ROOTFS/bin/busybox --list); do
    sudo ln -sf busybox $cmd 2>/dev/null || true
done

# Set up required devices
echo "Setting up devices"
sudo mknod -m 666 $ROOTFS/dev/null c 1 3
sudo mknod -m 666 $ROOTFS/dev/zero c 1 5
sudo mknod -m 666 $ROOTFS/dev/random c 1 8
sudo mknod -m 666 $ROOTFS/dev/urandom c 1 9
sudo mknod -m 666 $ROOTFS/dev/tty c 5 0

# Create basic config files
echo "Setting up configuration files"
echo "root:x:0:0:root:/root:/bin/sh" | sudo tee $ROOTFS/etc/passwd
echo "root:x:0:" | sudo tee $ROOTFS/etc/group

echo "BusyBox rootfs created at $ROOTFS"
echo "You can now run: sudo ./mydocker run --rootfs $ROOTFS /bin/sh"