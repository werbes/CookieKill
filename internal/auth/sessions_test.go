package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func storageService(path string, now time.Time) *Service {
	return &Service{
		cfg:      Config{DevMode: true, SessionFile: path},
		now:      func() time.Time { return now },
		sessions: make(map[[32]byte]session),
	}
}

func TestRememberedSessionStorageRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	path := filepath.Join(t.TempDir(), "private", "sessions.json")
	s := storageService(path, now)
	token := "a-raw-token-that-must-never-be-written"
	digest := sha256.Sum256([]byte(token))
	player := playerForEmail("baker@example.com")
	want := session{player: player, expires: now.Add(rememberedSessionTTL), persistent: true}
	s.sessions[digest] = want
	s.sessions[sha256.Sum256([]byte("temporary"))] = session{player: player, expires: now.Add(time.Hour)}
	s.sessions[sha256.Sum256([]byte("expired"))] = session{player: player, expires: now, persistent: true}
	if err := s.saveSessionsLocked(s.sessions); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(token)) || bytes.Contains(b, []byte("baker@example.com")) {
		t.Fatal("session storage exposed a token or email")
	}
	if !bytes.Contains(b, []byte(hex.EncodeToString(digest[:]))) {
		t.Fatal("session digest missing from storage")
	}
	if runtime.GOOS != "windows" {
		for _, target := range []struct {
			path string
			perm os.FileMode
		}{{path, 0600}, {filepath.Dir(path), 0700}} {
			info, err := os.Stat(target.path)
			if err != nil || info.Mode().Perm() != target.perm {
				t.Fatalf("private permissions missing for %s: %v, %v", target.path, info, err)
			}
		}
	}
	loaded, err := New(Config{DevMode: true, SessionFile: path})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := loaded.sessions[digest]
	if len(loaded.sessions) != 1 || !ok || got.player != want.player || !got.expires.Equal(want.expires) || !got.persistent {
		t.Fatalf("remembered session did not survive restart: %+v", loaded.sessions)
	}
	// Saving an empty map must replace the old file, so revocations also survive.
	if err := s.saveSessionsLocked(nil); err != nil {
		t.Fatal(err)
	}
	if err := loaded.loadSessions(); err != nil || len(loaded.sessions) != 0 {
		t.Fatalf("empty save retained sessions: %v, %+v", err, loaded.sessions)
	}
}

func TestSessionStorageDisabledOrMissing(t *testing.T) {
	for _, path := range []string{"", filepath.Join(t.TempDir(), "missing", "sessions.json")} {
		s := storageService(path, time.Now())
		if err := s.loadSessions(); err != nil || len(s.sessions) != 0 {
			t.Fatalf("missing storage failed: %v", err)
		}
	}
	s := storageService("", time.Now())
	// Disabled storage does not validate or write anything.
	if err := s.saveSessionsLocked(map[[32]byte]session{{}: {persistent: true, expires: time.Now().Add(time.Hour)}}); err != nil {
		t.Fatal(err)
	}
}

func TestSessionStorageSkipsExpiredRecords(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	path := filepath.Join(t.TempDir(), "sessions.json")
	s := storageService(path, now)
	digest := sha256.Sum256([]byte("remembered"))
	s.sessions[digest] = session{player: playerForEmail("baker@example.com"), expires: now.Add(time.Hour), persistent: true}
	if err := s.saveSessionsLocked(s.sessions); err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now.Add(time.Hour) }
	if err := s.loadSessions(); err != nil || len(s.sessions) != 0 {
		t.Fatalf("expired record was restored: %v, %+v", err, s.sessions)
	}
}

func TestInvalidSessionStorageIsPreserved(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	digest := sha256.Sum256([]byte("remembered"))
	record := storedSession{Digest: hex.EncodeToString(digest[:]), Player: playerForEmail("baker@example.com"), Expires: now.Add(time.Hour)}
	encode := func(records []storedSession) []byte {
		t.Helper()
		b, err := json.Marshal(sessionDocument{Version: 1, Sessions: records})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	valid := encode([]storedSession{record})
	tests := map[string][]byte{
		"empty":          {},
		"malformed":      []byte(`{"version":1,`),
		"version":        []byte(`{"version":2,"sessions":[]}`),
		"missing array":  []byte(`{"version":1}`),
		"null array":     []byte(`{"version":1,"sessions":null}`),
		"unknown field":  []byte(`{"version":1,"sessions":[],"secret":true}`),
		"trailing JSON":  append(append([]byte{}, valid...), []byte(`{}`)...),
		"bad digest":     []byte(strings.Replace(string(valid), record.Digest, "bad", 1)),
		"missing player": []byte(strings.Replace(string(valid), record.Player.ID, "", 1)),
		"email in name":  []byte(strings.Replace(string(valid), `"name":"baker"`, `"name":"baker@example.com"`, 1)),
		"duplicate":      encode([]storedSession{record, record}),
		"oversized":      bytes.Repeat([]byte(" "), maxSessionFileBytes+1),
	}
	missingExpiry := record
	missingExpiry.Expires = time.Time{}
	tests["missing expiry"] = encode([]storedSession{missingExpiry})
	tooMany := make([]storedSession, maxSessions+1)
	for i := range tooMany {
		tooMany[i] = record
	}
	tests["too many"] = encode(tooMany)
	for name, original := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sessions.json")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			s := storageService(path, now)
			previous := session{player: record.Player, expires: record.Expires, persistent: true}
			s.sessions[digest] = previous
			if err := s.loadSessions(); err == nil {
				t.Fatal("invalid file was accepted")
			}
			if len(s.sessions) != 1 || s.sessions[digest] != previous {
				t.Fatal("invalid load changed live sessions")
			}
			if _, err := New(Config{DevMode: true, SessionFile: path}); err == nil {
				t.Fatal("invalid session file did not fail startup")
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("invalid file was not preserved: %v", err)
			}
		})
	}
}

func TestFailedSessionSavePreservesPreviousFile(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	path := filepath.Join(t.TempDir(), "sessions.json")
	s := storageService(path, now)
	digest := sha256.Sum256([]byte("remembered"))
	s.sessions[digest] = session{player: playerForEmail("baker@example.com"), expires: now.Add(time.Hour), persistent: true}
	if err := s.saveSessionsLocked(s.sessions); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	invalid := s.sessions[digest]
	invalid.expires = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := s.saveSessionsLocked(map[[32]byte]session{digest: invalid}); err == nil {
		t.Fatal("unencodable session was accepted")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("failed save changed previous file: %v", err)
	}
	// A failed replacement also removes its private temporary file.
	s.cfg.SessionFile = filepath.Join(filepath.Dir(path), "directory")
	if err := os.Mkdir(s.cfg.SessionFile, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.saveSessionsLocked(s.sessions); err == nil {
		t.Fatal("save over a directory succeeded")
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".sessions-*.tmp"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("failed save left temporary files: %v, %v", leftovers, err)
	}
}
