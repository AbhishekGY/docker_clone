package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Client represents a client for communicating with the daemon
type Client struct {
	socketPath string
	httpClient *http.Client
}

// NewClient creates a new client that communicates over a Unix socket
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
			Timeout: 30 * time.Second,
		},
	}
}

// CreateContainer creates a new container and returns its ID
func (c *Client) CreateContainer(req ContainerCreateRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := c.httpClient.Post("http://unix/containers/create", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var createResp ContainerCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	return createResp.ID, nil
}

// ListContainers returns a list of all containers
func (c *Client) ListContainers() ([]ContainerInfo, error) {
	resp, err := c.httpClient.Get("http://unix/containers/list")
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var listResp ContainerListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return listResp.Containers, nil
}

// StopContainer stops a container by ID
func (c *Client) StopContainer(id string) error {
	req := ContainerStopRequest{ID: id}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := c.httpClient.Post("http://unix/containers/stop", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var stopResp ContainerStopResponse
	if err := json.NewDecoder(resp.Body).Decode(&stopResp); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	if !stopResp.Success {
		return fmt.Errorf("failed to stop container")
	}

	return nil
}
