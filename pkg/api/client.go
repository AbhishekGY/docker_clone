package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"
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
	// For detached mode, use simple HTTP request
	if req.Detach {
		return c.createDetachedContainer(req)
	}

	// For attached mode, handle interactive I/O
	return c.createAttachedContainer(req)
}

// createDetachedContainer creates a container in detached mode
func (c *Client) createDetachedContainer(req ContainerCreateRequest) (string, error) {
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

// createAttachedContainer creates a container in attached mode with interactive I/O
func (c *Client) createAttachedContainer(req ContainerCreateRequest) (string, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Connect to Unix socket
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return "", fmt.Errorf("failed to connect to daemon: %v", err)
	}
	defer conn.Close()

	// Send HTTP request
	httpReq := fmt.Sprintf("POST /containers/create HTTP/1.1\r\nHost: unix\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(body), string(body))
	if _, err := conn.Write([]byte(httpReq)); err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}

	// Read HTTP response headers and container ID
	respBuf := make([]byte, 4096)
	n, err := conn.Read(respBuf)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response to extract container ID
	var createResp ContainerCreateResponse
	respStr := string(respBuf[:n])

	// Find the JSON body (after \r\n\r\n)
	bodyStart := bytes.Index(respBuf[:n], []byte("\r\n\r\n"))
	if bodyStart == -1 {
		return "", fmt.Errorf("invalid response format")
	}
	bodyStart += 4 // Skip the \r\n\r\n

	if err := json.Unmarshal(respBuf[bodyStart:n], &createResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %v (response: %s)", err, respStr)
	}

	// Put terminal in raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("failed to set terminal to raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Copy I/O bidirectionally
	done := make(chan error, 2)

	// Copy stdin to connection
	go func() {
		_, err := io.Copy(conn, os.Stdin)
		done <- err
	}()

	// Copy connection to stdout
	go func() {
		_, err := io.Copy(os.Stdout, conn)
		done <- err
	}()

	// Wait for signals or I/O completion
	select {
	case <-sigChan:
		// Signal received, connection will be closed by defer
	case <-done:
		// I/O completed
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
