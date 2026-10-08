package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"cookiekill/internal/game"
)

func TestLegacySaveUpgradesWithoutLosingInventory(t *testing.T) {
	db, _ := New(t.TempDir())
	legacy := `{"version":1,"profiles":{"alice":{"id":"alice","name":"Alice","coins":321,"inventory":{"sugar":10,"berry_cookie":6,"nut_cookie":4,"cactus_cookie":3,"salt_cookie":2,"dough":7},"protected":["sugar","berry_cookie","nut_cookie","cactus_cookie","salt_cookie"],"selected":"sugar","bakery":true,"bakeryLevel":2}}}`
	if err := os.WriteFile(db.legacyPath, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	old, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	w := game.New(old)
	w.Join("alice", "Alice")
	profiles := w.Profiles()
	p := profiles["alice"]
	if p.Coins != 321 || p.Inventory["sugar"] != 10 || p.Inventory["salt_cookie"] != 2 || p.Inventory["dough"] != 7 || !p.Bakery || p.BakeryPlot != "" || p.BakeryLevel != 2 {
		t.Fatal("legacy progress lost")
	}
	if err := db.Save(profiles); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(db.dir, "alice.json"))
	if err != nil {
		t.Fatal(err)
	}
	var d playerDocument
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if d.Version != 3 || d.Profile == nil || d.Profile.ID != "alice" {
		t.Fatal("slot save not versioned")
	}
	restored, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(restored["alice"].SafeSlots) != 0 || restored["alice"].BagSlots != p.BagSlots || restored["alice"].Hotbar != p.Hotbar {
		t.Fatal("canonical slot layout changed on reload")
	}
}

func TestVersionTwoSafeSlotsMigrateBeforeVersionThreeSave(t *testing.T) {
	db, _ := New(t.TempDir())
	bag := make([]game.Stack, 15)
	for i := range bag {
		bag[i] = game.Stack{Item: "wood", Count: 100}
	}
	legacy := map[string]any{"version": 2, "profiles": map[string]any{"alice": map[string]any{
		"id": "alice", "inventoryVersion": 2, "bagSlots": bag,
		"safeSlots": []game.Stack{{Item: "cake", Count: 12}, {Item: "pearl", Count: 7}, {Item: "wood", Count: 40}},
		"hotbar":    []game.Stack{{Item: "sugar", Count: 10}},
	}}}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(db.legacyPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	w := game.New(loaded)
	profiles := w.Profiles()
	p := profiles["alice"]
	if p.InventoryVersion != 3 || len(p.BagSlots) != 20 || len(p.SafeSlots) != 0 || p.Inventory["wood"] != 1540 || p.Inventory["cake"] != 12 || p.Inventory["pearl"] != 7 || p.Inventory["sugar"] != 10 {
		t.Fatal("version-two inventory lost protected items during migration")
	}
	if err := db.Save(profiles); err != nil {
		t.Fatal(err)
	}
	loaded, err = db.Load()
	if err != nil || loaded["alice"].BagSlots != p.BagSlots || loaded["alice"].Hotbar != p.Hotbar || len(loaded["alice"].SafeSlots) != 0 {
		t.Fatal("version-three save did not preserve migrated slots")
	}
}

func TestSaveReloadAndReplace(t *testing.T) {
	db, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	initial, err := db.Load()
	if err != nil || len(initial) != 0 {
		t.Fatalf("empty store: %v", err)
	}
	w := game.New(initial)
	w.Join("alice", "Alice")
	if err = db.Save(w.Profiles()); err != nil {
		t.Fatal(err)
	}
	w.Join("bob", "Bob")
	if err = db.Save(w.Profiles()); err != nil {
		t.Fatal("replace save", err)
	}
	saved, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 2 || saved["alice"].Inventory["sugar"] != 10 {
		t.Fatalf("progress lost: %+v", saved)
	}
	files, _ := filepath.Glob(filepath.Join(db.dir, ".player-*.tmp"))
	if len(files) != 0 {
		t.Fatal("temporary save not cleaned up")
	}
}

func TestInvalidSaveFailsWithoutOverwriting(t *testing.T) {
	db, _ := New(t.TempDir())
	for _, body := range []string{"{broken", `{"version":999,"profiles":{}}`} {
		if err := os.WriteFile(db.legacyPath, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Load(); err == nil {
			t.Fatal("bad save silently accepted")
		}
		unchanged, _ := os.ReadFile(db.legacyPath)
		if string(unchanged) != body {
			t.Fatal("bad save overwritten")
		}
	}
}

func TestPlayersSavedSeparatelyAndOnlyChangedFilesReplaced(t *testing.T) {
	dir := t.TempDir()
	db, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := game.New(nil)
	w.Join("alice", "Alice")
	w.Join("bob", "Bob")
	profiles := w.Profiles()
	if err := db.Save(profiles); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(db.dir)
	if err != nil || len(files) != 2 || files[0].Name() != "alice.json" || files[1].Name() != "bob.json" {
		t.Fatalf("expected one file per player: %v, %v", files, err)
	}
	if _, err := os.Stat(db.legacyPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new saves must not create a combined file: %v", err)
	}
	bobPath := filepath.Join(db.dir, "bob.json")
	bobBefore, err := os.ReadFile(bobPath)
	if err != nil {
		t.Fatal(err)
	}
	oldTime := time.Unix(1000000000, 0)
	if err := os.Chtimes(bobPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	alice := profiles["alice"]
	alice.Coins += 100
	profiles["alice"] = alice
	if err := db.Save(profiles); err != nil {
		t.Fatal(err)
	}
	bobAfter, err := os.ReadFile(bobPath)
	if err != nil || !bytes.Equal(bobBefore, bobAfter) {
		t.Fatalf("saving Alice changed Bob's file: %v", err)
	}
	info, err := os.Stat(bobPath)
	if err != nil || !info.ModTime().Equal(oldTime) {
		t.Fatalf("unchanged player was rewritten: %v", err)
	}
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Load()
	if err != nil {
		t.Fatalf("per-player round trip lost progress: %v", err)
	}
	want, err := json.Marshal(profiles)
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(loaded)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("per-player round trip changed persisted fields: %v", err)
	}
	delete(profiles, "bob")
	if err := reopened.Save(profiles); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(bobPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed profile remains on disk: %v", err)
	}
	if err := reopened.Save(nil); err != nil {
		t.Fatal(err)
	}
	loaded, err = reopened.Load()
	if err != nil || len(loaded) != 0 {
		t.Fatalf("empty snapshot resurrected players: %v, %v", loaded, err)
	}
}

func TestLegacyMigrationResumesAndPreservesBackup(t *testing.T) {
	for _, version := range []int{1, 2, 3} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			dir := t.TempDir()
			db, err := New(dir)
			if err != nil {
				t.Fatal(err)
			}
			legacy, err := json.Marshal(document{Version: version, Profiles: map[string]game.Profile{
				"alice": {ID: "alice", Coins: 12}, "bob": {Coins: 34},
			}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(db.legacyPath, legacy, 0600); err != nil {
				t.Fatal(err)
			}
			// Simulate an interruption after Alice's new progress reached disk.
			alicePath := filepath.Join(db.dir, "alice.json")
			if err := os.WriteFile(alicePath, []byte(`{"version":3,"profile":{"id":"alice","coins":56}}`), 0600); err != nil {
				t.Fatal(err)
			}
			previousBackup := filepath.Join(dir, "players.json.previous.bak")
			if err := os.WriteFile(previousBackup, []byte("previous backup"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(db.dir, ".player-interrupted.tmp"), []byte("{broken"), 0600); err != nil {
				t.Fatal(err)
			}
			loaded, err := db.Load()
			if err != nil || len(loaded) != 2 || loaded["alice"].Coins != 56 || loaded["bob"].ID != "bob" || loaded["bob"].Coins != 34 {
				t.Fatalf("interrupted migration lost progress: %v, %v", loaded, err)
			}
			if err := db.Save(loaded); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(db.legacyPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("legacy save was not retired: %v", err)
			}
			backups, err := filepath.Glob(filepath.Join(dir, "players.json.*.bak"))
			if err != nil || len(backups) != 2 {
				t.Fatalf("migration did not preserve both backups: %v, %v", backups, err)
			}
			for _, path := range backups {
				want := legacy
				if path == previousBackup {
					want = []byte("previous backup")
				}
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("backup changed: %s, %v", path, err)
				}
			}
			// Removing a migrated player must not resurrect the legacy record.
			if err := os.Remove(alicePath); err != nil {
				t.Fatal(err)
			}
			reopened, err := New(dir)
			if err != nil {
				t.Fatal(err)
			}
			loaded, err = reopened.Load()
			if err != nil || len(loaded) != 1 || loaded["bob"].Coins != 34 {
				t.Fatalf("archived legacy data was loaded again: %v, %v", loaded, err)
			}
		})
	}
}

func TestInvalidPlayerSaveFailsWithoutOverwriting(t *testing.T) {
	for _, body := range []string{
		"{broken", `{"version":999,"profile":{"id":"alice"}}`,
		`{"version":3}`, `{"version":3,"profile":null}`,
		`{"version":3,"profile":{"id":"bob"}}`,
		`{"version":3,"profile":{"id":"../alice"}}`,
	} {
		t.Run(body, func(t *testing.T) {
			db, err := New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(db.dir, "alice.json")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			loaded, err := db.Load()
			if err == nil || loaded != nil || !strings.Contains(err.Error(), path) {
				t.Fatalf("bad player save accepted or not identified: %v, %v", loaded, err)
			}
			unchanged, err := os.ReadFile(path)
			if err != nil || string(unchanged) != body {
				t.Fatalf("bad player save overwritten: %v", err)
			}
		})
	}
}

func TestInvalidPlayerIDsCannotCreateFiles(t *testing.T) {
	for _, id := range []string{"", "../escape", `..\escape`, "/escape", `C:\escape`, "alice/bob", "Alice", "alice.", "con", "com1", strings.Repeat("a", 129)} {
		t.Run(id, func(t *testing.T) {
			db, err := New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Save(map[string]game.Profile{id: {ID: id}}); err == nil {
				t.Fatalf("invalid player ID accepted: %q", id)
			}
			entries, err := os.ReadDir(db.dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid ID changed saved files: %v", err)
			}
		})
	}
}

func TestFailedEncodePreservesAllPlayersAndLegacySave(t *testing.T) {
	db, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	profiles := map[string]game.Profile{"alice": {ID: "alice", Coins: 1}, "bob": {ID: "bob", Coins: 2}}
	if err := db.Save(profiles); err != nil {
		t.Fatal(err)
	}
	legacy, err := json.Marshal(document{Version: 3, Profiles: profiles})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(db.legacyPath, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	broken := map[string]game.Profile{"alice": {ID: "alice", Coins: 100}, "bob": {ID: "bob", Home: &game.Home{X: math.NaN()}}}
	if err := db.Save(broken); err == nil {
		t.Fatal("unencodable profile accepted")
	}
	loaded, err := db.Load()
	if err != nil || !reflect.DeepEqual(loaded, profiles) {
		t.Fatalf("failed save changed player data: %v", err)
	}
	unchanged, err := os.ReadFile(db.legacyPath)
	if err != nil || !bytes.Equal(unchanged, legacy) {
		t.Fatalf("failed migration changed legacy save: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(db.dir, ".player-*.tmp"))
	if err != nil || len(files) != 0 {
		t.Fatalf("failed save leaked temporary files: %v", err)
	}
}
