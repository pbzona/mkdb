package cmd

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/pbzona/mkdb/internal/docker"
)

func TestResolvePortExplicitConflict(t *testing.T) {
	port, cleanup := reserveTestPort(t)
	defer cleanup()

	_, err := resolvePort(port, "5432", true)
	if err == nil {
		t.Fatal("resolvePort() expected an error for an explicit port conflict")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("port %s is already in use", port)) {
		t.Fatalf("resolvePort() error = %v, want explicit port conflict", err)
	}
}

func TestResolvePortImplicitConflictAdvances(t *testing.T) {
	port, cleanup := reserveTestPort(t)
	defer cleanup()

	got, err := resolvePort(port, "5432", false)
	if err != nil {
		t.Fatalf("resolvePort() error = %v", err)
	}
	if got == port {
		t.Fatalf("resolvePort() returned occupied port %s", got)
	}
}

func TestCreateContainerWithPortRetriesImplicitConflict(t *testing.T) {
	port, cleanup := reserveTestPort(t)
	defer cleanup()

	var attempts []string
	id, gotPort, err := createContainerWithPort(port, false, func(port string) (string, error) {
		attempts = append(attempts, port)
		if len(attempts) == 1 {
			return "", &docker.PortConflictError{Port: port, Err: errors.New("port is already allocated")}
		}
		return "container-id", nil
	})
	if err != nil {
		t.Fatalf("createContainerWithPort() error = %v", err)
	}
	if id != "container-id" {
		t.Fatalf("container ID = %q, want container-id", id)
	}
	if len(attempts) != 2 {
		t.Fatalf("createContainerWithPort() made %d attempts, want 2", len(attempts))
	}
	if gotPort != attempts[1] || attempts[0] == attempts[1] {
		t.Fatalf("ports attempted = %v, returned port = %s", attempts, gotPort)
	}
}

func TestCreateContainerWithPortDoesNotRetryExplicitConflict(t *testing.T) {
	var attempts int
	_, _, err := createContainerWithPort("5432", true, func(port string) (string, error) {
		attempts++
		return "", &docker.PortConflictError{Port: port, Err: errors.New("port is already allocated")}
	})
	if err == nil {
		t.Fatal("createContainerWithPort() expected an explicit port conflict")
	}
	if attempts != 1 {
		t.Fatalf("createContainerWithPort() made %d attempts, want 1", attempts)
	}
	if !strings.Contains(err.Error(), "port 5432 is already in use") {
		t.Fatalf("createContainerWithPort() error = %v, want explicit port conflict", err)
	}
}

func TestNextPort(t *testing.T) {
	got, err := nextPort("5432")
	if err != nil || got != "5433" {
		t.Fatalf("nextPort(5432) = %q, %v; want 5433", got, err)
	}

	if _, err := nextPort("65535"); err == nil {
		t.Fatal("nextPort(65535) expected an error")
	}
}

func reserveTestPort(t *testing.T) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("reserve test port: %v", err)
	}
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	return port, func() { _ = listener.Close() }
}
