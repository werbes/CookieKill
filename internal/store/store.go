// Package store persists the single-world server's player profiles.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"cookiekill/internal/game"
)

type Store struct {
	path string
	mu   sync.Mutex
}

type document struct {
	Version  int                     `json:"version"`
	Profiles map[string]game.Profile `json:"profiles"`
}

func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(dir, "players.json")}, nil
}

func (s *Store) Load() (map[string]game.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]game.Profile), nil
	}
	if err != nil {
		return nil, err
	}
	var doc document
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("invalid save file %s (preserved): %w", s.path, err)
	}
	if doc.Version != 1 && doc.Version != 2 {
		return nil, fmt.Errorf("unsupported save version %d", doc.Version)
	}
	if doc.Profiles == nil {
		doc.Profiles = make(map[string]game.Profile)
	}
	return doc.Profiles, nil
}

// Save writes, syncs, and closes a temporary file before replacing the save.
// A failed encode/write never truncates the last good save.
func (s *Store) Save(profiles map[string]game.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Version 2 preserves the separate permanent, bag and hotbar containers.
	// Older servers must reject these saves rather than flattening the slots.
	b, err := json.Marshal(document{Version: 2, Profiles: profiles})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".players-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), s.path)
}
