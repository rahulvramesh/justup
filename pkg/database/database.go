package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection
type DB struct {
	db *sql.DB
}

// SSHKey represents a stored SSH public key
type SSHKey struct {
	ID          string
	UserID      string
	Name        string
	PublicKey   string
	Fingerprint string
	CreatedAt   time.Time
	LastUsedAt  *time.Time
}

// Workspace represents stored workspace metadata
type Workspace struct {
	ID        string
	Name      string
	UserID    string
	GitURL    string
	Branch    string
	Image     string
	CPU       string
	Memory    string
	Storage   string
	EnableDinD bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Open opens or creates the SQLite database
func Open(path string) (*DB, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create tables
	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &DB{db: db}, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.db.Close()
}

// createTables creates the database schema
func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ssh_keys (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		name TEXT NOT NULL,
		public_key TEXT NOT NULL,
		fingerprint TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS workspaces (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		user_id TEXT NOT NULL,
		git_url TEXT NOT NULL,
		branch TEXT DEFAULT 'main',
		image TEXT DEFAULT 'justup/devcontainer:latest',
		cpu TEXT DEFAULT '1',
		memory TEXT DEFAULT '2Gi',
		storage TEXT DEFAULT '10Gi',
		enable_dind BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_ssh_keys_fingerprint ON ssh_keys(fingerprint);
	CREATE INDEX IF NOT EXISTS idx_ssh_keys_user_id ON ssh_keys(user_id);
	CREATE INDEX IF NOT EXISTS idx_workspaces_user_id ON workspaces(user_id);
	CREATE INDEX IF NOT EXISTS idx_workspaces_name ON workspaces(name);
	`

	_, err := db.Exec(schema)
	return err
}

// --- User operations ---

// CreateUser creates a new user
func (d *DB) CreateUser(id, username string) error {
	_, err := d.db.Exec(
		"INSERT INTO users (id, username) VALUES (?, ?)",
		id, username,
	)
	return err
}

// GetUser retrieves a user by ID
func (d *DB) GetUser(id string) (*User, error) {
	var user User
	err := d.db.QueryRow(
		"SELECT id, username, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetOrCreateDefaultUser gets or creates the default user for single-user mode
func (d *DB) GetOrCreateDefaultUser() (*User, error) {
	const defaultUserID = "default"
	const defaultUsername = "default"

	user, err := d.GetUser(defaultUserID)
	if err == nil {
		return user, nil
	}

	// Create default user
	if err := d.CreateUser(defaultUserID, defaultUsername); err != nil {
		return nil, err
	}

	return d.GetUser(defaultUserID)
}

// User represents a user in the system
type User struct {
	ID        string
	Username  string
	CreatedAt time.Time
}

// --- SSH Key operations ---

// AddSSHKey adds a new SSH key for a user
func (d *DB) AddSSHKey(key *SSHKey) error {
	_, err := d.db.Exec(
		`INSERT INTO ssh_keys (id, user_id, name, public_key, fingerprint, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		key.ID, key.UserID, key.Name, key.PublicKey, key.Fingerprint, key.CreatedAt,
	)
	return err
}

// GetSSHKeyByFingerprint retrieves an SSH key by its fingerprint
func (d *DB) GetSSHKeyByFingerprint(fingerprint string) (*SSHKey, error) {
	var key SSHKey
	var lastUsed sql.NullTime

	err := d.db.QueryRow(
		`SELECT id, user_id, name, public_key, fingerprint, created_at, last_used_at
		 FROM ssh_keys WHERE fingerprint = ?`,
		fingerprint,
	).Scan(&key.ID, &key.UserID, &key.Name, &key.PublicKey, &key.Fingerprint, &key.CreatedAt, &lastUsed)

	if err != nil {
		return nil, err
	}

	if lastUsed.Valid {
		key.LastUsedAt = &lastUsed.Time
	}

	return &key, nil
}

// ListSSHKeys lists all SSH keys for a user
func (d *DB) ListSSHKeys(userID string) ([]SSHKey, error) {
	rows, err := d.db.Query(
		`SELECT id, user_id, name, public_key, fingerprint, created_at, last_used_at
		 FROM ssh_keys WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []SSHKey
	for rows.Next() {
		var key SSHKey
		var lastUsed sql.NullTime

		if err := rows.Scan(&key.ID, &key.UserID, &key.Name, &key.PublicKey, &key.Fingerprint, &key.CreatedAt, &lastUsed); err != nil {
			return nil, err
		}

		if lastUsed.Valid {
			key.LastUsedAt = &lastUsed.Time
		}

		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// DeleteSSHKey deletes an SSH key
func (d *DB) DeleteSSHKey(id string) error {
	result, err := d.db.Exec("DELETE FROM ssh_keys WHERE id = ?", id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("key not found")
	}
	return nil
}

// DeleteSSHKeyByFingerprint deletes an SSH key by fingerprint
func (d *DB) DeleteSSHKeyByFingerprint(fingerprint string) error {
	result, err := d.db.Exec("DELETE FROM ssh_keys WHERE fingerprint = ?", fingerprint)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("key not found")
	}
	return nil
}

// UpdateSSHKeyLastUsed updates the last_used_at timestamp
func (d *DB) UpdateSSHKeyLastUsed(id string) error {
	_, err := d.db.Exec(
		"UPDATE ssh_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?",
		id,
	)
	return err
}

// --- Workspace operations ---

// SaveWorkspace saves workspace metadata
func (d *DB) SaveWorkspace(ws *Workspace) error {
	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO workspaces
		 (id, name, user_id, git_url, branch, image, cpu, memory, storage, enable_dind, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		ws.ID, ws.Name, ws.UserID, ws.GitURL, ws.Branch, ws.Image, ws.CPU, ws.Memory, ws.Storage, ws.EnableDinD, ws.CreatedAt,
	)
	return err
}

// GetWorkspace retrieves workspace metadata by name
func (d *DB) GetWorkspace(name string) (*Workspace, error) {
	var ws Workspace
	err := d.db.QueryRow(
		`SELECT id, name, user_id, git_url, branch, image, cpu, memory, storage, enable_dind, created_at, updated_at
		 FROM workspaces WHERE name = ?`,
		name,
	).Scan(&ws.ID, &ws.Name, &ws.UserID, &ws.GitURL, &ws.Branch, &ws.Image, &ws.CPU, &ws.Memory, &ws.Storage, &ws.EnableDinD, &ws.CreatedAt, &ws.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ws, nil
}

// ListWorkspaces lists all workspaces for a user
func (d *DB) ListWorkspaces(userID string) ([]Workspace, error) {
	rows, err := d.db.Query(
		`SELECT id, name, user_id, git_url, branch, image, cpu, memory, storage, enable_dind, created_at, updated_at
		 FROM workspaces WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []Workspace
	for rows.Next() {
		var ws Workspace
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.UserID, &ws.GitURL, &ws.Branch, &ws.Image, &ws.CPU, &ws.Memory, &ws.Storage, &ws.EnableDinD, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, ws)
	}

	return workspaces, rows.Err()
}

// DeleteWorkspace deletes workspace metadata
func (d *DB) DeleteWorkspace(name string) error {
	_, err := d.db.Exec("DELETE FROM workspaces WHERE name = ?", name)
	return err
}
