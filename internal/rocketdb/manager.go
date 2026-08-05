package rocketdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	pb "github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	_ "modernc.org/sqlite"
)

var ErrInvalidName = errors.New("db name must be lowercase letters, digits, underscore or dash, 3-40 chars")
var ErrNotFound = errors.New("database not found")

type Manager struct {
	app        *pb.PocketBase
	root       string
	publicHost string
	tunnelURL  string

	mu   sync.Mutex
	open map[string]*sql.DB
}

func NewManager(app *pb.PocketBase, root string) (*Manager, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Manager{app: app, root: root, open: make(map[string]*sql.DB)}, nil
}

func (m *Manager) SetPublicHost(host string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publicHost = strings.TrimRight(host, "/")
}

// SetTunnelURL is called by the tunnel supervisor when a new URL is discovered.
// The tunnel URL takes precedence over publicHost when rendering connection info.
func (m *Manager) SetTunnelURL(u string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tunnelURL = strings.TrimRight(u, "/")
}

// PublicHost returns the tunnel URL if known, otherwise the configured host.
func (m *Manager) PublicHost() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tunnelURL != "" {
		return m.tunnelURL
	}
	return m.publicHost
}

// TunnelURL returns just the tunnel URL (empty if none).
func (m *Manager) TunnelURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tunnelURL
}

func (m *Manager) Root() string { return m.root }

func (m *Manager) DBPath(name string) string {
	return filepath.Join(m.root, name+".sqlite")
}

func ValidName(name string) bool {
	if len(name) < 3 || len(name) > 40 {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

// Create provisions a new SQLite file and records metadata.
func (m *Manager) Create(ctx context.Context, name, description, ownerID string) (*core.Record, error) {
	if !ValidName(name) {
		return nil, ErrInvalidName
	}
	name = strings.ToLower(name)

	col, err := m.app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		return nil, err
	}

	existing, _ := m.app.FindFirstRecordByFilter(collectionName, "name = {:n}", map[string]any{"n": name})
	if existing != nil {
		return nil, fmt.Errorf("database %q already exists", name)
	}

	record := core.NewRecord(col)
	record.Set("name", name)
	record.Set("description", description)
	if ownerID != "" {
		record.Set("owner_id", ownerID)
	}
	apiKey := "encomdb_" + randomHex(24)
	record.Set("api_key", apiKey)
	record.Set("status", "pending")

	if err := m.app.Save(record); err != nil {
		return nil, err
	}

	if err := m.provisionFile(ctx, name); err != nil {
		record.Set("status", "error")
		record.Set("error_message", err.Error())
		_ = m.app.Save(record)
		return record, err
	}

	size := int64(0)
	if info, statErr := os.Stat(m.DBPath(name)); statErr == nil {
		size = info.Size()
	}
	record.Set("status", "ready")
	record.Set("size_bytes", size)
	if err := m.app.Save(record); err != nil {
		return record, err
	}
	return record, nil
}

// Delete removes the SQLite file and the metadata record.
func (m *Manager) Delete(ctx context.Context, name string) error {
	m.close(name)
	path := m.DBPath(name)
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")

	record, err := m.app.FindFirstRecordByFilter(collectionName, "name = {:n}", map[string]any{"n": name})
	if err != nil {
		return ErrNotFound
	}
	return m.app.Delete(record)
}

// Open lazily opens (and caches) the sql.DB for a managed database.
func (m *Manager) Open(name string) (*sql.DB, error) {
	if !ValidName(name) {
		return nil, ErrInvalidName
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if db, ok := m.open[name]; ok {
		return db, nil
	}
	path := m.DBPath(name)
	if _, err := os.Stat(path); err != nil {
		return nil, ErrNotFound
	}
	db, err := openSQLite(path)
	if err != nil {
		return nil, err
	}
	m.open[name] = db
	return db, nil
}

func (m *Manager) close(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if db, ok := m.open[name]; ok {
		_ = db.Close()
		delete(m.open, name)
	}
}

// CloseAll releases every cached handle.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, db := range m.open {
		_ = db.Close()
		delete(m.open, name)
	}
}

// provisionFile creates a fresh SQLite file with WAL and a marker table.
func (m *Manager) provisionFile(ctx context.Context, name string) error {
	path := m.DBPath(name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists at %s", path)
	}
	db, err := openSQLite(path)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _encomdb_meta (k TEXT PRIMARY KEY, v TEXT)`); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx,
		`INSERT OR IGNORE INTO _encomdb_meta(k, v) VALUES ('created_at', datetime('now'))`)
	return err
}

func openSQLite(path string) (*sql.DB, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ConnectionInfo returns everything a client needs to talk to this DB.
type ConnectionInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	APIKey      string `json:"api_key"`
	SQLEndpoint string `json:"sql_endpoint"`
	Host        string `json:"host"`
	CurlExample string `json:"curl_example"`
	SizeBytes   int64  `json:"size_bytes"`
	Status      string `json:"status"`
}

func (m *Manager) ConnectionInfo(record *core.Record) ConnectionInfo {
	name := record.GetString("name")
	host := m.PublicHost()
	if host == "" {
		host = "http://localhost:8090"
	}
	endpoint := fmt.Sprintf("%s/api/rocketdb/dbs/%s/sql", host, name)
	apiKey := record.GetString("api_key")
	curl := fmt.Sprintf("curl -X POST %s \\\n  -H \"Authorization: Bearer %s\" \\\n  -H \"Content-Type: application/json\" \\\n  -d '{\"sql\":\"SELECT 1\"}'", endpoint, apiKey)
	size := int64(0)
	if info, err := os.Stat(m.DBPath(name)); err == nil {
		size = info.Size()
	} else {
		size = int64(record.GetInt("size_bytes"))
	}
	return ConnectionInfo{
		Name:        name,
		Description: record.GetString("description"),
		APIKey:      apiKey,
		SQLEndpoint: endpoint,
		Host:        host,
		CurlExample: curl,
		SizeBytes:   size,
		Status:      record.GetString("status"),
	}
}
