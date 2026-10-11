package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

func clubCakeWorld() (*World, *player) {
	w, p := clubWorld()
	w.time = DayDuration
	p.x, p.z = -191, -245
	return w, p
}

func cakeSeller(t *testing.T, nodes []Node) Node {
	t.Helper()
	for _, n := range nodes {
		if n.ID == "club_cake_seller" {
			return n
		}
	}
	t.Fatal("missing cake seller")
	return Node{}
}

func TestClubCakeSellerNightBoundariesAndStaleAvailability(t *testing.T) {
	w, p := clubCakeWorld()
	for _, tc := range []struct {
		time  float64
		night bool
	}{{0, false}, {DayDuration - .001, false}, {DayDuration, true}, {DayDuration + NightDuration - .001, true}, {DayDuration + NightDuration, false}, {2*DayDuration + NightDuration, true}} {
		w.time = tc.time
		// Deliberately invert cached availability. Authorization and both snapshot
		// modes must still agree with the server clock at nightfall and dawn.
		for _, n := range w.nodes {
			if n.Kind == "club_cake_seller" {
				n.Available = !tc.night
			}
		}
		for _, snap := range []Snapshot{w.Snapshot("one"), w.SnapshotUpdate("one")} {
			n := cakeSeller(t, snap.Nodes)
			if n.Available != tc.night || n.Kind != "club_cake_seller" || n.Label != "Disco Duke · funny cakes" || n.X != -191 || n.Z != -245 || n.Y != ClubFloorY || !n.InCave {
				t.Fatalf("time %v: incorrect seller %+v", tc.time, n)
			}
		}
		before := cloneProfile(p.profile)
		err := w.Act("one", Action{Action: "buy_club_cake", Target: "club_cake_seller", Item: "funny_cake"})
		if (err == nil) != tc.night {
			t.Fatalf("time %v: night=%t purchase error=%v", tc.time, tc.night, err)
		}
		if !tc.night && !reflect.DeepEqual(p.profile, before) {
			t.Fatal("daytime purchase changed profile")
		}
		if err := w.Act("one", Action{Action: "interact", Target: "club_cake_seller"}); (err == nil) != tc.night {
			t.Fatalf("time %v: seller interaction disagrees with phase: %v", tc.time, err)
		}
		w.Tick(.0001)
		for _, n := range w.nodes {
			if n.Kind == "club_cake_seller" && n.Available != tc.night {
				t.Fatal("simulation did not synchronize seller availability")
			}
		}
	}
}

func TestClubCakePurchaseValidationIsAtomic(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*World, *player, *Action)
	}{
		{"day", func(w *World, p *player, a *Action) { w.time = 0 }},
		{"dawn", func(w *World, p *player, a *Action) { w.time = DayDuration + NightDuration }},
		{"surface", func(w *World, p *player, a *Action) { p.inCave = false }},
		{"desert cave", func(w *World, p *player, a *Action) { p.x, p.z = 120, -117 }},
		{"island", func(w *World, p *player, a *Action) { p.island = "one" }},
		{"distant", func(w *World, p *player, a *Action) { p.x = -210 }},
		{"wrong height", func(w *World, p *player, a *Action) { p.jumpOffset = 5 }},
		{"wrong target", func(w *World, p *player, a *Action) { a.Target = "club_bar" }},
		{"missing target", func(w *World, p *player, a *Action) { a.Target = "" }},
		{"wrong item", func(w *World, p *player, a *Action) { a.Item = "beer" }},
		{"missing item", func(w *World, p *player, a *Action) { a.Item = "" }},
		{"negative amount", func(w *World, p *player, a *Action) { a.Amount = -1 }},
		{"oversized amount", func(w *World, p *player, a *Action) { a.Amount = 21 }},
		{"insufficient coins", func(w *World, p *player, a *Action) { p.profile.Coins = 29 }},
		{"partial stack capacity", func(w *World, p *player, a *Action) {
			for i := range p.profile.BagSlots {
				p.profile.BagSlots[i] = Stack{"wood", inventoryLimit}
			}
			p.profile.BagSlots[0] = Stack{"funny_cake", inventoryLimit - 1}
			syncInventory(&p.profile)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, p := clubCakeWorld()
			a := Action{Action: "buy_club_cake", Target: "club_cake_seller", Item: "funny_cake", Amount: 2}
			tc.setup(w, p, &a)
			before := cloneProfile(p.profile)
			if err := w.Act("one", a); err == nil {
				t.Fatal("invalid purchase succeeded")
			}
			if !reflect.DeepEqual(p.profile, before) || w.active(p, "funny_cake") || p.cooldowns["eat"] != 0 {
				t.Fatal("failed purchase altered inventory, wallet or consumption state")
			}
		})
	}
}

func TestClubCakesPurchaseStorageDropAndPersistence(t *testing.T) {
	w, p := clubCakeWorld()
	for _, cake := range clubCakes {
		before := p.profile.Coins
		mustAct(t, w, Action{Action: "buy_club_cake", Target: "club_cake_seller", Item: cake.ID, Amount: 20})
		if p.profile.Coins != before-20*cake.Price || p.profile.Inventory[cake.ID] != 20 || w.active(p, "funny_cake") {
			t.Fatal("purchase has incorrect price, quantity or automatic consumption")
		}
		for i, stack := range p.profile.BagSlots {
			if stack.Item == cake.ID {
				mustAct(t, w, Action{Action: "drop_item", From: "bag", FromSlot: i, Amount: 5})
				break
			}
		}
		if p.profile.Inventory[cake.ID] != 15 || len(w.drops) != 1 {
			t.Fatal("cake drop failed")
		}
		for id, drop := range w.drops {
			if drop.node.Y != ClubFloorY || !drop.node.InCave {
				t.Fatal("cake dropped onto wrong layer")
			}
			mustAct(t, w, Action{Action: "pickup_item", Target: id})
		}
	}
	// Exercise persistent hotbar normalization as well as bag storage.
	for i, stack := range p.profile.BagSlots {
		if stack.Item == "cosmic_cake" {
			mustAct(t, w, Action{Action: "move_item", From: "bag", FromSlot: i, To: "hotbar", ToSlot: 2})
			break
		}
	}
	data, err := json.Marshal(w.Profiles())
	if err != nil {
		t.Fatal(err)
	}
	var profiles map[string]Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		t.Fatal(err)
	}
	restored := New(profiles).Profiles()["one"]
	if restored.Inventory["funny_cake"] != 20 || restored.Inventory["cosmic_cake"] != 20 || restored.Hotbar[2] != (Stack{"cosmic_cake", 20}) || restored.Coins != 200 {
		t.Fatalf("cakes or wallet changed on restart: %+v", restored)
	}
}

func TestClubCakeConsumptionRefreshCooldownAndPresentationOnly(t *testing.T) {
	w, p := clubCakeWorld()
	setItem(p, "funny_cake", 3)
	setItem(p, "cosmic_cake", 2)
	p.health, p.stamina, p.yaw, p.pitch = 63, 47, .8, .2
	beforeSpeed := w.playerMovementSpeed(p)
	// Cakes remain edible during the day and away from the seller.
	w.time = 10
	p.x, p.z, p.inCave = -35, -35, false
	mustAct(t, w, Action{Action: "eat", Item: "funny_cake", Amount: 20})
	if p.profile.Inventory["funny_cake"] != 2 || p.buffs["funny_cake"] != 55 || p.cooldowns["eat"] != 11 {
		t.Fatal("cake did not consume exactly one or set bounded effect and cooldown")
	}
	if p.health != 63 || p.stamina != 47 || p.x != -35 || p.z != -35 || p.yaw != .8 || p.pitch != .2 || w.playerMovementSpeed(p) != beforeSpeed || p.alcoholDrinks != 0 {
		t.Fatal("presentation effect changed player physiology, movement or aim")
	}
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "eat", Item: "cosmic_cake"}); err == nil || !reflect.DeepEqual(before, p.profile) || p.buffs["funny_cake"] != 55 {
		t.Fatal("cooldown did not protect cake and active effect")
	}
	w.time++
	mustAct(t, w, Action{Action: "eat", Item: "cosmic_cake"})
	if p.buffs["funny_cake"] != 86 {
		t.Fatal("cosmic cake stacked duration instead of refreshing")
	}
	for i, stack := range p.profile.BagSlots {
		if stack.Item == "funny_cake" {
			mustAct(t, w, Action{Action: "move_item", From: "bag", FromSlot: i, To: "hotbar", ToSlot: 1})
			break
		}
	}
	w.time++
	mustAct(t, w, Action{Action: "select", Slot: 1})
	mustAct(t, w, Action{Action: "eat"})
	if p.profile.Hotbar[1] != (Stack{"funny_cake", 1}) || p.buffs["funny_cake"] != 57 {
		t.Fatal("selected hotbar cake did not consume one and refresh duration")
	}
	setItem(p, "water", 1)
	if err := w.Act("one", Action{Action: "drink", Item: "water"}); err == nil || p.profile.Inventory["water"] != 1 {
		t.Fatal("cake did not share refreshment cooldown")
	}
	w.time++
	mustAct(t, w, Action{Action: "drink", Item: "water"})
	if err := w.Act("one", Action{Action: "eat", Item: "funny_cake"}); err == nil || p.profile.Inventory["funny_cake"] != 1 {
		t.Fatal("drink did not share cake cooldown")
	}
	w.time++
	take(p, "cosmic_cake", 1)
	if err := w.Act("one", Action{Action: "eat", Item: "cosmic_cake"}); err == nil || p.buffs["funny_cake"] != 57 || p.cooldowns["eat"] != 14 {
		t.Fatal("missing cake changed consumption state")
	}
}

func TestClubCakeEffectSnapshotsExpiryReconnectAndDeath(t *testing.T) {
	w, p := clubCakeWorld()
	setItem(p, "funny_cake", 4)
	mustAct(t, w, Action{Action: "eat", Item: "funny_cake"})
	w.Join("two", "Other dancer")
	before, changed := w.Snapshot("one"), w.SnapshotUpdate("one")
	if before.Me.Buffs["funny_cake"] != 45 || w.Snapshot("two").Me.Buffs["funny_cake"] != 0 {
		t.Fatal("effect missing from eater or applied to another viewer")
	}
	changed.Me.Buffs["funny_cake"] = 999
	changed.ClubCakes[0].Price = 0
	changed.ClubCakes[1].Duration = 999
	if !reflect.DeepEqual(before, w.Snapshot("one")) {
		t.Fatal("snapshot mutation changed world or shared catalog")
	}
	w.Leave("one")
	w.time += 10
	w.Join("one", "Returning dancer")
	if w.Snapshot("one").Me.Buffs["funny_cake"] != 0 {
		t.Fatal("reconnect restored a discarded runtime effect")
	}
	w.Leave("one")
	w.time += 35
	w.Join("one", "Returning dancer")
	p = w.players["one"]
	if w.Snapshot("one").Me.Buffs["funny_cake"] != 0 {
		t.Fatal("offline time failed to expire effect")
	}
	w.Tick(.05)
	if _, exists := p.buffs["funny_cake"]; exists {
		t.Fatal("simulation retained expired cake buff")
	}
	mustAct(t, w, Action{Action: "eat", Item: "funny_cake"})
	delete(p.buffs, "spawn_shield")
	if !w.damagePlayer(nil, p, 1000, "test") || w.active(p, "funny_cake") || p.profile.Inventory["funny_cake"] != 2 {
		t.Fatal("death did not clear effect and retain unconsumed cakes")
	}
	w.time++
	mustAct(t, w, Action{Action: "eat", Item: "funny_cake"})
	restarted := New(w.Profiles())
	restarted.Join("one", "After restart")
	if restarted.Snapshot("one").Me.Buffs["funny_cake"] != 0 || restarted.Snapshot("one").Me.Inventory["funny_cake"] != 1 {
		t.Fatal("restart persisted transient effect or lost unconsumed cake")
	}
}
