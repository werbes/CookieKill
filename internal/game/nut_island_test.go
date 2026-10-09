package game

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func nutTestWorld(t *testing.T) (*World, *player) {
	t.Helper()
	w, p := testWorld()
	p.profile.Coins = 5000
	p.x, p.y, p.z, p.inCave = 120, CaveFloorY, -126, true
	mustAct(t, w, Action{Action: "nut_island_unlock", Target: "green_lollipop"})
	mustAct(t, w, Action{Action: "teleport_nut_island"})
	return w, p
}

func TestNutIslandUnlockIsAtomicOwnedAndProximityChecked(t *testing.T) {
	for _, setup := range []func(*player){
		func(p *player) { p.profile.Coins = 999 },
		func(p *player) { p.x += 5 },
		func(p *player) { p.inCave = false },
		func(p *player) { p.island = "other" },
		func(p *player) { p.jumpOffset = 3 },
	} {
		w, p := testWorld()
		p.profile.Coins = 1000
		p.x, p.y, p.z, p.inCave = 120, CaveFloorY, -126, true
		setup(p)
		before := p.profile.Coins
		if err := w.Act("one", Action{Action: "nut_island_unlock", Target: "green_lollipop"}); err == nil || p.profile.Coins != before || p.profile.NutIsland != nil {
			t.Fatal("invalid purchase changed the wallet or granted an island")
		}
	}
	w, p := nutTestWorld(t)
	if p.profile.Coins != 4000 || p.profile.NutIsland.Owner != "one" || p.island != "nut:one" || p.inCave || p.x != 0 || p.z != 20 {
		t.Fatal("unlock/teleport did not charge exactly1000 and enter the private island")
	}
	if err := w.Act("one", Action{Action: "nut_island_unlock", Target: "green_lollipop"}); err == nil || p.profile.Coins != 4000 {
		t.Fatal("repeat purchase charged twice")
	}
	if err := w.Act("one", Action{Action: "build_mode", Enabled: true}); err == nil || p.buildMode {
		t.Fatal("home building became available on the animal island")
	}
	w.Join("other", "Other owner")
	other := w.players["other"]
	if err := w.Act("other", Action{Action: "teleport_nut_island", Target: "one"}); err == nil || other.island != "" {
		t.Fatal("a non-owner could enter someone else's nut island")
	}
	mustAct(t, w, Action{Action: "nut_island_leave"})
	if p.island != "" || !p.inCave || playerY(p) != CaveFloorY || p.x != 120 || p.z != -117 {
		t.Fatal("return did not reach safe cave floor")
	}
}

func TestNutIslandReleaseAllSpeciesConsumesRealInventoryOnce(t *testing.T) {
	w, p := nutTestWorld(t)
	for _, species := range rodentCatalog {
		item := "pet_" + species.ID
		if !gain(p, item, 2) {
			t.Fatalf("captured %s did not fit valid inventory", species.ID)
		}
		before := len(p.profile.NutIsland.Pets)
		mustAct(t, w, Action{Action: "rodent_release", Item: item})
		if p.profile.Inventory[item] != 1 || len(p.profile.NutIsland.Pets) != before+1 || p.profile.NutIsland.Pets[before].Species != species.ID {
			t.Fatal("release failed to consume exactly one physical captured animal")
		}
	}
	seen := map[string]bool{}
	for _, pet := range p.profile.NutIsland.Pets {
		if pet.ID == "" || seen[pet.ID] || pet.CageID != "" || nutPointBlocked(p.profile.NutIsland, pet.X, pet.Z, .3) {
			t.Fatal("released animal had duplicate identity or unsafe free position")
		}
		seen[pet.ID] = true
	}
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "rodent_release", Item: "sugar"}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("non-animal release mutated inventory or pets")
	}
	p.island = "nut:other"
	if err := w.Act("one", Action{Action: "rodent_release", Item: "pet_wood_mouse"}); err == nil || p.profile.Inventory["pet_wood_mouse"] != 1 {
		t.Fatal("release consumed inventory on another owner's island")
	}
}

func TestNutCageCapacityAssignmentFreeAndAtomicBagPickup(t *testing.T) {
	w, p := nutTestWorld(t)
	p.x, p.z = 4, 22
	mustAct(t, w, Action{Action: "nut_cage_buy"})
	if p.profile.Coins != 3850 || len(p.profile.NutIsland.Cages) != 1 {
		t.Fatal("cage did not cost150 coins")
	}
	cage := p.profile.NutIsland.Cages[0]
	p.x, p.z = cage.X-4.3, cage.Z
	if !gain(p, "pet_wood_mouse", 6) {
		t.Fatal("fixture inventory rejected pets")
	}
	for i := 0; i < 4; i++ {
		mustAct(t, w, Action{Action: "rodent_release", Item: "pet_wood_mouse", Target: cage.ID})
	}
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "rodent_release", Item: "pet_wood_mouse", Target: cage.ID}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("fifth resident consumed inventory or overfilled cage")
	}
	first := p.profile.NutIsland.Pets[0].ID
	mustAct(t, w, Action{Action: "nut_pet_free", Target: first})
	if p.profile.NutIsland.Pets[0].CageID != "" || nutCageCount(p.profile.NutIsland, cage.ID) != 3 || nutPointBlocked(p.profile.NutIsland, p.profile.NutIsland.Pets[0].X, p.profile.NutIsland.Pets[0].Z, .3) {
		t.Fatal("freeing a pet did not leave the cage at a safe point")
	}
	mustAct(t, w, Action{Action: "nut_pet_cage", Target: first, Item: cage.ID})
	if nutCageCount(p.profile.NutIsland, cage.ID) != 4 {
		t.Fatal("assignment did not occupy the freed slot")
	}
	// Fill every real slot with another item. Picking up must not delete a pet.
	for i := range p.profile.BagSlots {
		p.profile.BagSlots[i] = Stack{Item: "sugar", Count: inventoryLimit}
	}
	for i := range p.profile.Hotbar {
		p.profile.Hotbar[i] = Stack{Item: "sugar", Count: inventoryLimit}
	}
	syncInventory(&p.profile)
	before = cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "nut_pet_pack", Target: first}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("full bag caused a lost or duplicated released pet")
	}
	p.profile.BagSlots[0] = Stack{}
	syncInventory(&p.profile)
	mustAct(t, w, Action{Action: "nut_pet_pack", Target: first})
	if p.profile.Inventory["pet_wood_mouse"] != 1 || len(p.profile.NutIsland.Pets) != 3 || nutCageCount(p.profile.NutIsland, cage.ID) != 3 {
		t.Fatal("pickup did not exchange one released pet for one inventory item")
	}
	if err := w.Act("one", Action{Action: "nut_pet_pack", Target: first}); err == nil || p.profile.Inventory["pet_wood_mouse"] != 1 {
		t.Fatal("repeat pickup duplicated a pet")
	}
}

func TestNutIslandCagesCannotTrapAnimalsAndRespectPlotLimit(t *testing.T) {
	w, p := nutTestWorld(t)
	p.x, p.z = 4, 22
	p.profile.NutIsland.Pets = []NutPet{{ID: "pet_fixture", Species: "rat", X: 16, Z: 18}}
	if err := w.Act("one", Action{Action: "nut_cage_buy"}); err == nil || p.profile.Coins != 4000 {
		t.Fatal("cage was built over a roaming pet")
	}
	p.profile.NutIsland.Pets[0].X = 0
	for i := 0; i < 8; i++ {
		mustAct(t, w, Action{Action: "nut_cage_buy"})
	}
	if p.profile.Coins != 2800 || len(p.profile.NutIsland.Cages) != 8 {
		t.Fatal("cage plot purchases charged an incorrect amount")
	}
	if err := w.Act("one", Action{Action: "nut_cage_buy"}); err == nil || p.profile.Coins != 2800 {
		t.Fatal("ninth cage overfilled the island or charged for failure")
	}
	if !w.islandBlocked(p, 16, 18) || !w.islandBlocked(p, 37, 0) || w.islandBlocked(p, 0, 20) || w.islandGround(p) != 0 {
		t.Fatal("island shore, cages, or central walkway collision incorrect")
	}
	for _, trunk := range nutGroveTrees {
		if !w.islandBlocked(p, trunk.X, trunk.Z) || !nutPointBlocked(p.profile.NutIsland, trunk.X, trunk.Z, .35) {
			t.Fatal("a visible grove trunk lacked player or pet collision")
		}
	}
}

func TestNutIslandPetsRoamWholeIslandAndStayInsideCages(t *testing.T) {
	w, p := nutTestWorld(t)
	p.x, p.z = 4, 22
	mustAct(t, w, Action{Action: "nut_cage_buy"})
	cage := p.profile.NutIsland.Cages[0]
	p.profile.NutIsland.Pets = []NutPet{{ID: "free", Species: "squirrel", X: 0, Z: 0}, {ID: "caged", Species: "hamster", CageID: cage.ID, X: cage.X, Z: cage.Z}}
	maxTravel := 0.
	for i := 0; i < 2400; i++ {
		w.time += .05
		w.tickNutIslandPets(.05)
		free, penned := p.profile.NutIsland.Pets[0], p.profile.NutIsland.Pets[1]
		maxTravel = max(maxTravel, math.Hypot(free.X, free.Z))
		if nutPointBlocked(p.profile.NutIsland, free.X, free.Z, .3) || math.Abs(penned.X-cage.X) > 2.4 || math.Abs(penned.Z-cage.Z) > 2.4 {
			t.Fatal("a pet crossed a cage fence or island shoreline")
		}
	}
	if maxTravel < 15 {
		t.Fatal("free pet only paced beside its release point")
	}
	before := cloneNutIsland(p.profile.NutIsland)
	w.SetConnected("one", false)
	w.time += 50
	w.tickNutIslandPets(.05)
	if !reflect.DeepEqual(before, p.profile.NutIsland) {
		t.Fatal("offline pets drifted while their owner was disconnected")
	}
}

func TestNutIslandJSONRestartClonePrivacyAndNormalization(t *testing.T) {
	w, p := nutTestWorld(t)
	p.x, p.z = 4, 22
	mustAct(t, w, Action{Action: "nut_cage_buy"})
	if !gain(p, "pet_otter", 2) {
		t.Fatal("otter inventory fixture failed")
	}
	mustAct(t, w, Action{Action: "rodent_release", Item: "pet_otter"})
	before := cloneNutIsland(p.profile.NutIsland)
	data, err := json.Marshal(w.Profiles())
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]Profile
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	restored := New(saved)
	restored.Join("one", "Returned")
	rider := restored.players["one"]
	if !reflect.DeepEqual(before, rider.profile.NutIsland) || rider.profile.Inventory["pet_otter"] != 1 {
		t.Fatal("JSON save/restart lost cages, pets, or captured inventory")
	}
	mustAct(t, restored, Action{Action: "teleport_nut_island"})
	view := restored.SnapshotUpdate("one")
	if view.NutIsland == nil || len(view.NutIsland.Pets) != 1 || view.Island != nil || view.Me.Area != "nut island" {
		t.Fatal("nut island snapshot was overwritten by the home-island decorator")
	}
	for _, node := range view.Nodes {
		if node.Kind == "island_portal" || node.Kind == "island_upgrade" {
			t.Fatal("home-island nodes leaked into Nut Island")
		}
	}
	view.NutIsland.Pets[0].Species = "changed"
	view.NutIsland.Cages[0].X = 999
	view.Me.NutIsland.Pets[0].X = 999
	if !reflect.DeepEqual(before, rider.profile.NutIsland) {
		t.Fatal("snapshot exposed mutable persistent pet/cage data")
	}
	restored.Join("other", "Other")
	if restored.Snapshot("other").NutIsland != nil || len(restored.Snapshot("other").Players) != 0 {
		t.Fatal("mainland viewer saw another player's private pet island")
	}
	profile := cloneProfile(rider.profile)
	profile.NutIsland.Owner = "forged"
	profile.NutIsland.Cages[0].Capacity = 99
	profile.NutIsland.Pets = []NutPet{}
	for i := 0; i < 5; i++ {
		profile.NutIsland.Pets = append(profile.NutIsland.Pets, NutPet{ID: "duplicate", Species: "rat", CageID: profile.NutIsland.Cages[0].ID, X: math.NaN(), Z: 999})
	}
	normalizeNutIsland(&profile)
	ids := map[string]bool{}
	for _, pet := range profile.NutIsland.Pets {
		if ids[pet.ID] || !finite(pet.X) || !finite(pet.Z) {
			t.Fatal("normalization retained duplicate identities or unsafe positions")
		}
		ids[pet.ID] = true
	}
	if profile.NutIsland.Owner != "one" || len(profile.NutIsland.Pets) != 5 || nutCageCount(profile.NutIsland, profile.NutIsland.Cages[0].ID) != 4 {
		t.Fatal("normalization dropped a valid pet instead of freeing cage overflow")
	}
}

func TestNutIslandMountedTravelAndRaceReturnsPreserveOrigin(t *testing.T) {
	for _, scheduled := range []bool{false, true} {
		w, p := nutTestWorld(t)
		p.profile.Camels = []Camel{{ID: "one_camel_1", Name: "Peanut", X: 0, Z: 20, Island: p.island, TraitsVersion: 1, Speed: 11, Jump: 7, Stamina: 150, Happiness: 80}}
		p.ridingCamel, p.mountedCamelID = true, "one_camel_1"
		before := p.profile.Camels[0]
		if scheduled {
			w.raceRegistrations = map[string]float64{"one": 120}
			w.time = 100
			w.tickCamelRace(0)
		} else {
			mustAct(t, w, Action{Action: "race_casual_host"})
		}
		mustAct(t, w, Action{Action: "race_leave"})
		if p.island != "nut:one" || p.x != 0 || p.z != 20 || !p.ridingCamel || p.profile.Camels[0] != before {
			t.Fatal("race return lost the nut-island origin or owned camel")
		}
		mustAct(t, w, Action{Action: "nut_island_leave"})
		if !p.ridingCamel || !p.inCave || p.profile.Camels[0].Island != "" || p.profile.Camels[0].Y != CaveFloorY {
			t.Fatal("return-to-cave portal lost the mounted companion")
		}
		mustAct(t, w, Action{Action: "teleport_nut_island"})
		if p.profile.Camels[0].Island != "nut:one" || p.profile.Camels[0].Y != 0 || !p.ridingCamel {
			t.Fatal("mounted teleport did not bring the same camel back")
		}
	}
}

func TestNutIslandCamelCallUsesTheAnimalIslandWalkway(t *testing.T) {
	w, p := nutTestWorld(t)
	p.x, p.z = 0, 1
	p.profile.Camels = []Camel{{ID: "one_camel_1", Name: "Peanut", X: 0, Z: 20, Island: p.island, TraitsVersion: 1, Speed: 11, Jump: 7, Stamina: 150, Happiness: 80}}
	mustAct(t, w, Action{Action: "camel_call", Target: "one_camel_1"})
	for i := 0; i < 160 && p.camelCalls["one_camel_1"] != nil; i++ {
		w.Tick(.05)
	}
	camel := p.profile.Camels[0]
	if distance(camel.X, camel.Z, p.x, p.z) > 3.5 || camel.Island != "nut:one" || camel.Y != 0 || p.camelCalls[camel.ID] != nil {
		t.Fatal("called camel could not follow the Nut Island walkway")
	}
	view := w.SnapshotUpdate("one")
	before := cloneNutIsland(view.Me.NutIsland)
	p.profile.NutIsland.Pets = append(p.profile.NutIsland.Pets, NutPet{ID: "added_later", Species: "chipmunk", X: 0, Z: 0})
	w.Tick(.05)
	if !reflect.DeepEqual(before, view.Me.NutIsland) || len(view.NutIsland.Pets) != 0 {
		t.Fatal("live pet movement/addition mutated an earlier saved snapshot")
	}
}
