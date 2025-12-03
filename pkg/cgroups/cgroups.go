package cgroups

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Controller represents a cgroup controller/subsystem
type Controller string

// Available cgroup controllers
const (
	Cpu    Controller = "cpu"
	Memory Controller = "memory"
	CpuSet Controller = "cpuset"
	Pids   Controller = "pids"
	BlkIO  Controller = "blkio"
)

// Cgroup represents a control group
type Cgroup struct {
	Name        string
	Controllers []Controller
	Path        string
}

// ResourceLimits defines resource constraints for a container
type ResourceLimits struct {
	// CPU limits
	CpuShares uint64 // CPU shares (relative weight)
	CpuQuota  int64  // CPU quota in microseconds (-1 for no limit)
	CpuPeriod uint64 // CPU period in microseconds

	// Memory limits
	MemoryLimit     uint64 // Memory limit in bytes
	MemorySwapLimit uint64 // Memory+Swap limit in bytes

	// Process limits
	PidsLimit int64 // Maximum number of processes
}

// DefaultResourceLimits returns default resource limits
func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		CpuShares:       1024,   // Default CPU shares
		CpuQuota:        -1,     // No quota by default
		CpuPeriod:       100000, // 100ms default period
		MemoryLimit:     0,      // No memory limit by default
		MemorySwapLimit: 0,      // No swap limit by default
		PidsLimit:       0,      // No process limit by default
	}
}

func NewCgroup(name string, controllers []Controller) (*Cgroup, error) {
	// Prepare the cgroup name - sanitize it for use in filesystem
	cgroupName := fmt.Sprintf("mydocker-%s", strings.Replace(name, "/", "_", -1))

	cg := &Cgroup{
		Name:        cgroupName,
		Controllers: controllers,
	}

	// Detect cgroups v2 unified hierarchy
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		// We're using cgroups v2
		cg.Path = filepath.Join("/sys/fs/cgroup", cgroupName)
		return cg, nil
	}

	// Fallback to cgroups v1
	cg.Path = "" // We'll set individual paths per controller
	return cg, nil
}

// Create creates the cgroup directories for all specified controllers
func (cg *Cgroup) Create() error {
	// Check if we're using cgroups v2
	if cg.Path != "" {
		// Create the unified cgroup directory
		if err := os.MkdirAll(cg.Path, 0755); err != nil {
			return fmt.Errorf("failed to create unified cgroup %s: %v", cg.Path, err)
		}

		// Enable controllers in the unified hierarchy
		controllerList := []string{}
		for _, ctrl := range cg.Controllers {
			controllerList = append(controllerList, string(ctrl))
		}

		// Try to enable controllers (may fail if we don't have permissions)
		enablePath := filepath.Join(cg.Path, "cgroup.subtree_control")
		_ = os.WriteFile(enablePath, []byte("+"+strings.Join(controllerList, " +")), 0644)

		return nil
	}

	// For cgroups v1, create a directory for each controller
	for _, ctrl := range cg.Controllers {
		cgPath := filepath.Join("/sys/fs/cgroup", string(ctrl), cg.Name)
		if err := os.MkdirAll(cgPath, 0755); err != nil {
			return fmt.Errorf("failed to create cgroup %s: %v", cgPath, err)
		}
	}

	return nil
}

// Delete removes the cgroup
func (cg *Cgroup) Delete() error {
	// Check if we're using cgroups v2
	if cg.Path != "" {
		return os.RemoveAll(cg.Path)
	}

	// For cgroups v1, remove directories for each controller
	var lastErr error
	for _, ctrl := range cg.Controllers {
		cgPath := filepath.Join("/sys/fs/cgroup", string(ctrl), cg.Name)
		if err := os.RemoveAll(cgPath); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// AddProcess adds a process to the cgroup
func (cg *Cgroup) AddProcess(pid int) error {
	// Check if we're using cgroups v2
	if cg.Path != "" {
		procsFile := filepath.Join(cg.Path, "cgroup.procs")
		return os.WriteFile(procsFile, []byte(strconv.Itoa(pid)), 0644)
	}

	// For cgroups v1, add process to each controller
	var lastErr error
	for _, ctrl := range cg.Controllers {
		cgPath := filepath.Join("/sys/fs/cgroup", string(ctrl), cg.Name)
		procsFile := filepath.Join(cgPath, "cgroup.procs")
		if err := os.WriteFile(procsFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// SetResourceLimits applies the specified resource limits to the cgroup
func (cg *Cgroup) SetResourceLimits(limits ResourceLimits) error {
	// Apply CPU limits
	if err := cg.applyCpuLimits(limits); err != nil {
		return err
	}

	// Apply memory limits
	if err := cg.applyMemoryLimits(limits); err != nil {
		return err
	}

	// Apply pids limits
	if limits.PidsLimit > 0 {
		if err := cg.applyPidsLimits(limits); err != nil {
			return err
		}
	}

	return nil
}

func (cg *Cgroup) applyCpuLimits(limits ResourceLimits) error {
	// Check if we're using cgroups v2
	if cg.Path != "" {
		// Set CPU weight (shares equivalent in cgroups v2)
		if limits.CpuShares > 0 {
			// Convert from shares to weight (1-10000)
			weight := 1 + ((limits.CpuShares-2)*9999)/262142
			if weight == 0 {
				weight = 1
			}
			if err := os.WriteFile(
				filepath.Join(cg.Path, "cpu.weight"),
				[]byte(strconv.FormatUint(weight, 10)),
				0644,
			); err != nil {
				return fmt.Errorf("failed to set cpu weight: %v", err)
			}
		}

		// Set CPU quota and period
		if limits.CpuQuota > 0 {
			maxVal := limits.CpuQuota
			periodVal := limits.CpuPeriod

			if periodVal == 0 {
				periodVal = 100000 // 100ms default
			}

			// Format: "max quota period"
			maxStr := fmt.Sprintf("%d %d", maxVal, periodVal)
			if err := os.WriteFile(
				filepath.Join(cg.Path, "cpu.max"),
				[]byte(maxStr),
				0644,
			); err != nil {
				return fmt.Errorf("failed to set cpu.max: %v", err)
			}
		}

		return nil
	}

	// For cgroups v1
	cpuCgPath := filepath.Join("/sys/fs/cgroup", "cpu", cg.Name)

	// Set CPU shares
	if limits.CpuShares > 0 {
		if err := os.WriteFile(
			filepath.Join(cpuCgPath, "cpu.shares"),
			[]byte(strconv.FormatUint(limits.CpuShares, 10)),
			0644,
		); err != nil {
			return fmt.Errorf("failed to set cpu shares: %v", err)
		}
	}

	// Set CPU quota
	if limits.CpuQuota >= 0 {
		if err := os.WriteFile(
			filepath.Join(cpuCgPath, "cpu.cfs_quota_us"),
			[]byte(strconv.FormatInt(limits.CpuQuota, 10)),
			0644,
		); err != nil {
			return fmt.Errorf("failed to set cpu quota: %v", err)
		}
	}

	// Set CPU period
	if limits.CpuPeriod > 0 {
		if err := os.WriteFile(
			filepath.Join(cpuCgPath, "cpu.cfs_period_us"),
			[]byte(strconv.FormatUint(limits.CpuPeriod, 10)),
			0644,
		); err != nil {
			return fmt.Errorf("failed to set cpu period: %v", err)
		}
	}

	return nil
}

// applyMemoryLimits applies memory-specific limits
func (cg *Cgroup) applyMemoryLimits(limits ResourceLimits) error {
	// Check if we're using cgroups v2
	if cg.Path != "" {
		// Set memory limit
		if limits.MemoryLimit > 0 {
			if err := os.WriteFile(
				filepath.Join(cg.Path, "memory.max"),
				[]byte(strconv.FormatUint(limits.MemoryLimit, 10)),
				0644,
			); err != nil {
				return fmt.Errorf("failed to set memory.max: %v", err)
			}
		}

		// Set memory+swap limit
		if limits.MemorySwapLimit > 0 {
			if err := os.WriteFile(
				filepath.Join(cg.Path, "memory.swap.max"),
				[]byte(strconv.FormatUint(limits.MemorySwapLimit-limits.MemoryLimit, 10)),
				0644,
			); err != nil {
				// Swap limit may not be supported, ignore errors
				fmt.Printf("Warning: failed to set swap limit: %v\n", err)
			}
		}

		return nil
	}

	// For cgroups v1
	memCgPath := filepath.Join("/sys/fs/cgroup", "memory", cg.Name)

	// Set memory limit
	if limits.MemoryLimit > 0 {
		if err := os.WriteFile(
			filepath.Join(memCgPath, "memory.limit_in_bytes"),
			[]byte(strconv.FormatUint(limits.MemoryLimit, 10)),
			0644,
		); err != nil {
			return fmt.Errorf("failed to set memory limit: %v", err)
		}
	}

	// Set memory+swap limit
	if limits.MemorySwapLimit > 0 {
		if err := os.WriteFile(
			filepath.Join(memCgPath, "memory.memsw.limit_in_bytes"),
			[]byte(strconv.FormatUint(limits.MemorySwapLimit, 10)),
			0644,
		); err != nil {
			// Swap limit may not be supported, ignore errors
			fmt.Printf("Warning: failed to set swap limit: %v\n", err)
		}
	}

	return nil
}

// applyPidsLimits applies process count limits
func (cg *Cgroup) applyPidsLimits(limits ResourceLimits) error {
	// Check if we're using cgroups v2
	if cg.Path != "" {
		return os.WriteFile(
			filepath.Join(cg.Path, "pids.max"),
			[]byte(strconv.FormatInt(limits.PidsLimit, 10)),
			0644,
		)
	}

	// For cgroups v1
	pidsCgPath := filepath.Join("/sys/fs/cgroup", "pids", cg.Name)
	return os.WriteFile(
		filepath.Join(pidsCgPath, "pids.max"),
		[]byte(strconv.FormatInt(limits.PidsLimit, 10)),
		0644,
	)
}
