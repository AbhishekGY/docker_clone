package api

// ContainerCreateRequest represents a request to create a new container
type ContainerCreateRequest struct {
	Image      string   `json:"image"`
	Command    []string `json:"command"`
	Rootfs     string   `json:"rootfs"`
	Memory     uint64   `json:"memory"`
	MemorySwap uint64   `json:"memory_swap"`
	CpuShares  uint64   `json:"cpu_shares"`
	CpuQuota   int64    `json:"cpu_quota"`
	CpuPeriod  uint64   `json:"cpu_period"`
	PidsLimit  int64    `json:"pids_limit"`
}

// ContainerCreateResponse represents the response after creating a container
type ContainerCreateResponse struct {
	ID string `json:"id"`
}

// ContainerInfo represents information about a container
type ContainerInfo struct {
	ID      string `json:"id"`
	Image   string `json:"image"`
	Command string `json:"command"`
	Status  string `json:"status"`
	Created int64  `json:"created"`
	PID     int    `json:"pid"`
}

// ContainerListResponse represents the response for listing containers
type ContainerListResponse struct {
	Containers []ContainerInfo `json:"containers"`
}

// ContainerStopRequest represents a request to stop a container
type ContainerStopRequest struct {
	ID string `json:"id"`
}

// ContainerStopResponse represents the response after stopping a container
type ContainerStopResponse struct {
	Success bool `json:"success"`
}
