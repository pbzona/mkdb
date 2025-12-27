package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	// Setup: Initialize a temporary config
	setupTestConfig(t)
	defer cleanupTestConfig(t)

	tests := []struct {
		name      string
		plaintext string
	}{
		{"Simple password", "password123"},
		{"Complex password", "$uper$ecret!@#123"},
		{"Empty string", ""},
		{"Long string", strings.Repeat("a", 1000)},
		{"Special characters", "!@#$%^&*()_+-=[]{}|;:',.<>?/~`"},
		{"Unicode characters", "Hello 世界 🌍"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			encrypted, err := Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			if encrypted == "" {
				t.Error("Encrypt() returned empty string")
			}

			// Encrypted should be different from plaintext (except for empty string)
			if tt.plaintext != "" && encrypted == tt.plaintext {
				t.Error("Encrypt() returned plaintext unchanged")
			}

			// Decrypt
			decrypted, err := Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("Decrypt() = %v, want %v", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptDifferentOutputs(t *testing.T) {
	setupTestConfig(t)
	defer cleanupTestConfig(t)

	plaintext := "testpassword"
	encrypted1, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	encrypted2, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Each encryption should produce different ciphertext due to random nonce
	if encrypted1 == encrypted2 {
		t.Error("Encrypt() produces same output for same input (nonce not randomized)")
	}

	// But both should decrypt to the same plaintext
	decrypted1, _ := Decrypt(encrypted1)
	decrypted2, _ := Decrypt(encrypted2)

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("Decrypt() failed to decrypt different encrypted values to same plaintext")
	}
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	setupTestConfig(t)
	defer cleanupTestConfig(t)

	tests := []struct {
		name       string
		ciphertext string
	}{
		{"Invalid hex", "not-hex-string"},
		{"Too short", "abc123"},
		{"Empty string", ""},
		{"Random hex", "deadbeef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(tt.ciphertext)
			if err == nil {
				t.Error("Decrypt() expected error for invalid ciphertext, got nil")
			}
		})
	}
}

func TestInitializeCreatesDirectories(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tempDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	err := Initialize()
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Check that directories were created
	expectedDataDir := filepath.Join(tempDir, AppName)
	if _, err := os.Stat(expectedDataDir); os.IsNotExist(err) {
		t.Errorf("Initialize() did not create data directory: %s", expectedDataDir)
	}

	expectedVolumesDir := filepath.Join(expectedDataDir, "volumes")
	if _, err := os.Stat(expectedVolumesDir); os.IsNotExist(err) {
		t.Errorf("Initialize() did not create volumes directory: %s", expectedVolumesDir)
	}

	// Check that encryption key was created
	keyPath := filepath.Join(expectedDataDir, KeyFileName)
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Errorf("Initialize() did not create encryption key: %s", keyPath)
	}

	// Check that log file was created
	logPath := filepath.Join(expectedDataDir, LogFileName)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Initialize() did not create log file: %s", logPath)
	}

	// Check that global variables were set
	if DataDir == "" {
		t.Error("Initialize() did not set DataDir")
	}

	if DBPath == "" {
		t.Error("Initialize() did not set DBPath")
	}

	if LogPath == "" {
		t.Error("Initialize() did not set LogPath")
	}

	if VolumesDir == "" {
		t.Error("Initialize() did not set VolumesDir")
	}

	if Logger == nil {
		t.Error("Initialize() did not set Logger")
	}
}

func TestEncryptionKeyPersistence(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tempDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	// Initialize first time
	err := Initialize()
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Encrypt something
	plaintext := "testpassword"
	encrypted, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Reset the encryption key variable to simulate a restart
	encryptionKey = nil

	// Initialize again (should load existing key)
	err = Initialize()
	if err != nil {
		t.Fatalf("Initialize() second time error = %v", err)
	}

	// Should be able to decrypt with loaded key
	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt() after reload error = %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Decrypt() after reload = %v, want %v", decrypted, plaintext)
	}
}

func TestConstants(t *testing.T) {
	if AppName != "mkdb" {
		t.Errorf("AppName = %v, want mkdb", AppName)
	}

	if DBFileName != "mkdb.db" {
		t.Errorf("DBFileName = %v, want mkdb.db", DBFileName)
	}

	if LogFileName != "mkdb.log" {
		t.Errorf("LogFileName = %v, want mkdb.log", LogFileName)
	}

	if KeyFileName != ".encryption.key" {
		t.Errorf("KeyFileName = %v, want .encryption.key", KeyFileName)
	}
}

// Helper functions

func setupTestConfig(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tempDir)

	err := Initialize()
	if err != nil {
		t.Fatalf("setupTestConfig() failed: %v", err)
	}
}

func cleanupTestConfig(t *testing.T) {
	os.Unsetenv("XDG_DATA_HOME")
	encryptionKey = nil
	DataDir = ""
	DBPath = ""
	LogPath = ""
	VolumesDir = ""
	Logger = nil
}
