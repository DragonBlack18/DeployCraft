// Package journal writes an append-only JSON Lines execution history.
// No payload bytes, serial numbers, environment variables or raw arguments
// may be placed in a journal event by callers.
package journal

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Event struct {
	Timestamp  time.Time        `json:"ts"`
	RunID      string           `json:"runId"`
	Event      string           `json:"event"`
	Package    string           `json:"package,omitempty"`
	Version    string           `json:"version,omitempty"`
	Action     string           `json:"action,omitempty"`
	Outcome    string           `json:"outcome,omitempty"`
	Reason     string           `json:"reason,omitempty"`
	Code       string           `json:"code,omitempty"`
	DurationMs int64            `json:"durationMs,omitempty"`
	ExitCode   *int             `json:"exitCode,omitempty"`
	Detail     string           `json:"detail,omitempty"`
	KitRoot    string           `json:"kit,omitempty"`
	Catalog    string           `json:"catalog,omitempty"`
	CatalogDigest string       `json:"catalogDigest,omitempty"`
	Selection  *StoredSelection `json:"selection,omitempty"`
	Options    *StoredOptions   `json:"options,omitempty"`
	Resumed    bool             `json:"resumed,omitempty"`
}
type Store struct {
	Root string
	mu   sync.Mutex
}

func New(root string) *Store { return &Store{Root: root} }
func NewRunID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func (s *Store) Append(ev Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ev.RunID == "" || ev.Event == "" {
		return fmt.Errorf("evento sem runId ou event")
	}
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}
	root := filepath.Join(s.Root, "state")
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	file := filepath.Join(root, "journal.jsonl")
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	if _, err := f.Write(line); err != nil {
		return err
	}
	return f.Sync()
}

type Snapshot struct {
	RunID     string            `json:"runId"`
	Status    string            `json:"status"`
	UpdatedAt time.Time         `json:"updatedAt"`
	Packages  map[string]string `json:"packages"`
}

func (s *Store) WriteSnapshot(snapshot Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	root := filepath.Join(s.Root, "state")
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = time.Now().UTC()
	}
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(root, ".state-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(encoded); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, filepath.Join(root, "state.json"))
}
func (s *Store) WriteReport(runID string, report any) error {
	path := filepath.Join(s.Root, "logs", runID)
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(path, ".report-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, filepath.Join(path, "report.json"))
}
