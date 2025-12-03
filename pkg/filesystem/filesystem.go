package filesystem

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// SetupRootfs sets up the root filesystem for the container
func SetupRootfs(rootPath string) error {
	// Check if the rootfs directory exists
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		return fmt.Errorf("rootfs directory doesn't exist: %s", rootPath)
	}

	// Make private mount to avoid propagating mounts
	if err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("failed to make private mount: %v", err)
	}

	// Mount proc filesystem
	procPath := filepath.Join(rootPath, "proc")
	if err := os.MkdirAll(procPath, 0755); err != nil {
		return fmt.Errorf("failed to create proc directory: %v", err)
	}

	if err := syscall.Mount("proc", procPath, "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount proc: %v", err)
	}

	// Try to use pivot_root first, fall back to chroot if necessary
	if err := PivotRoot(rootPath); err != nil {
		fmt.Printf("pivot_root failed, falling back to chroot: %v\n", err)
		// Change root to the new filesystem
		if err := syscall.Chroot(rootPath); err != nil {
			return fmt.Errorf("failed to chroot: %v", err)
		}
	}

	// Change working directory after changing root
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("failed to change directory: %v", err)
	}

	fmt.Printf("Successfully set up rootfs at %s\n", rootPath)
	return nil
}

// CreateRootfs creates a basic rootfs from a base image
func CreateRootfs(baseImage, targetPath string) error {
	// Create the target directory if it doesn't exist
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return fmt.Errorf("failed to create rootfs directory: %v", err)
	}

	// For now, we'll use a simple method - just extract a base image (like busybox)
	// In a real implementation, you'd handle layers and image management
	fmt.Printf("Extracting base image %s to %s\n", baseImage, targetPath)

	// This is a simplified example - in reality, you'd handle container images properly
	// For now, let's just copy some essential binaries if a base image tarball is provided
	if baseImage != "" {
		cmd := exec.Command("tar", "-xf", baseImage, "-C", targetPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to extract base image: %v", err)
		}
	}

	return nil
}

// PivotRoot performs a pivot_root operation, which is more secure than chroot
func PivotRoot(newRoot string) error {
	// Ensure the new root is an absolute path
	newRoot, err := filepath.Abs(newRoot)
	if err != nil {
		return err
	}

	// pivot_root requires that the new root is a mount point
	if err := syscall.Mount(newRoot, newRoot, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mount rootfs failed: %v", err)
	}

	// Create a temporary mount point for old root
	pivotDir := filepath.Join(newRoot, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0777); err != nil {
		// Directory might already exist
		if !os.IsExist(err) {
			return err
		}
	}

	// Pivot the root mount
	if err := syscall.PivotRoot(newRoot, pivotDir); err != nil {
		return fmt.Errorf("pivot_root failed: %v", err)
	}

	// Change working directory to new root
	if err := os.Chdir("/"); err != nil {
		return err
	}

	// Unmount the old root and remove the mount point
	pivotDir = "/.pivot_root"
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir failed: %v", err)
	}

	return os.Remove(pivotDir)
}
