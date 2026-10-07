package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"cookiekill/internal/game"
)

func TestLegacySaveUpgradesWithoutLosingInventory(t *testing.T) {
	db, _ := New(t.TempDir())
	legacy := `{"version":1,"profiles":{"alice":{"id":"alice","name":"Alice","coins":321,"inventory":{"sugar":10,"berry_cookie":6,"nut_cookie":4,"cactus_cookie":3,"salt_cookie":2,"dough":7},"protected":["sugar","berry_cookie","nut_cookie","cactus_cookie","salt_cookie"],"selected":"sugar","bakery":true,"bakeryLevel":2}}}`
	if err := os.WriteFile(db.path, []byte(legacy), 0600); err != nil {
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
	raw, err := os.ReadFile(db.path)
	if err != nil {
		t.Fatal(err)
	}
	var d document
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if d.Version != 3 {
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
	if err := os.WriteFile(db.path, data, 0600); err != nil {
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
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(db.path), ".players-*.tmp"))
	if len(files) != 0 {
		t.Fatal("temporary save not cleaned up")
	}
}

func TestInvalidSaveFailsWithoutOverwriting(t *testing.T) {
	db, _ := New(t.TempDir())
	for _, body := range []string{"{broken", `{"version":999,"profiles":{}}`} {
		if err := os.WriteFile(db.path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Load(); err == nil {
			t.Fatal("bad save silently accepted")
		}
		unchanged, _ := os.ReadFile(db.path)
		if string(unchanged) != body {
			t.Fatal("bad save overwritten")
		}
	}
}
