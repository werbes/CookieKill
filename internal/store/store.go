// Package store persists the single-world server's player profiles.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"cookiekill/internal/game"
)

type Store struct {
	dir        string
	legacyPath string
	mu         sync.Mutex
}

// document is the legacy, combined save format. It is only read for migration.
type document struct {
	Version  int                     `json:"version"`
	Profiles map[string]game.Profile `json:"profiles"`
}

type playerDocument struct {
	Version int           `json:"version"`
	Profile *game.Profile `json:"profile"`
}

func New(dir string) (*Store, error) {
	playersDir := filepath.Join(dir, "players")
	if err := os.MkdirAll(playersDir, 0700); err != nil {
		return nil, err
	}
	return &Store{dir: playersDir, legacyPath: filepath.Join(dir, "players.json")}, nil
}

func (s *Store) Load() (map[string]game.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profiles := make(map[string]game.Profile)
	b, err := os.ReadFile(s.legacyPath)
	if err == nil {
		var doc document
		if err := decodeSave(s.legacyPath, b, &doc, &doc.Version); err != nil {
			return nil, err
		}
		for id, profile := range doc.Profiles {
			if !validPlayerID(id) {
				return nil, fmt.Errorf("invalid save file %s (preserved): invalid player ID %q", s.legacyPath, id)
			}
			profile.ID = id // The legacy map key has always owned the identity.
			profiles[id] = profile
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	files, err := s.playerFiles()
	if err != nil {
		return nil, err
	}
	for _, name := range files {
		path := filepath.Join(s.dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var doc playerDocument
		if err := decodeSave(path, b, &doc, &doc.Version); err != nil {
			return nil, err
		}
		if doc.Profile == nil || doc.Profile.ID+".json" != name {
			return nil, fmt.Errorf("invalid save file %s (preserved): missing profile or mismatched player ID", path)
		}
		// A save interrupted during migration can leave both formats. The
		// individual file contains the newer progress for that player.
		profiles[doc.Profile.ID] = *doc.Profile
	}
	return profiles, nil
}

func decodeSave(path string, b []byte, doc any, version *int) error {
	if err := json.Unmarshal(b, doc); err != nil {
		return fmt.Errorf("invalid save file %s (preserved): %w", path, err)
	}
	if *version < 1 || *version > 3 {
		return fmt.Errorf("invalid save file %s (preserved): unsupported save version %d", path, *version)
	}
	return nil
}

// IDs are lower-case account identifiers, not display names. Keep filenames
// portable, without traversal, Windows device names, or case-folding collisions.
func validPlayerID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	switch id {
	case "con", "prn", "aux", "nul":
		return false
	}
	return !(len(id) == 4 && (strings.HasPrefix(id, "com") || strings.HasPrefix(id, "lpt")) && id[3] >= '1' && id[3] <= '9')
}

func (s *Store) playerFiles() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue // Ignore temporary files left by interrupted writes.
		}
		if !entry.Type().IsRegular() || !validPlayerID(strings.TrimSuffix(name, ".json")) {
			return nil, fmt.Errorf("invalid save file %s (preserved): expected a regular player file with a valid ID", filepath.Join(s.dir, name))
		}
		files = append(files, name)
	}
	return files, nil
}

// Save replaces each changed player's file using a synced temporary file.
// Each replacement is independent; an interrupted save may persist some players
// before others. Encoding errors are checked before any files are changed.
func (s *Store) Save(profiles map[string]game.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	encoded := make(map[string][]byte, len(profiles))
	names := make([]string, 0, len(profiles))
	for id, profile := range profiles {
		if !validPlayerID(id) {
			return fmt.Errorf("invalid player ID %q", id)
		}
		profile.ID = id
		// Version 3 has twenty protected bag slots and five hotbar slots.
		b, err := json.Marshal(playerDocument{Version: 3, Profile: &profile})
		if err != nil {
			return fmt.Errorf("encode player %s: %w", id, err)
		}
		name := id + ".json"
		encoded[name] = b
		names = append(names, name)
	}
	sort.Strings(names)
	previous, err := s.playerFiles()
	if err != nil {
		return err
	}
	for _, name := range names {
		path := filepath.Join(s.dir, name)
		old, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if bytes.Equal(old, encoded[name]) {
			continue
		}
		if err := writePlayer(path, encoded[name]); err != nil {
			return fmt.Errorf("save player file %s: %w", path, err)
		}
	}
	// Retire the combined file only after every player has been written. A
	// uniquely named backup preserves the original and any earlier backups.
	if err := s.archiveLegacy(); err != nil {
		return err
	}
	// Preserve Save's full-snapshot semantics, including removed profiles.
	for _, name := range previous {
		if _, keep := encoded[name]; !keep {
			if err := os.Remove(filepath.Join(s.dir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) archiveLegacy() error {
	if _, err := os.Stat(s.legacyPath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.legacyPath), "players.json.*.bak")
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return err
	}
	if err := os.Rename(s.legacyPath, f.Name()); err != nil {
		os.Remove(f.Name())
		return fmt.Errorf("archive legacy save: %w", err)
	}
	return nil
}

func writePlayer(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".player-*.tmp")
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
	return os.Rename(f.Name(), path)
}
