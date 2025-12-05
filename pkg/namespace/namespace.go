package namespace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// PrepareNamespaces configures an exec.Cmd to run with Linux namespaces
// This should be called before starting the command
func PrepareNamespaces(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Set up namespaces (no user namespace - we're running as root)
		// CLONE_NEWPID: Isolate process IDs
		// CLONE_NEWNS: Isolate mount points
		// CLONE_NEWUTS: Isolate hostname
		// CLONE_NEWNET: Isolate network
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWNET,

		// Create a new session for the terminal
		Setsid: true,
	}
}

// ContainerInit sets up the container environment (mounts, rootfs, etc.)
// This is called by the container-init binary inside the container namespaces
func ContainerInit(rootfs string, command string, args []string) error {
	fmt.Println("Container init: Setting up container environment...")

	// Set up mount namespace - make / private so our mounts don't leak
	if err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("failed to make / private: %v", err)
	}

	// Mount proc filesystem
	procPath := filepath.Join(rootfs, "proc")
	if err := os.MkdirAll(procPath, 0755); err != nil {
		return fmt.Errorf("failed to create proc dir: %v", err)
	}

	if err := syscall.Mount("proc", procPath, "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount proc: %v", err)
	}

	// Change root using pivot_root or fallback to chroot
	if err := pivotRoot(rootfs); err != nil {
		fmt.Printf("pivot_root failed, using chroot: %v\n", err)
		if err := syscall.Chroot(rootfs); err != nil {
			return fmt.Errorf("chroot failed: %v", err)
		}
	}

	// Change directory to /
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("failed to chdir: %v", err)
	}

	// Set up environment
	os.Setenv("TERM", "xterm")

	fmt.Printf("Container init: Executing command: %s %v\n", command, args)

	// Execute the actual container command
	// This replaces the current process with the container command
	return syscall.Exec(command, append([]string{command}, args...), os.Environ())
}

// pivotRoot performs a pivot_root operation to change the root filesystem
func pivotRoot(newRoot string) error {
	// Ensure new root is an absolute path
	if !filepath.IsAbs(newRoot) {
		return fmt.Errorf("rootfs must be an absolute path")
	}

	// Mount rootfs as MS_BIND to make it a mount point
	if err := syscall.Mount(newRoot, newRoot, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mount rootfs failed: %v", err)
	}

	// Create pivot directory for old root
	pivotDir := filepath.Join(newRoot, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0777); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("failed to create pivot dir: %v", err)
		}
	}

	// Pivot root - swaps the root mount
	if err := syscall.PivotRoot(newRoot, pivotDir); err != nil {
		return fmt.Errorf("pivot_root failed: %v", err)
	}

	// Change working directory to new root
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir failed: %v", err)
	}

	// Unmount old root
	if err := syscall.Unmount("/.pivot_root", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir failed: %v", err)
	}

	// Remove pivot directory
	return os.Remove("/.pivot_root")
}
