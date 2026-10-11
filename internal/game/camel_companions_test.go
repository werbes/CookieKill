package game

import (
	"fmt"
	"math"
	"reflect"
	"testing"
)

func companionWorld() (*World, *player) {
	w, p := testWorld()
	w.colliders, w.propertyColliders = nil, nil
	p.x, p.z = 14, -20
	p.profile.Camels = []Camel{{ID: "one_camel_1", X: 0, Z: -20, Speed: 10, Jump: 6, Stamina: 120, Happiness: 100, TraitsVersion: 1}}
	camelCompatibility(&p.profile)
	return w, p
}

func TestHeldTreatInteractionFeedsSelectedStackAndNeverMounts(t *testing.T) {
	w, p := companionWorld()
	p.profile.Camels[0].X = 12
	p.profile.BagSlots[0] = Stack{"swift_dates", 5}
	p.profile.Hotbar[1] = Stack{"swift_dates", 2}
	p.profile.SelectedSlot = 1
	syncInventory(&p.profile)
	mustAct(t, w, Action{Action: "interact", Target: "one_camel_1"})
	if p.ridingCamel || p.profile.Camels[0].Speed != 10 || p.profile.Camels[0].TrainingBoost.Speed != 120 || p.profile.Hotbar[1].Count != 1 || p.profile.BagSlots[0].Count != 5 {
		t.Fatal("E did not feed exactly one held treat")
	}
	snapshot := w.Snapshot("one")
	if snapshot.Me.Camels[0].EatingUntil != 2 {
		t.Fatal("feeding animation missing")
	}
	found := false
	for _, node := range snapshot.Nodes {
		if node.ID == "one_camel_1" {
			found = true
			if node.Phase != "eating" {
				t.Fatal("camel node has no chewing phase")
			}
		}
	}
	if !found {
		t.Fatal("feeding hid the parked camel")
	}
	snapshot.Me.Camels[0].EatingUntil = 999
	if p.profile.Camels[0].EatingUntil != 0 || w.Profiles()["one"].Camels[0].EatingUntil != 0 {
		t.Fatal("transient chewing state leaked into persistence")
	}
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "interact", Target: "one_camel_1"}); err == nil || !reflect.DeepEqual(before, p.profile) || p.ridingCamel {
		t.Fatal("cooldown fed or mounted camel")
	}
	w.time++
	mustAct(t, w, Action{Action: "interact", Target: "one_camel_1"})
	if p.profile.Hotbar[1].Count != 0 || p.profile.Camels[0].Speed != 10 || p.profile.Camels[0].TrainingBoost.Speed != 240 || p.ridingCamel {
		t.Fatal("last held treat did not feed without mounting")
	}
	w.time += 2
	if w.Snapshot("one").Me.Camels[0].EatingUntil != 0 {
		t.Fatal("chewing did not expire")
	}
	mustAct(t, w, Action{Action: "interact", Target: "one_camel_1"})
	if !p.ridingCamel {
		t.Fatal("ordinary E interaction no longer mounts")
	}
}

func TestCamelCallRunsAroundWallsAndStopsNearOwner(t *testing.T) {
	w, p := companionWorld()
	w.colliders = []Collider{{ID: "test_wall", X: 7, Z: -20, Width: .4, Depth: 8, Height: 3}}
	mustAct(t, w, Action{Action: "camel_call", Target: "one_camel_1"})
	if !w.Snapshot("one").Me.Camels[0].Following {
		t.Fatal("call status absent")
	}
	if w.Profiles()["one"].Camels[0].Following {
		t.Fatal("call persisted as live state")
	}
	if err := w.Act("one", Action{Action: "camel_call", Target: "one_camel_1"}); err == nil {
		t.Fatal("duplicate call accepted")
	}
	detoured := false
	for i := 0; i < 180 && p.camelCalls["one_camel_1"] != nil; i++ {
		before := p.profile.Camels[0]
		w.Tick(.05)
		c := p.profile.Camels[0]
		if distance(before.X, before.Z, c.X, c.Z) > .60001 {
			t.Fatal("called camel teleported instead of running")
		}
		if w.solidBlockedAt(c.X, c.Y, c.Z, .65) {
			t.Fatal("camel crossed a solid wall")
		}
		if math.Abs(c.Z+20) > 4.6 {
			detoured = true
		}
	}
	c := p.profile.Camels[0]
	if !detoured || p.camelCalls[c.ID] != nil || distance(c.X, c.Z, p.x, p.z) > 2.21 || distance(c.X, c.Z, p.x, p.z) < 1.8 {
		t.Fatalf("camel failed to route and stop nearby: %+v", c)
	}
	w.Tick(1)
	if p.profile.Camels[0] != c {
		t.Fatal("called camel drifted after arrival")
	}
}

func TestCamelCallRejectsOtherWorldsAndCancelsOnTravelDisconnectOrMount(t *testing.T) {
	for _, change := range []func(*player){func(p *player) { p.profile.Camels[0].Island = "other" }, func(p *player) { p.profile.Camels[0].InCave = true }, func(p *player) { p.x, p.z = -100, 100 }} {
		w, p := companionWorld()
		change(p)
		before := cloneProfile(p.profile)
		if err := w.Act("one", Action{Action: "camel_call", Target: "one_camel_1"}); err == nil || !reflect.DeepEqual(before, p.profile) {
			t.Fatal("cross-world or water call accepted")
		}
	}
	for _, mode := range []string{"travel", "disconnect", "mount"} {
		t.Run(mode, func(t *testing.T) {
			w, p := companionWorld()
			mustAct(t, w, Action{Action: "camel_call", Target: "one_camel_1"})
			w.Tick(.1)
			before := p.profile.Camels[0]
			switch mode {
			case "travel":
				p.island = "other"
			case "disconnect":
				w.SetConnected("one", false)
			case "mount":
				p.x, p.z = before.X, before.Z
				mustAct(t, w, Action{Action: "camel_mount", Target: before.ID})
			}
			w.Tick(.05)
			if p.camelCalls[before.ID] != nil {
				t.Fatal("call survived travel, disconnect or mounting")
			}
			if mode != "mount" && (p.profile.Camels[0].X != before.X || p.profile.Camels[0].Z != before.Z) {
				t.Fatal("cancelled call moved or teleported camel")
			}
		})
	}
	// The club is a valid same-layer route even though John Pork cannot enter.
	w, p := companionWorld()
	p.x, p.z, p.inCave = -202, -238, true
	p.profile.Camels[0].X, p.profile.Camels[0].Z, p.profile.Camels[0].Y, p.profile.Camels[0].InCave = -211, -238, ClubFloorY, true
	mustAct(t, w, Action{Action: "camel_call", Target: "one_camel_1"})
	for i := 0; i < 30; i++ {
		w.Tick(.05)
	}
	if p.camelCalls["one_camel_1"] != nil || p.profile.Camels[0].Y != ClubFloorY {
		t.Fatal("same-level underground call failed")
	}
}

func penWorld(count int) (*World, *player) {
	w, p := companionWorld()
	p.profile.HomeIsland = &Island{Owner: "one", Visitors: true, Builders: map[string]bool{}, Chest: map[string]int{}, Objects: []BuildObject{{ID: "home_pen", Kind: "camel_pen"}}}
	p.island, p.x, p.z = "one", 0, 6
	p.profile.Camels = nil
	for i := 0; i < count; i++ {
		c := Camel{ID: fmt.Sprintf("one_camel_%d", i+1), Island: "one", Z: 6}
		camelTraits(&c)
		p.profile.Camels = append(p.profile.Camels, c)
	}
	camelCompatibility(&p.profile)
	return w, p
}

func TestCamelPenCapacityAssignmentProtectionAndRelease(t *testing.T) {
	w, p := penWorld(5)
	used := map[int]bool{}
	for i := 0; i < 4; i++ {
		mustAct(t, w, Action{Action: "camel_house", Target: p.profile.Camels[i].ID, Item: "home_pen"})
		c := p.profile.Camels[i]
		if c.HomePenID != "home_pen" || used[c.HomePenSlot] {
			t.Fatal("pen did not reserve unique sleeping slots")
		}
		used[c.HomePenSlot] = true
	}
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "camel_house", Target: p.profile.Camels[4].ID, Item: "home_pen"}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("full pen accepted fifth camel or mutated assignments")
	}
	if !w.camelPenOccupied("one", "home_pen") {
		t.Fatal("occupied pen not protected")
	}
	p.buildMode = true
	for _, action := range []string{"edit_build", "pack_build", "destroy_build"} {
		if err := w.Act("one", Action{Action: action, Target: "home_pen"}); err == nil {
			t.Fatal("occupied pen could be removed or moved")
		}
	}
	p.x, p.z = 0, 0
	for i := 0; i < 4; i++ {
		mustAct(t, w, Action{Action: "camel_unhouse", Target: p.profile.Camels[i].ID})
	}
	if w.camelPenOccupied("one", "home_pen") {
		t.Fatal("released pen still occupied")
	}
}

func TestAssignedCamelReturnsHomeOnDisconnectLogoutAndRestart(t *testing.T) {
	w, p := penWorld(1)
	mustAct(t, w, Action{Action: "camel_house", Target: "one_camel_1", Item: "home_pen"})
	home := p.profile.Camels[0]
	// Call the camel out through the gate, then ride away to the mainland.
	mustAct(t, w, Action{Action: "camel_call", Target: "one_camel_1"})
	for i := 0; i < 100 && p.camelCalls["one_camel_1"] != nil; i++ {
		w.Tick(.05)
	}
	if distance(p.profile.Camels[0].X, p.profile.Camels[0].Z, p.x, p.z) > 2.3 {
		t.Fatal("housed camel could not leave through its gate")
	}
	mustAct(t, w, Action{Action: "camel_mount", Target: "one_camel_1"})
	mustAct(t, w, Action{Action: "teleport_main"})
	if p.profile.Camels[0].Island != "" || p.profile.Camels[0].HomePenID != "home_pen" {
		t.Fatal("travel lost home assignment")
	}
	w.SetConnected("one", false)
	w.Tick(.05)
	if p.ridingCamel || p.profile.Camels[0] != home {
		t.Fatal("disconnected rider's assigned camel did not return to its pen")
	}
	w.Leave("one")
	w.Join("one", "Returning owner")
	p = w.players["one"]
	if p.ridingCamel || p.profile.Camels[0] != home || p.island != "" {
		t.Fatal("reconnect stole housed camel back from the island")
	}
	w.Join("visitor", "Visitor")
	visitor := w.players["visitor"]
	visitor.island = "one"
	visible := false
	for _, n := range w.Snapshot("visitor").Nodes {
		if n.ID == home.ID && n.Phase == "housed" && n.Island == "one" {
			visible = true
		}
	}
	if !visible {
		t.Fatal("visitor cannot see offline-housed camel")
	}
	// A clean restart also houses assigned camels saved while their rider was away.
	p.profile.Camels[0].Island, p.profile.Camels[0].X, p.profile.Camels[0].Z = "", 120, -70
	restarted := New(w.Profiles())
	if got := restarted.Profiles()["one"].Camels[0]; got != home {
		t.Fatalf("restart misplaced camel or traits: %+v", got)
	}
}

func TestCamelHomesMigrationInvalidSlotsAndExpandedIslandPersistence(t *testing.T) {
	w, p := penWorld(5)
	p.profile.HomeIsland.UpgradeLevel = 5
	for i := range p.profile.Camels {
		p.profile.Camels[i].HomePenID = "home_pen"
		p.profile.Camels[i].HomePenSlot = 0
	}
	p.profile.Camels[4].HomePenID = "removed_pen"
	normalizeCamels(&p.profile)
	for i := 0; i < 4; i++ {
		if p.profile.Camels[i].HomePenSlot != i {
			t.Fatal("duplicate sleeping slots not repaired")
		}
	}
	if p.profile.Camels[4].HomePenID != "" {
		t.Fatal("missing pen assignment survived")
	}
	p.profile.Camels[4].X, p.profile.Camels[4].Z = 61, 0
	restarted := New(w.Profiles())
	restarted.Join("one", "Returning owner")
	saved := restarted.Profiles()["one"].Camels[4]
	if saved.X != 61 || saved.Island != "one" {
		t.Fatal("expanded-island camel reset to Abu Fanous")
	}
	restarted.Leave("one")
	restarted.Join("one", "Returning owner")
	if got := restarted.players["one"].profile.Camels[4]; got != saved {
		t.Fatal("expanded-island camel changed on reconnect")
	}
}
