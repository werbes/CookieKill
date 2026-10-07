package auth

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Each generated record is under 300 bytes. Bound the read independently of
// record validation so an unexpectedly large file cannot exhaust memory.
const maxSessionFileBytes = maxSessions * 512

type sessionDocument struct {
	Version  int             `json:"version"`
	Sessions []storedSession `json:"sessions"`
}

type storedSession struct {
	Digest  string    `json:"digest"`
	Player  Player    `json:"player"`
	Expires time.Time `json:"expires"`
}

// loadSessions runs during construction, before the service is shared. Invalid
// files are left untouched and never partially applied to the live session map.
func (s *Service) loadSessions() error {
	if s.cfg.SessionFile == "" {
		return nil
	}
	f, err := os.Open(s.cfg.SessionFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read session file: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxSessionFileBytes+1))
	if err != nil {
		return fmt.Errorf("read session file: %w", err)
	}
	invalid := func(err error) error {
		return fmt.Errorf("invalid session file %s (preserved): %w", s.cfg.SessionFile, err)
	}
	if len(b) > maxSessionFileBytes {
		return invalid(errors.New("file exceeds size limit"))
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	var doc sessionDocument
	if err := decoder.Decode(&doc); err != nil {
		return invalid(err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return invalid(errors.New("unexpected content after session document"))
	}
	if doc.Version != 1 {
		return invalid(fmt.Errorf("unsupported version %d", doc.Version))
	}
	if doc.Sessions == nil {
		return invalid(errors.New("missing sessions array"))
	}
	if len(doc.Sessions) > maxSessions {
		return invalid(errors.New("too many sessions"))
	}
	now := s.now()
	sessions := make(map[[32]byte]session, len(doc.Sessions))
	seen := make(map[[32]byte]bool, len(doc.Sessions))
	for _, record := range doc.Sessions {
		digest, err := validateStoredSession(record)
		if err != nil {
			return invalid(err)
		}
		if seen[digest] {
			return invalid(errors.New("duplicate session digest"))
		}
		seen[digest] = true
		if now.Before(record.Expires) {
			sessions[digest] = session{player: record.Player, expires: record.Expires, persistent: true}
		}
	}
	s.sessions = sessions
	return nil
}

func validateStoredSession(record storedSession) ([32]byte, error) {
	var digest [32]byte
	decoded, err := hex.DecodeString(record.Digest)
	if err != nil || len(decoded) != len(digest) {
		return digest, errors.New("invalid session digest")
	}
	copy(digest[:], decoded)
	if len(record.Player.ID) != 34 || record.Player.ID[:2] != "u_" {
		return digest, errors.New("invalid session player ID")
	}
	if _, err := hex.DecodeString(record.Player.ID[2:]); err != nil {
		return digest, errors.New("invalid session player ID")
	}
	if len(record.Player.Name) < 1 || len(record.Player.Name) > 18 {
		return digest, errors.New("invalid session player name")
	}
	for _, c := range record.Player.Name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return digest, errors.New("invalid session player name")
		}
	}
	if record.Expires.IsZero() {
		return digest, errors.New("missing session expiry")
	}
	return digest, nil
}

// saveSessionsLocked writes a proposed map before the caller commits it in
// memory. Only remembered sessions are saved; the file never contains tokens
// or email addresses. The caller holds s.mu throughout the change.
func (s *Service) saveSessionsLocked(sessions map[[32]byte]session) error {
	if s.cfg.SessionFile == "" {
		return nil
	}
	now := s.now()
	doc := sessionDocument{Version: 1, Sessions: make([]storedSession, 0)}
	for digest, sess := range sessions {
		if !sess.persistent || !now.Before(sess.expires) {
			continue
		}
		record := storedSession{Digest: hex.EncodeToString(digest[:]), Player: sess.player, Expires: sess.expires}
		if _, err := validateStoredSession(record); err != nil {
			return err
		}
		doc.Sessions = append(doc.Sessions, record)
		if len(doc.Sessions) > maxSessions {
			return errors.New("too many sessions")
		}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	if len(b) > maxSessionFileBytes {
		return errors.New("session file exceeds size limit")
	}
	if err := os.MkdirAll(filepath.Dir(s.cfg.SessionFile), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.cfg.SessionFile), ".sessions-*.tmp")
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
	return os.Rename(f.Name(), s.cfg.SessionFile)
}
