package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/pbzona/mkdb/internal/config"
	_ "modernc.org/sqlite"
)

var db *sql.DB

// Container represents a database container
type Container struct {
	ID          int
	Name        string
	DisplayName string
	Type        string
	Version     string
	ContainerID string
	Port        string
	Status      string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	VolumeType  string
	VolumePath  string
}

// User represents a database user
type User struct {
	ID           int
	ContainerID  int
	Username     string
	PasswordHash string
	IsDefault    bool
	CreatedAt    time.Time
}

// Event represents a container event
type Event struct {
	ID          int
	ContainerID int
	EventType   string
	Timestamp   time.Time
	Details     string
}

// Initialize creates the database schema
func Initialize() error {
	var err error
	db, err = sql.Open("sqlite", config.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Create tables
	schema := `
	CREATE TABLE IF NOT EXISTS containers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		display_name TEXT NOT NULL,
		type TEXT NOT NULL,
		version TEXT NOT NULL,
		container_id TEXT,
		port TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL,
		volume_type TEXT,
		volume_path TEXT
	);

	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		container_id INTEGER NOT NULL,
		username TEXT,
		password_hash TEXT,
		is_default BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (container_id) REFERENCES containers(id) ON DELETE CASCADE,
		UNIQUE(container_id, username)
	);

	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		container_id INTEGER NOT NULL,
		event_type TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		details TEXT,
		FOREIGN KEY (container_id) REFERENCES containers(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_containers_status ON containers(status);
	CREATE INDEX IF NOT EXISTS idx_containers_expires_at ON containers(expires_at);
	CREATE INDEX IF NOT EXISTS idx_events_container_id ON events(container_id);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// CreateContainer creates a new container record
func CreateContainer(c *Container) error {
	result, err := db.Exec(`
		INSERT INTO containers (name, display_name, type, version, container_id, port, status, created_at, expires_at, volume_type, volume_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, c.Name, c.DisplayName, c.Type, c.Version, c.ContainerID, c.Port, c.Status, c.CreatedAt, c.ExpiresAt, c.VolumeType, c.VolumePath)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	c.ID = int(id)
	return nil
}

// GetContainer retrieves a container by name
func GetContainer(name string) (*Container, error) {
	c := &Container{}
	err := db.QueryRow(`
		SELECT id, name, display_name, type, version, container_id, port, status, created_at, expires_at, volume_type, volume_path
		FROM containers WHERE name = ?
	`, name).Scan(&c.ID, &c.Name, &c.DisplayName, &c.Type, &c.Version, &c.ContainerID, &c.Port, &c.Status, &c.CreatedAt, &c.ExpiresAt, &c.VolumeType, &c.VolumePath)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// GetContainerByDisplayName retrieves a container by display name
func GetContainerByDisplayName(displayName string) (*Container, error) {
	c := &Container{}
	err := db.QueryRow(`
		SELECT id, name, display_name, type, version, container_id, port, status, created_at, expires_at, volume_type, volume_path
		FROM containers WHERE display_name = ?
	`, displayName).Scan(&c.ID, &c.Name, &c.DisplayName, &c.Type, &c.Version, &c.ContainerID, &c.Port, &c.Status, &c.CreatedAt, &c.ExpiresAt, &c.VolumeType, &c.VolumePath)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// GetContainerByID retrieves a container by ID
func GetContainerByID(id int) (*Container, error) {
	c := &Container{}
	err := db.QueryRow(`
		SELECT id, name, display_name, type, version, container_id, port, status, created_at, expires_at, volume_type, volume_path
		FROM containers WHERE id = ?
	`, id).Scan(&c.ID, &c.Name, &c.DisplayName, &c.Type, &c.Version, &c.ContainerID, &c.Port, &c.Status, &c.CreatedAt, &c.ExpiresAt, &c.VolumeType, &c.VolumePath)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// ListContainers retrieves all containers (excluding cleaned up expired ones)
func ListContainers() ([]*Container, error) {
	return listContainersWithStatus(false)
}

// ListAllContainers retrieves all containers including expired ones
func ListAllContainers() ([]*Container, error) {
	return listContainersWithStatus(true)
}

// listContainersWithStatus retrieves containers, optionally including expired
func listContainersWithStatus(includeExpired bool) ([]*Container, error) {
	query := `
		SELECT id, name, display_name, type, version, container_id, port, status, created_at, expires_at, volume_type, volume_path
		FROM containers`

	if !includeExpired {
		query += ` WHERE status != 'expired'`
	}

	query += ` ORDER BY created_at DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var containers []*Container
	for rows.Next() {
		c := &Container{}
		if err := rows.Scan(&c.ID, &c.Name, &c.DisplayName, &c.Type, &c.Version, &c.ContainerID, &c.Port, &c.Status, &c.CreatedAt, &c.ExpiresAt, &c.VolumeType, &c.VolumePath); err != nil {
			return nil, err
		}
		containers = append(containers, c)
	}

	return containers, nil
}

// UpdateContainer updates a container record
func UpdateContainer(c *Container) error {
	_, err := db.Exec(`
		UPDATE containers
		SET container_id = ?, status = ?, expires_at = ?
		WHERE id = ?
	`, c.ContainerID, c.Status, c.ExpiresAt, c.ID)
	return err
}

// DeleteContainer deletes a container record
func DeleteContainer(id int) error {
	_, err := db.Exec("DELETE FROM containers WHERE id = ?", id)
	return err
}

// GetExpiredContainers retrieves containers that have expired
func GetExpiredContainers() ([]*Container, error) {
	rows, err := db.Query(`
		SELECT id, name, display_name, type, version, container_id, port, status, created_at, expires_at, volume_type, volume_path
		FROM containers WHERE expires_at < ? AND status != 'stopped' AND status != 'expired'
	`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var containers []*Container
	for rows.Next() {
		c := &Container{}
		if err := rows.Scan(&c.ID, &c.Name, &c.DisplayName, &c.Type, &c.Version, &c.ContainerID, &c.Port, &c.Status, &c.CreatedAt, &c.ExpiresAt, &c.VolumeType, &c.VolumePath); err != nil {
			return nil, err
		}
		containers = append(containers, c)
	}

	return containers, nil
}

// CreateUser creates a new user record
func CreateUser(u *User) error {
	result, err := db.Exec(`
		INSERT INTO users (container_id, username, password_hash, is_default, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, u.ContainerID, u.Username, u.PasswordHash, u.IsDefault, u.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	u.ID = int(id)
	return nil
}

// GetDefaultUser retrieves the default user for a container
func GetDefaultUser(containerID int) (*User, error) {
	u := &User{}
	err := db.QueryRow(`
		SELECT id, container_id, username, password_hash, is_default, created_at
		FROM users WHERE container_id = ? AND is_default = 1
	`, containerID).Scan(&u.ID, &u.ContainerID, &u.Username, &u.PasswordHash, &u.IsDefault, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ListUsers retrieves all users for a container
func ListUsers(containerID int) ([]*User, error) {
	rows, err := db.Query(`
		SELECT id, container_id, username, password_hash, is_default, created_at
		FROM users WHERE container_id = ?
	`, containerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.ContainerID, &u.Username, &u.PasswordHash, &u.IsDefault, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, nil
}

// UpdateUser updates a user record
func UpdateUser(u *User) error {
	_, err := db.Exec(`
		UPDATE users SET password_hash = ? WHERE id = ?
	`, u.PasswordHash, u.ID)
	return err
}

// DeleteUser deletes a user record
func DeleteUser(id int) error {
	_, err := db.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

// CreateEvent creates a new event record
func CreateEvent(e *Event) error {
	_, err := db.Exec(`
		INSERT INTO events (container_id, event_type, timestamp, details)
		VALUES (?, ?, ?, ?)
	`, e.ContainerID, e.EventType, e.Timestamp, e.Details)
	return err
}
