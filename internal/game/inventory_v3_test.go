package game

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestLegacySafeStacksMoveIntoTwentyProtectedBagSlotsLosslessly(t *testing.T) {
	p := Profile{ID: "one", InventoryVersion: 2, SafeSlots: []Stack{{"wood", 40}, {"cake", 8}, {"pearl", 12}}}
	for i := 0; i < 15; i++ {
		p.BagSlots[i] = Stack{"wood", inventoryLimit}
	}
	p.Hotbar[0] = Stack{"sugar", 10}
	w := New(map[string]Profile{"one": p})
	w.Join("one", "Baker")
	got := w.Profiles()["one"]
	if got.InventoryVersion != 3 || len(got.BagSlots) != 20 || len(got.SafeSlots) != 0 || len(got.Protected) != 0 {
		t.Fatal("legacy safe storage was not retired")
	}
	if got.Inventory["wood"] != 15*inventoryLimit+40 || got.Inventory["cake"] != 8 || got.Inventory["pearl"] != 12 || got.Inventory["sugar"] != 10 {
		t.Fatal("legacy migration lost items")
	}
	again := New(w.Profiles()).Profiles()["one"]
	if got.BagSlots != again.BagSlots || got.Hotbar != again.Hotbar || !reflect.DeepEqual(got.Inventory, again.Inventory) {
		t.Fatal("repeat migration changed inventory")
	}
	data, err := json.Marshal(w.Snapshot("one"))
	if err != nil || strings.Contains(string(data), `"safeSlots"`) {
		t.Fatal("runtime snapshot still exposes retired safe slots")
	}
	if err := w.Act("one", Action{Action: "move_item", From: "bag", FromSlot: 0, To: "safe", ToSlot: 0}); err == nil {
		t.Fatal("retired safe slots remain usable")
	}
}

func TestLegacySafeOverflowIsRecoverableAndMigrationIsDetached(t *testing.T) {
	p := Profile{ID: "one", InventoryVersion: 2, SafeSlots: []Stack{{"cake", inventoryLimit}, {"cake", inventoryLimit}}}
	for i := range p.BagSlots {
		p.BagSlots[i] = Stack{"wood", inventoryLimit}
	}
	w := New(map[string]Profile{"one": p})
	w.Join("one", "Baker")
	player := w.players["one"]
	if player.profile.Recovery["cake"] != 2*inventoryLimit {
		t.Fatal("overflow safe stacks were truncated")
	}
	player.profile.BagSlots[0] = Stack{}
	mustAct(t, w, Action{Action: "claim_recovery", Item: "cake"})
	if player.profile.BagSlots[0].Count != inventoryLimit || player.profile.Recovery["cake"] != inventoryLimit {
		t.Fatal("overflow could not be recovered one stack at a time")
	}
}
