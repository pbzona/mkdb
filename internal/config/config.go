package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
)

const (
	AppName     = "mkdb"
	DBFileName  = "mkdb.db"
	LogFileName = "mkdb.log"
	KeyFileName = ".encryption.key"
)

var (
	DataDir       string
	DBPath        string
	LogPath       string
	VolumesDir    string
	Logger        *log.Logger
	encryptionKey []byte
)

// Initialize sets up the configuration directories and logger
func Initialize() error {
	// Get XDG_DATA_HOME or use default
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataHome = filepath.Join(homeDir, ".local", "share")
	}

	// Set up data directory
	DataDir = filepath.Join(dataHome, AppName)
	if err := os.MkdirAll(DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Set up volumes directory
	VolumesDir = filepath.Join(DataDir, "volumes")
	if err := os.MkdirAll(VolumesDir, 0755); err != nil {
		return fmt.Errorf("failed to create volumes directory: %w", err)
	}

	DBPath = filepath.Join(DataDir, DBFileName)
	LogPath = filepath.Join(DataDir, LogFileName)

	// Initialize logger
	logFile, err := os.OpenFile(LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	Logger = log.NewWithOptions(io.MultiWriter(os.Stdout, logFile), log.Options{
		ReportTimestamp: true,
		TimeFormat:      "2006-01-02 15:04:05",
		Prefix:          "mkdb",
	})
	Logger.SetLevel(log.InfoLevel)

	// Initialize or load encryption key
	if err := initEncryptionKey(); err != nil {
		return fmt.Errorf("failed to initialize encryption key: %w", err)
	}

	return nil
}

// initEncryptionKey creates or loads the encryption key for password storage
func initEncryptionKey() error {
	keyPath := filepath.Join(DataDir, KeyFileName)

	// Check if key exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		// Generate new key
		key := make([]byte, 32) // AES-256
		if _, err := rand.Read(key); err != nil {
			return fmt.Errorf("failed to generate encryption key: %w", err)
		}

		// Save key to file with restricted permissions
		if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(key)), 0600); err != nil {
			return fmt.Errorf("failed to save encryption key: %w", err)
		}

		encryptionKey = key
	} else {
		// Load existing key
		keyHex, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("failed to read encryption key: %w", err)
		}

		key, err := hex.DecodeString(string(keyHex))
		if err != nil {
			return fmt.Errorf("failed to decode encryption key: %w", err)
		}

		encryptionKey = key
	}

	return nil
}

// Encrypt encrypts plaintext using AES-GCM
func Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-GCM
func Decrypt(ciphertext string) (string, error) {
	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, encryptedData := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
