package namespace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// RunContainer will run a command in an isolated environment
func RunContainer(command string, args []string, rootfs string) error {
	fmt.Printf("Running command in container: %s %v with rootfs: %s\n", command, args, rootfs)

	// Check if rootfs exists
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		return fmt.Errorf("rootfs directory doesn't exist: %s", rootfs)
	}

	// We're in the first phase - set up namespaces and re-execute
	fmt.Println("Creating container namespaces...")

	// Get absolute path to current executable
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}
	fmt.Printf("DEBUG: Executable path is %s\n", executable)

	// Prepare arguments for re-execution - recreate the original command
	reexecArgs := os.Args
	fmt.Printf("DEBUG: Re-execution arguments: %v\n", reexecArgs)

	// Create the command to re-execute ourselves
	cmd := exec.Command(executable, reexecArgs...)

	// Pass through standard IO
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set the special environment variable to signal we're in init phase
	cmd.Env = append(os.Environ(), "_CONTAINER_INIT=1")

	// Set up namespaces
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// CLONE_NEWUSER: Isolate user namespace (must be first for unprivileged users)
		// CLONE_NEWUTS: Isolate hostname
		// CLONE_NEWPID: Isolate process IDs
		// CLONE_NEWNS: Isolate mount points
		// CLONE_NEWNET: Isolate network
		Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}

	// Set up user mappings - map container's root to the current user
	cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{
		{
			ContainerID: 0,
			HostID:      os.Getuid(),
			Size:        1,
		},
	}
	cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{
		{
			ContainerID: 0,
			HostID:      os.Getgid(),
			Size:        1,
		},
	}

	// Explicitly create a new session for the terminal
	cmd.SysProcAttr.Setsid = true

	fmt.Println("DEBUG: Starting re-executed process...")
	// Run the command (this executes the same program but in the container namespaces)
	err = cmd.Run()
	fmt.Printf("DEBUG: Re-executed process completed with error: %v\n", err)
	return err
}

// ContainerInit is called when we're in the second phase inside the namespaces
// This is now exported so main.go can call it directly
func ContainerInit(command string, args []string, rootfs string) error {
	fmt.Println("INIT PHASE: Initializing container environment...")

	// Set up mount namespace
	fmt.Println("DEBUG: Setting up mount namespace")
	if err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("failed to make / private: %v", err)
	}

	// Mount proc
	fmt.Println("DEBUG: Mounting proc filesystem")
	procPath := filepath.Join(rootfs, "proc")
	if err := os.MkdirAll(procPath, 0755); err != nil {
		return fmt.Errorf("failed to create proc dir: %v", err)
	}

	if err := syscall.Mount("proc", procPath, "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount proc: %v", err)
	}

	// Change root using pivot_root or fallback to chroot
	fmt.Println("DEBUG: Changing root filesystem")
	if err := pivotRoot(rootfs); err != nil {
		fmt.Printf("pivot_root failed, using chroot: %v\n", err)
		if err := syscall.Chroot(rootfs); err != nil {
			return fmt.Errorf("chroot failed: %v", err)
		}
	}

	// Change directory to /
	fmt.Println("DEBUG: Changing to root directory")
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("failed to chdir: %v", err)
	}

	fmt.Println("Container environment ready, executing command...")
	fmt.Printf("DEBUG: Executing command: %s with args: %v\n", command, args)

	// Add the term environment variable to help with terminal functionality
	os.Setenv("TERM", "xterm")

	// This is where we finally exec the actual container command
	return syscall.Exec(command, append([]string{command}, args...), os.Environ())
}

// pivotRoot performs a pivot_root operation to change the root filesystem
func pivotRoot(newRoot string) error {
	// Ensure new root is an absolute path
	if !filepath.IsAbs(newRoot) {
		return fmt.Errorf("rootfs must be an absolute path")
	}

	fmt.Printf("DEBUG: Setting up pivot_root with %s\n", newRoot)

	// Mount rootfs as MS_BIND
	if err := syscall.Mount(newRoot, newRoot, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mount rootfs failed: %v", err)
	}

	// Create pivot directory
	pivotDir := filepath.Join(newRoot, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0777); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("failed to create pivot dir: %v", err)
		}
	}

	// Pivot root
	fmt.Println("DEBUG: Calling pivot_root syscall")
	if err := syscall.PivotRoot(newRoot, pivotDir); err != nil {
		return fmt.Errorf("pivot_root failed: %v", err)
	}

	// Change working directory
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir failed: %v", err)
	}

	// Unmount old root
	fmt.Println("DEBUG: Unmounting old root")
	if err := syscall.Unmount("/.pivot_root", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir failed: %v", err)
	}

	// Remove pivot directory
	return os.Remove("/.pivot_root")
}
