package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/pbzona/mkdb/internal/adapters"
	"github.com/pbzona/mkdb/internal/config"
)

const (
	containerPrefix = "mkdb-"
	labelManaged    = "mkdb.managed"
	labelType       = "mkdb.type"
	labelName       = "mkdb.name"
)

var cli *client.Client

// DBConfig represents database-specific configuration
type DBConfig struct {
	Image       string
	DefaultPort string
	EnvVars     map[string]string
}

// Initialize creates a Docker client
func Initialize() error {
	var err error
	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Test connection
	ctx := context.Background()
	if _, err := cli.Ping(ctx); err != nil {
		return fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}

	return nil
}

// Close closes the Docker client
func Close() error {
	if cli != nil {
		return cli.Close()
	}
	return nil
}

// GetDBConfig returns the configuration for a database type
func GetDBConfig(dbType, version string) *DBConfig {
	registry := adapters.GetRegistry()
	adapter, err := registry.Get(dbType)
	if err != nil {
		// Return nil if adapter not found
		return nil
	}

	return &DBConfig{
		Image:       adapter.GetImage(version),
		DefaultPort: adapter.GetDefaultPort(),
	}
}

// IsPortAvailable checks if a port is available on the host
func IsPortAvailable(port string) (bool, error) {
	ctx := context.Background()

	// List all containers
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return false, err
	}

	portNum := uint16(mustAtoi(port))

	// Check if any container is using this port
	for _, c := range containers {
		for _, p := range c.Ports {
			if p.PublicPort == portNum {
				return false, nil
			}
		}
	}

	return true, nil
}

// FindAvailablePort finds the next available port starting from the default port
// Returns the available port as a string
func FindAvailablePort(startPort string) (string, error) {
	basePort := mustAtoi(startPort)
	maxAttempts := 100 // Check up to 100 ports

	for i := 0; i < maxAttempts; i++ {
		port := fmt.Sprintf("%d", basePort+i)
		available, err := IsPortAvailable(port)
		if err != nil {
			return "", err
		}
		if available {
			return port, nil
		}
	}

	return "", fmt.Errorf("no available ports found in range %d-%d", basePort, basePort+maxAttempts)
}

// CreateContainer creates and starts a database container
func CreateContainer(dbType, displayName, username, password, port, volumeType, volumePath string) (string, error) {
	ctx := context.Background()

	dbConfig := GetDBConfig(dbType, "")
	containerName := containerPrefix + displayName

	// Pull image if not exists
	config.Logger.Info("Pulling image", "image", dbConfig.Image)
	reader, err := cli.ImagePull(ctx, dbConfig.Image, image.PullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader)

	// Get adapter for this database type
	registry := adapters.GetRegistry()
	adapter, err := registry.Get(dbType)
	if err != nil {
		return "", fmt.Errorf("failed to get adapter: %w", err)
	}

	// Prepare environment variables
	env := adapter.GetEnvVars(displayName, username, password)

	// Prepare port bindings
	exposedPorts := nat.PortSet{
		nat.Port(dbConfig.DefaultPort + "/tcp"): struct{}{},
	}
	portBindings := nat.PortMap{
		nat.Port(dbConfig.DefaultPort + "/tcp"): []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: port,
			},
		},
	}

	// Prepare volume mounts
	var mounts []mount.Mount
	if volumeType != "" && volumePath != "" {
		mounts = append(mounts, createMount(adapter, volumeType, volumePath))
	}

	// Always add config mount for all databases
	configMount, err := createConfigMount(adapter, displayName)
	if err != nil {
		return "", fmt.Errorf("failed to create config mount: %w", err)
	}
	mounts = append(mounts, configMount)

	// Get custom command args if needed (e.g., for Redis password)
	cmdArgs := adapter.GetCommandArgs(password)

	// Create container
	containerConfig := &container.Config{
		Image:        dbConfig.Image,
		Env:          env,
		ExposedPorts: exposedPorts,
		Labels: map[string]string{
			labelManaged: "true",
			labelType:    dbType,
			labelName:    displayName,
		},
	}

	// Set custom command if provided
	if len(cmdArgs) > 0 {
		containerConfig.Cmd = cmdArgs
	}

	resp, err := cli.ContainerCreate(ctx, containerConfig, &container.HostConfig{
		PortBindings: portBindings,
		Mounts:       mounts,
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	config.Logger.Info("Container created", "id", resp.ID[:12], "name", displayName)
	return resp.ID, nil
}

// createMount creates a mount configuration
func createMount(adapter adapters.DatabaseAdapter, volumeType, volumePath string) mount.Mount {
	target := adapter.GetDataPath()

	if volumeType == "bind" {
		return mount.Mount{
			Type:   mount.TypeBind,
			Source: volumePath,
			Target: target,
		}
	}

	// Named volume (stored in XDG_DATA_HOME/mkdb/volumes)
	return mount.Mount{
		Type:   mount.TypeBind,
		Source: filepath.Join(config.VolumesDir, volumePath),
		Target: target,
	}
}

// GetConfigFileName returns the main config file name for the database type
func GetConfigFileName(dbType string) string {
	registry := adapters.GetRegistry()
	adapter, err := registry.Get(dbType)
	if err != nil {
		return "config"
	}
	return adapter.GetConfigFileName()
}

// createConfigMount creates a mount for config files in XDG_DATA_HOME
func createConfigMount(adapter adapters.DatabaseAdapter, displayName string) (mount.Mount, error) {
	// Create config directory in XDG_DATA_HOME/mkdb/configs/<dbname>
	configDir := filepath.Join(config.DataDir, "configs", displayName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return mount.Mount{}, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create default config file if it doesn't exist
	configFile := filepath.Join(configDir, adapter.GetConfigFileName())
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err := createDefaultConfig(adapter, configFile); err != nil {
			return mount.Mount{}, fmt.Errorf("failed to create default config: %w", err)
		}
	}

	return mount.Mount{
		Type:   mount.TypeBind,
		Source: configDir,
		Target: adapter.GetConfigPath(),
	}, nil
}

// createDefaultConfig creates a default config file for the database type
func createDefaultConfig(adapter adapters.DatabaseAdapter, configFile string) error {
	content := adapter.GetDefaultConfig()
	return os.WriteFile(configFile, []byte(content), 0644)
}

// StopContainer stops a container gracefully
func StopContainer(containerID string) error {
	ctx := context.Background()

	timeout := 10
	if err := cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	config.Logger.Info("Container stopped", "id", containerID[:12])
	return nil
}

// RemoveContainer removes a container
func RemoveContainer(containerID string) error {
	ctx := context.Background()

	if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	config.Logger.Info("Container removed", "id", containerID[:12])
	return nil
}

// RestartContainer restarts a container
func RestartContainer(containerID string) error {
	ctx := context.Background()

	timeout := 10
	if err := cli.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to restart container: %w", err)
	}

	config.Logger.Info("Container restarted", "id", containerID[:12])
	return nil
}

// StartContainer starts an existing container
func StartContainer(containerID string) error {
	ctx := context.Background()

	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	config.Logger.Info("Container started", "id", containerID[:12])
	return nil
}

// GetContainerStatus returns the status of a container
func GetContainerStatus(containerID string) (string, error) {
	ctx := context.Background()

	info, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}

	return info.State.Status, nil
}

// ContainerExists checks if a container exists
func ContainerExists(containerID string) bool {
	ctx := context.Background()

	_, err := cli.ContainerInspect(ctx, containerID)
	return err == nil
}

// RemoveVolume removes a volume
func RemoveVolume(volumePath string) error {
	ctx := context.Background()

	// For bind mounts, we don't remove through Docker
	// For named volumes, remove the directory
	filter := filters.NewArgs()
	filter.Add("name", volumePath)

	volumes, err := cli.VolumeList(ctx, volume.ListOptions{Filters: filter})
	if err != nil {
		return err
	}

	for _, vol := range volumes.Volumes {
		if err := cli.VolumeRemove(ctx, vol.Name, true); err != nil {
			return err
		}
	}

	return nil
}

// ExecInContainer executes a command in a running container
func ExecInContainer(containerID string, cmd []string) error {
	ctx := context.Background()

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec: %w", err)
	}

	if err := cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to start exec: %w", err)
	}

	// Wait for the exec to complete
	for {
		inspect, err := cli.ContainerExecInspect(ctx, execID.ID)
		if err != nil {
			return err
		}
		if !inspect.Running {
			if inspect.ExitCode != 0 {
				return fmt.Errorf("command exited with code %d", inspect.ExitCode)
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// CreateUser creates a new user in the database
func CreateUser(containerID, dbType, username, password, dbName string) error {
	registry := adapters.GetRegistry()
	adapter, err := registry.Get(dbType)
	if err != nil {
		return fmt.Errorf("failed to get adapter: %w", err)
	}

	cmd := adapter.CreateUserCommand(username, password, dbName)
	if cmd == nil {
		return fmt.Errorf("user creation not supported for %s", dbType)
	}

	return ExecInContainer(containerID, cmd)
}

// DeleteUser deletes a user from the database
func DeleteUser(containerID, dbType, username, dbName string) error {
	registry := adapters.GetRegistry()
	adapter, err := registry.Get(dbType)
	if err != nil {
		return fmt.Errorf("failed to get adapter: %w", err)
	}

	cmd := adapter.DeleteUserCommand(username, dbName)
	if cmd == nil {
		return fmt.Errorf("user deletion not supported for %s", dbType)
	}

	return ExecInContainer(containerID, cmd)
}

// RotatePassword rotates a user's password
func RotatePassword(containerID, dbType, username, newPassword, dbName string) error {
	registry := adapters.GetRegistry()
	adapter, err := registry.Get(dbType)
	if err != nil {
		return fmt.Errorf("failed to get adapter: %w", err)
	}

	cmd := adapter.RotatePasswordCommand(username, newPassword, dbName)
	if cmd == nil {
		return fmt.Errorf("password rotation not supported for %s", dbType)
	}

	return ExecInContainer(containerID, cmd)
}

// ExecCommand executes a command in a container and returns the output
func ExecCommand(containerName string, cmd []string) (string, error) {
	ctx := context.Background()

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := cli.ContainerExecCreate(ctx, containerName, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Read the output
	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to read output: %w", err)
	}

	// Wait for completion and check exit code
	for {
		inspect, err := cli.ContainerExecInspect(ctx, execID.ID)
		if err != nil {
			return string(output), err
		}
		if !inspect.Running {
			if inspect.ExitCode != 0 {
				return string(output), fmt.Errorf("command exited with code %d", inspect.ExitCode)
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return string(output), nil
}

func mustAtoi(s string) int {
	i, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return i
}

// GetActualVersion retrieves the actual database version from a running container
func GetActualVersion(containerID, dbType string) (string, error) {
	registry := adapters.GetRegistry()
	adapter, err := registry.Get(dbType)
	if err != nil {
		return "", fmt.Errorf("failed to get adapter: %w", err)
	}

	// Get the version command for this database type
	versionCmd := adapter.GetVersionCommand()
	if versionCmd == nil || len(versionCmd) == 0 {
		return "", fmt.Errorf("version detection not supported for %s", dbType)
	}

	// Execute the version command in the container
	output, err := ExecCommand(containerID, versionCmd)
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}

	// Parse the version output using the adapter
	version := adapter.ParseVersion(output)
	return version, nil
}
