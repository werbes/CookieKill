package game

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestDropGroupingAndActualPickupPreserveItems(t *testing.T) {
	expected := map[int][]int{1: {1}, 2: {1, 1}, 3: {1, 1, 1}, 4: {1, 1, 1, 1}, 5: {5}, 9: {5, 1, 1, 1, 1}, 10: {10}, 17: {10, 5, 1, 1}, 24: {10, 10, 1, 1, 1, 1}, 35: {10, 10, 10, 5}}
	for _, count := range []int{1, 2, 3, 4, 5, 9, 10, 17, 24, 35} {
		w, p := movementWorld()
		if !gain(p, "wood", count) {
			t.Fatal("setup gain")
		}
		mustAct(t, w, Action{Action: "drop_item", From: "bag", FromSlot: 0})
		amounts, total := []int{}, 0
		for _, d := range w.drops {
			amounts = append(amounts, d.node.Amount)
			total += d.node.Amount
			if d.node.Item != "wood" || d.node.Kind != "item_drop" {
				t.Fatal("drop lost real item identity")
			}
		}
		want := expected[count]
		sort.Ints(amounts)
		sort.Ints(want)
		if total != count || !reflect.DeepEqual(amounts, want) || p.profile.Inventory["wood"] != 0 {
			t.Fatalf("drop %d lost items or grouped incorrectly: %v", count, amounts)
		}
		for id := range w.drops {
			mustAct(t, w, Action{Action: "pickup_item", Target: id})
		}
		if len(w.drops) != 0 || p.profile.Inventory["wood"] != count {
			t.Fatal("pickup did not restore every dropped item")
		}
	}
}

func TestDroppedItemsStayInTheirIslandOrCaveInstance(t *testing.T) {
	w, p := movementWorld()
	p.profile.HomeIsland = &Island{Owner: "one"}
	w.setIsland(p, "one")
	mustAct(t, w, Action{Action: "drop_item", From: "hotbar", FromSlot: 0, Amount: 5})
	homeDrops := w.dropNodes(p)
	if len(homeDrops) != 1 || homeDrops[0].Island != "one" || homeDrops[0].InCave || homeDrops[0].Y != groundY(p) {
		t.Fatal("home drop lost its island or height")
	}
	w.setIsland(p, "")
	if len(w.dropNodes(p)) != 0 {
		t.Fatal("dropped items followed their owner to the mainland")
	}
	p.x, p.z, p.inCave = 120, -117, true
	mustAct(t, w, Action{Action: "drop_item", From: "hotbar", FromSlot: 0, Amount: 5})
	caveDrops := w.dropNodes(p)
	if len(caveDrops) != 1 || !caveDrops[0].InCave || caveDrops[0].Y >= 0 {
		t.Fatal("cave drop appeared on the desert surface")
	}
	p.inCave = false
	if len(w.dropNodes(p)) != 0 {
		t.Fatal("surface snapshot revealed a drop through the cave roof")
	}
}

func TestDropPartialStackAndFailedPickupAreAtomic(t *testing.T) {
	w, p := movementWorld()
	mustAct(t, w, Action{Action: "drop_item", From: "hotbar", FromSlot: 0, Amount: 5, Item: "sugar"})
	if p.profile.Hotbar[0].Count != 5 || len(w.drops) != 1 {
		t.Fatal("Drop 5 did not use the selected stack")
	}
	for i := range p.profile.BagSlots {
		p.profile.BagSlots[i] = Stack{"wood", inventoryLimit}
	}
	for i := range p.profile.Hotbar {
		p.profile.Hotbar[i] = Stack{"sugar", inventoryLimit}
	}
	p.profile.Hotbar[0].Count -= 2 // Partial merge must roll back when no slot fits the rest.
	syncInventory(&p.profile)
	before := cloneProfile(p.profile)
	var id string
	for key := range w.drops {
		id = key
	}
	if err := w.Act("one", Action{Action: "pickup_item", Target: id}); err == nil {
		t.Fatal("pickup bypassed full inventory")
	}
	if p.profile.BagSlots != before.BagSlots || p.profile.Hotbar != before.Hotbar || w.drops[id].node.Amount != 5 {
		t.Fatal("failed pickup lost or duplicated items")
	}
	for _, a := range []Action{
		{Action: "drop_item", From: "bag", FromSlot: 20},
		{Action: "drop_item", From: "safe", FromSlot: 0},
		{Action: "drop_item", From: "bag", FromSlot: 0, Amount: -1},
		{Action: "drop_item", From: "bag", FromSlot: 0, Amount: 4},
		{Action: "drop_item", From: "bag", FromSlot: 0, Item: "cake"},
	} {
		if err := w.Act("one", a); err == nil || p.profile.BagSlots != before.BagSlots || len(w.drops) != 1 {
			t.Fatal("invalid drop mutated items")
		}
	}
}

func TestDroppedItemsExpireAfterThirtyMinutesAndRedropRestartsTimer(t *testing.T) {
	w, p := movementWorld()
	now := time.Unix(2000000000, 0)
	w.now = func() time.Time { return now }
	mustAct(t, w, Action{Action: "drop_item", From: "hotbar", FromSlot: 0, Amount: 1})
	var first string
	for id := range w.drops {
		first = id
	}
	now = now.Add(29 * time.Minute)
	mustAct(t, w, Action{Action: "pickup_item", Target: first})
	mustAct(t, w, Action{Action: "drop_item", From: "hotbar", FromSlot: 0, Amount: 1})
	var second string
	for id := range w.drops {
		second = id
	}
	if first == second || len(w.drops) != 1 {
		t.Fatal("redrop reused the consumed ground entity")
	}
	now = now.Add(29 * time.Minute)
	w.Tick(.05)
	if len(w.dropNodes(p)) != 1 {
		t.Fatal("redropped item kept its old expiry")
	}
	now = now.Add(time.Minute)
	if err := w.Act("one", Action{Action: "pickup_item", Target: second}); err == nil || len(w.drops) != 0 {
		t.Fatal("expired item could still be picked up")
	}
}

func TestGroundPickupChecksInstanceLayerDistanceAndWalls(t *testing.T) {
	w, p := movementWorld()
	const id = "test_drop"
	drop := groundDrop{node: Node{ID: id, Kind: "item_drop", Item: "wood", Amount: 5, X: p.x + 2, Z: p.z, Available: true}, expires: w.now().Add(groundItemTTL)}
	w.drops = map[string]groundDrop{id: drop}
	for _, change := range []func(*Node){
		func(n *Node) { n.Island = "other" },
		func(n *Node) { n.InCave = true },
		func(n *Node) { n.X += 10 },
		func(n *Node) { n.Y = 5 },
	} {
		d := drop
		change(&d.node)
		w.drops[id] = d
		if err := w.Act("one", Action{Action: "pickup_item", Target: id}); err == nil || p.profile.Inventory["wood"] != 0 || len(w.drops) != 1 {
			t.Fatal("pickup bypassed location validation")
		}
	}
	w.drops[id] = drop
	w.colliders = []Collider{{X: p.x + 1, Z: p.z, Width: .2, Depth: 5, Height: 3}}
	if err := w.Act("one", Action{Action: "pickup_item", Target: id}); err == nil {
		t.Fatal("pickup moved an item through a wall")
	}
	w.colliders = nil
	mustAct(t, w, Action{Action: "interact", Target: id})
	if p.profile.Inventory["wood"] != 5 || len(w.drops) != 0 {
		t.Fatal("ordinary E interaction failed to collect the bundle")
	}
	if err := w.Act("one", Action{Action: "pickup_item", Target: id}); err == nil || p.profile.Inventory["wood"] != 5 {
		t.Fatal("second pickup duplicated a consumed bundle")
	}
}

func TestPackedFurnishingsCanBeDroppedAndPickedUp(t *testing.T) {
	w, p := movementWorld()
	item := "build_chest:teal"
	if !gain(p, item, 2) {
		t.Fatal("packed furniture was rejected as an inventory item")
	}
	mustAct(t, w, Action{Action: "drop_item", From: "bag", FromSlot: 0})
	for id, d := range w.drops {
		if d.node.Item != item {
			t.Fatal("drop lost furnishing paint")
		}
		mustAct(t, w, Action{Action: "pickup_item", Target: id})
	}
	if p.profile.Inventory[item] != 2 {
		t.Fatal("pickup lost packed furnishings")
	}
}
