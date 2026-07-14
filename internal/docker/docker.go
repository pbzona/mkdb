package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/pbzona/mkdb/internal/adapters"
	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/types"
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

// Initialize creates a Docker client. It does not contact the daemon, so
// commands that only read local state (version, ls, info) work while Docker is
// down; commands that need the daemon surface a clear error on first use.
func Initialize() error {
	var err error
	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
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

// GetDBConfig returns the configuration for a database type, or an error if the
// type is unknown.
func GetDBConfig(dbType, version string) (*DBConfig, error) {
	registry := adapters.GetRegistry()
	adapter, err := registry.Get(dbType)
	if err != nil {
		return nil, err
	}

	return &DBConfig{
		Image:       adapter.GetImage(version),
		DefaultPort: adapter.GetDefaultPort(),
	}, nil
}

// IsPortAvailable reports whether a TCP port can be bound on the host. This
// catches both Docker-published ports and any other local process.
func IsPortAvailable(port string) (bool, error) {
	if _, err := strconv.Atoi(strings.TrimSpace(port)); err != nil {
		return false, fmt.Errorf("invalid port %q", port)
	}

	ln, err := net.Listen("tcp", net.JoinHostPort("", port))
	if err != nil {
		// Port is in use or otherwise not bindable.
		return false, nil
	}
	_ = ln.Close()
	return true, nil
}

// FindAvailablePort finds the next available port starting from startPort.
func FindAvailablePort(startPort string) (string, error) {
	basePort, err := strconv.Atoi(strings.TrimSpace(startPort))
	if err != nil {
		return "", fmt.Errorf("invalid port %q", startPort)
	}

	const maxAttempts = 100
	for i := 0; i < maxAttempts; i++ {
		port := strconv.Itoa(basePort + i)
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
func CreateContainer(dbType, displayName, username, password, port, volumeType, volumePath, version string) (string, error) {
	ctx := context.Background()

	dbConfig, err := GetDBConfig(dbType, version)
	if err != nil {
		return "", err
	}
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

// StartContainer starts an existing (stopped) container
func StartContainer(containerID string) error {
	ctx := context.Background()

	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	config.Logger.Info("Container started", "id", containerID[:12])
	return nil
}

// ContainerExists checks if a container exists
func ContainerExists(containerID string) bool {
	ctx := context.Background()

	_, err := cli.ContainerInspect(ctx, containerID)
	return err == nil
}

// RemoveVolume removes the on-disk data for a container's volume. mkdb "named"
// volumes are bind-mounted directories under the data dir; bind mounts point at
// a user-supplied path and are left untouched.
func RemoveVolume(volumeType, volumePath string) error {
	if volumeType != types.VolumeTypeNamed || volumePath == "" {
		return nil
	}

	dir := filepath.Join(config.VolumesDir, volumePath)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to remove volume directory %s: %w", dir, err)
	}
	config.Logger.Info("Volume directory removed", "path", dir)
	return nil
}

// ExecCommand runs a command in a container and returns its combined,
// demultiplexed stdout/stderr output.
func ExecCommand(containerID string, cmd []string) (string, error) {
	ctx := context.Background()

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Demultiplex the Docker stream to strip the 8-byte frame headers.
	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, resp.Reader); err != nil {
		return "", fmt.Errorf("failed to read output: %w", err)
	}

	output := strings.TrimSpace(stdout.String() + stderr.String())

	// Wait for completion and check exit code.
	for {
		inspect, err := cli.ContainerExecInspect(ctx, execID.ID)
		if err != nil {
			return output, err
		}
		if !inspect.Running {
			if inspect.ExitCode != 0 {
				return output, fmt.Errorf("command exited with code %d", inspect.ExitCode)
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return output, nil
}

// ExecCommandStdin runs a command in a container, writing stdin to it, and
// returns the combined, demultiplexed stdout/stderr output. It is used to feed
// schema/seed scripts to a database client (e.g. psql, mysql).
func ExecCommandStdin(containerID string, cmd []string, stdin []byte) (string, error) {
	ctx := context.Background()

	execID, err := cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Write the script to the command's stdin, then signal EOF.
	if _, err := resp.Conn.Write(stdin); err != nil {
		return "", fmt.Errorf("failed to write stdin: %w", err)
	}
	if err := resp.CloseWrite(); err != nil {
		return "", fmt.Errorf("failed to close stdin: %w", err)
	}

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, resp.Reader); err != nil {
		return "", fmt.Errorf("failed to read output: %w", err)
	}
	output := strings.TrimSpace(stdout.String() + stderr.String())

	inspect, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return output, err
	}
	if inspect.ExitCode != 0 {
		return output, fmt.Errorf("command exited with code %d: %s", inspect.ExitCode, output)
	}
	return output, nil
}

// Ping verifies that the Docker daemon is reachable.
func Ping(ctx context.Context) error {
	if cli == nil {
		return fmt.Errorf("docker client not initialized")
	}
	if _, err := cli.Ping(ctx); err != nil {
		return fmt.Errorf("cannot reach Docker daemon: %w", err)
	}
	return nil
}

// ManagedContainer is a summary of a Docker container created by mkdb.
type ManagedContainer struct {
	ID    string
	Name  string // mkdb display name (from the mkdb.name label)
	State string // Docker state, e.g. "running", "exited"
}

// ListManaged returns all Docker containers labeled as mkdb-managed, running or
// not. It is used by `mkdb doctor` to detect drift between Docker and mkdb's
// own state.
func ListManaged(ctx context.Context) ([]ManagedContainer, error) {
	f := filters.NewArgs()
	f.Add("label", labelManaged+"=true")

	list, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	managed := make([]ManagedContainer, 0, len(list))
	for _, c := range list {
		managed = append(managed, ManagedContainer{
			ID:    c.ID,
			Name:  c.Labels[labelName],
			State: c.State,
		})
	}
	return managed, nil
}

// CreateUser creates a new user in the database. admin* are the privileged
// credentials used to authenticate the operation.
func CreateUser(containerID, dbType, adminUser, adminPassword, username, password, dbName string) error {
	registry := adapters.GetRegistry()
	adapter, err := registry.Get(dbType)
	if err != nil {
		return fmt.Errorf("failed to get adapter: %w", err)
	}

	cmd := adapter.CreateUserCommand(adminUser, adminPassword, username, password, dbName)
	if cmd == nil {
		return fmt.Errorf("user creation not supported for %s", dbType)
	}

	_, err = ExecCommand(containerID, cmd)
	return err
}

// DeleteUser deletes a user from the database.
func DeleteUser(containerID, dbType, adminUser, adminPassword, username, dbName string) error {
	registry := adapters.GetRegistry()
	adapter, err := registry.Get(dbType)
	if err != nil {
		return fmt.Errorf("failed to get adapter: %w", err)
	}

	cmd := adapter.DeleteUserCommand(adminUser, adminPassword, username, dbName)
	if cmd == nil {
		return fmt.Errorf("user deletion not supported for %s", dbType)
	}

	_, err = ExecCommand(containerID, cmd)
	return err
}

// RotatePassword rotates a user's password.
func RotatePassword(containerID, dbType, adminUser, adminPassword, username, newPassword, dbName string) error {
	registry := adapters.GetRegistry()
	adapter, err := registry.Get(dbType)
	if err != nil {
		return fmt.Errorf("failed to get adapter: %w", err)
	}

	cmd := adapter.RotatePasswordCommand(adminUser, adminPassword, username, newPassword, dbName)
	if cmd == nil {
		return fmt.Errorf("password rotation not supported for %s", dbType)
	}

	_, err = ExecCommand(containerID, cmd)
	return err
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
