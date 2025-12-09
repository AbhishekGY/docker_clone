package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/AbhishekGY/mydocker/pkg/api"
)

// httpServer holds the HTTP server instance
type httpServer struct {
	server *http.Server
}

var srv *httpServer

// Start starts the daemon HTTP server
func (d *Daemon) Start() error {
	// Remove old socket if it exists
	if err := os.RemoveAll(d.socketPath); err != nil {
		return fmt.Errorf("failed to remove old socket: %v", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create Unix socket: %v", err)
	}

	// Change socket permissions to allow access
	if err := os.Chmod(d.socketPath, 0666); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %v", err)
	}

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/containers/create", d.handleContainerCreate)
	mux.HandleFunc("/containers/list", d.handleContainerList)
	mux.HandleFunc("/containers/stop", d.handleContainerStop)

	// Create HTTP server
	srv = &httpServer{
		server: &http.Server{
			Handler: mux,
		},
	}

	fmt.Printf("Daemon listening on %s\n", d.socketPath)

	// Start serving (this blocks)
	if err := srv.server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %v", err)
	}

	return nil
}

// Stop gracefully stops the daemon
func (d *Daemon) Stop() error {
	fmt.Println("Shutting down daemon...")

	// Stop all running containers first
	d.stopAllContainers()

	// Then stop the HTTP server
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.server.Shutdown(ctx)
	}

	return nil
}

// handleContainerCreate handles container creation requests
func (d *Daemon) handleContainerCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req api.ContainerCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	id, runner, err := d.CreateContainer(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create container: %v", err), http.StatusInternalServerError)
		return
	}

	// If detached, just return the container ID
	if req.Detach {
		resp := api.ContainerCreateResponse{ID: id}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	// For attached mode, hijack the connection and stream I/O
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to hijack connection: %v", err), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Send container ID first as a JSON response
	resp := api.ContainerCreateResponse{ID: id}
	respBytes, _ := json.Marshal(resp)
	fmt.Fprintf(bufrw, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(respBytes), string(respBytes))
	bufrw.Flush()

	// Now stream I/O with the container's PTY
	if runner.GetPtyFile() == nil {
		fmt.Fprintln(bufrw, "Error: No PTY available for attached mode")
		bufrw.Flush()
		return
	}

	// Copy data bidirectionally between connection and PTY
	done := make(chan error, 2)

	// Copy from connection to PTY (stdin)
	go func() {
		_, err := io.Copy(runner.GetPtyFile(), conn)
		done <- err
	}()

	// Copy from PTY to connection (stdout/stderr)
	go func() {
		_, err := io.Copy(conn, runner.GetPtyFile())
		done <- err
	}()

	// Wait for either direction to finish
	<-done

	// Wait for container to exit
	runner.Wait()
}

// handleContainerList handles container listing requests
func (d *Daemon) handleContainerList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	containers := d.ListContainers()

	resp := api.ContainerListResponse{Containers: containers}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleContainerStop handles container stop requests
func (d *Daemon) handleContainerStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req api.ContainerStopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	err := d.StopContainer(req.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to stop container: %v", err), http.StatusInternalServerError)
		return
	}

	resp := api.ContainerStopResponse{Success: true}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
