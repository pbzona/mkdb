package database

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) string {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Temporarily override the module-level db path
	oldPath := dbPath

	// Initialize with test database
	db = nil
	err := initTestDatabase(dbPath)
	if err != nil {
		t.Fatalf("setupTestDB() failed: %v", err)
	}

	return oldPath
}

func cleanupTestDB(t *testing.T) {
	if db != nil {
		Close()
	}
}

// initTestDatabase initializes a test database
func initTestDatabase(path string) error {
	var err error
	db, err = sql.Open("sqlite", path)
	if err != nil {
		return err
	}

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
		username TEXT NOT NULL,
		password_hash TEXT NOT NULL,
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

	_, err = db.Exec(schema)
	return err
}

func TestCreateAndGetContainer(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	container := &Container{
		Name:        "mkdb-testdb",
		DisplayName: "testdb",
		Type:        "postgres",
		Version:     "15",
		ContainerID: "abc123",
		Port:        "5432",
		Status:      "running",
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
		VolumeType:  "named",
		VolumePath:  "testdb",
	}

	// Create container
	err := CreateContainer(container)
	if err != nil {
		t.Fatalf("CreateContainer() error = %v", err)
	}

	if container.ID == 0 {
		t.Error("CreateContainer() did not set container ID")
	}

	// Get container by name
	retrieved, err := GetContainer("mkdb-testdb")
	if err != nil {
		t.Fatalf("GetContainer() error = %v", err)
	}

	if retrieved.Name != container.Name {
		t.Errorf("GetContainer() Name = %v, want %v", retrieved.Name, container.Name)
	}

	if retrieved.DisplayName != container.DisplayName {
		t.Errorf("GetContainer() DisplayName = %v, want %v", retrieved.DisplayName, container.DisplayName)
	}

	if retrieved.Type != container.Type {
		t.Errorf("GetContainer() Type = %v, want %v", retrieved.Type, container.Type)
	}

	if retrieved.Port != container.Port {
		t.Errorf("GetContainer() Port = %v, want %v", retrieved.Port, container.Port)
	}
}

func TestGetContainerByID(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	container := &Container{
		Name:        "mkdb-testdb",
		DisplayName: "testdb",
		Type:        "mysql",
		Version:     "8",
		ContainerID: "xyz789",
		Port:        "3306",
		Status:      "running",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	err := CreateContainer(container)
	if err != nil {
		t.Fatalf("CreateContainer() error = %v", err)
	}

	retrieved, err := GetContainerByID(container.ID)
	if err != nil {
		t.Fatalf("GetContainerByID() error = %v", err)
	}

	if retrieved.ID != container.ID {
		t.Errorf("GetContainerByID() ID = %v, want %v", retrieved.ID, container.ID)
	}

	if retrieved.Name != container.Name {
		t.Errorf("GetContainerByID() Name = %v, want %v", retrieved.Name, container.Name)
	}
}

func TestListContainers(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// Create multiple containers
	containers := []*Container{
		{
			Name:        "mkdb-db1",
			DisplayName: "db1",
			Type:        "postgres",
			Version:     "15",
			Port:        "5432",
			Status:      "running",
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now().Add(24 * time.Hour),
		},
		{
			Name:        "mkdb-db2",
			DisplayName: "db2",
			Type:        "mysql",
			Version:     "8",
			Port:        "3306",
			Status:      "stopped",
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now().Add(24 * time.Hour),
		},
	}

	for _, c := range containers {
		if err := CreateContainer(c); err != nil {
			t.Fatalf("CreateContainer() error = %v", err)
		}
	}

	// List containers
	retrieved, err := ListContainers()
	if err != nil {
		t.Fatalf("ListContainers() error = %v", err)
	}

	if len(retrieved) != len(containers) {
		t.Errorf("ListContainers() returned %d containers, want %d", len(retrieved), len(containers))
	}
}

func TestUpdateContainer(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	container := &Container{
		Name:        "mkdb-testdb",
		DisplayName: "testdb",
		Type:        "postgres",
		Version:     "15",
		ContainerID: "abc123",
		Port:        "5432",
		Status:      "running",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	err := CreateContainer(container)
	if err != nil {
		t.Fatalf("CreateContainer() error = %v", err)
	}

	// Update status
	container.Status = "stopped"
	container.ExpiresAt = time.Now().Add(48 * time.Hour)

	err = UpdateContainer(container)
	if err != nil {
		t.Fatalf("UpdateContainer() error = %v", err)
	}

	// Retrieve and verify
	retrieved, err := GetContainer("mkdb-testdb")
	if err != nil {
		t.Fatalf("GetContainer() error = %v", err)
	}

	if retrieved.Status != "stopped" {
		t.Errorf("UpdateContainer() Status = %v, want stopped", retrieved.Status)
	}
}

func TestDeleteContainer(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	container := &Container{
		Name:        "mkdb-testdb",
		DisplayName: "testdb",
		Type:        "postgres",
		Version:     "15",
		Port:        "5432",
		Status:      "running",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	err := CreateContainer(container)
	if err != nil {
		t.Fatalf("CreateContainer() error = %v", err)
	}

	// Delete container
	err = DeleteContainer(container.ID)
	if err != nil {
		t.Fatalf("DeleteContainer() error = %v", err)
	}

	// Verify it's deleted
	_, err = GetContainer("mkdb-testdb")
	if err == nil {
		t.Error("GetContainer() expected error after deletion, got nil")
	}
}

func TestGetExpiredContainers(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	now := time.Now()

	// Create expired container
	expiredContainer := &Container{
		Name:        "mkdb-expired",
		DisplayName: "expired",
		Type:        "postgres",
		Version:     "15",
		Port:        "5432",
		Status:      "running",
		CreatedAt:   now.Add(-25 * time.Hour),
		ExpiresAt:   now.Add(-1 * time.Hour), // Expired 1 hour ago
	}

	// Create active container
	activeContainer := &Container{
		Name:        "mkdb-active",
		DisplayName: "active",
		Type:        "mysql",
		Version:     "8",
		Port:        "3306",
		Status:      "running",
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour), // Expires in 24 hours
	}

	if err := CreateContainer(expiredContainer); err != nil {
		t.Fatalf("CreateContainer() error = %v", err)
	}

	if err := CreateContainer(activeContainer); err != nil {
		t.Fatalf("CreateContainer() error = %v", err)
	}

	// Get expired containers
	expired, err := GetExpiredContainers()
	if err != nil {
		t.Fatalf("GetExpiredContainers() error = %v", err)
	}

	if len(expired) != 1 {
		t.Errorf("GetExpiredContainers() returned %d containers, want 1", len(expired))
	}

	if len(expired) > 0 && expired[0].Name != "mkdb-expired" {
		t.Errorf("GetExpiredContainers() returned wrong container: %s", expired[0].Name)
	}
}

func TestCreateAndGetUser(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// Create a container first
	container := &Container{
		Name:        "mkdb-testdb",
		DisplayName: "testdb",
		Type:        "postgres",
		Version:     "15",
		Port:        "5432",
		Status:      "running",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	err := CreateContainer(container)
	if err != nil {
		t.Fatalf("CreateContainer() error = %v", err)
	}

	// Create user
	user := &User{
		ContainerID:  container.ID,
		Username:     "testuser",
		PasswordHash: "encrypted_password",
		IsDefault:    true,
		CreatedAt:    time.Now(),
	}

	err = CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if user.ID == 0 {
		t.Error("CreateUser() did not set user ID")
	}

	// Get default user
	retrieved, err := GetDefaultUser(container.ID)
	if err != nil {
		t.Fatalf("GetDefaultUser() error = %v", err)
	}

	if retrieved.Username != user.Username {
		t.Errorf("GetDefaultUser() Username = %v, want %v", retrieved.Username, user.Username)
	}

	if retrieved.IsDefault != true {
		t.Error("GetDefaultUser() IsDefault = false, want true")
	}
}

func TestListUsers(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// Create a container
	container := &Container{
		Name:        "mkdb-testdb",
		DisplayName: "testdb",
		Type:        "postgres",
		Version:     "15",
		Port:        "5432",
		Status:      "running",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	err := CreateContainer(container)
	if err != nil {
		t.Fatalf("CreateContainer() error = %v", err)
	}

	// Create multiple users
	users := []*User{
		{
			ContainerID:  container.ID,
			Username:     "user1",
			PasswordHash: "hash1",
			IsDefault:    true,
			CreatedAt:    time.Now(),
		},
		{
			ContainerID:  container.ID,
			Username:     "user2",
			PasswordHash: "hash2",
			IsDefault:    false,
			CreatedAt:    time.Now(),
		},
	}

	for _, u := range users {
		if err := CreateUser(u); err != nil {
			t.Fatalf("CreateUser() error = %v", err)
		}
	}

	// List users
	retrieved, err := ListUsers(container.ID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(retrieved) != len(users) {
		t.Errorf("ListUsers() returned %d users, want %d", len(retrieved), len(users))
	}
}

func TestUpdateUser(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// Create container and user
	container := &Container{
		Name:        "mkdb-testdb",
		DisplayName: "testdb",
		Type:        "postgres",
		Version:     "15",
		Port:        "5432",
		Status:      "running",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	err := CreateContainer(container)
	if err != nil {
		t.Fatalf("CreateContainer() error = %v", err)
	}

	user := &User{
		ContainerID:  container.ID,
		Username:     "testuser",
		PasswordHash: "old_hash",
		IsDefault:    true,
		CreatedAt:    time.Now(),
	}

	err = CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Update password
	user.PasswordHash = "new_hash"
	err = UpdateUser(user)
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}

	// Retrieve and verify
	retrieved, err := GetDefaultUser(container.ID)
	if err != nil {
		t.Fatalf("GetDefaultUser() error = %v", err)
	}

	if retrieved.PasswordHash != "new_hash" {
		t.Errorf("UpdateUser() PasswordHash = %v, want new_hash", retrieved.PasswordHash)
	}
}

func TestDeleteUser(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// Create container and user
	container := &Container{
		Name:        "mkdb-testdb",
		DisplayName: "testdb",
		Type:        "postgres",
		Version:     "15",
		Port:        "5432",
		Status:      "running",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	err := CreateContainer(container)
	if err != nil {
		t.Fatalf("CreateContainer() error = %v", err)
	}

	user := &User{
		ContainerID:  container.ID,
		Username:     "testuser",
		PasswordHash: "hash",
		IsDefault:    false,
		CreatedAt:    time.Now(),
	}

	err = CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Delete user
	err = DeleteUser(user.ID)
	if err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	// Verify deletion
	users, err := ListUsers(container.ID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) != 0 {
		t.Errorf("ListUsers() returned %d users after deletion, want 0", len(users))
	}
}

func TestCreateEvent(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// Create container first
	container := &Container{
		Name:        "mkdb-testdb",
		DisplayName: "testdb",
		Type:        "postgres",
		Version:     "15",
		Port:        "5432",
		Status:      "running",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	err := CreateContainer(container)
	if err != nil {
		t.Fatalf("CreateContainer() error = %v", err)
	}

	// Create event
	event := &Event{
		ContainerID: container.ID,
		EventType:   "created",
		Timestamp:   time.Now(),
		Details:     "Test event",
	}

	err = CreateEvent(event)
	if err != nil {
		t.Fatalf("CreateEvent() error = %v", err)
	}
}
