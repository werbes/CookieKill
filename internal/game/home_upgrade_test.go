package game

import (
	"math"
	"reflect"
	"testing"
)

func TestHomeExpansionFivePurchasesAndDynamicSpace(t *testing.T) {
	w, p := buildingWorld(t)
	p.profile.Coins = 5000
	p.x, p.z = islandUpgradeX, islandUpgradeZ
	before := p.profile.Coins
	for level, price := range islandUpgradeCosts {
		mustAct(t, w, Action{Action: "upgrade_island", Target: "island_upgrade"})
		before -= price
		h := p.profile.HomeIsland
		if h.UpgradeLevel != level+1 || h.BuildRadius != 37+float64(level)*5 || h.WalkRadius != h.BuildRadius+8 || p.profile.Coins != before {
			t.Fatalf("expansion %d: %+v, coins=%d", level, h, p.profile.Coins)
		}
	}
	if err := w.Act("one", Action{Action: "upgrade_island", Target: "island_upgrade"}); err == nil || p.profile.Coins != before {
		t.Fatal("sixth expansion accepted or spent coins")
	}
	if p.profile.HomeIsland.NextUpgradeCost != 0 || p.profile.HomeIsland.MaxUpgrades != 5 {
		t.Fatal("maximum expansion catalog is wrong")
	}
	p.x, p.z = 50, 0
	mustAct(t, w, Action{Action: "build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "build", Item: "chair", X: 54})
	if w.islandBlocked(p, 64, 0) || !w.islandBlocked(p, 66, 0) {
		t.Fatal("expanded movement boundary does not use the purchased radius")
	}
	copy := cloneProfile(p.profile)
	normalizeIslandProfile(&copy)
	if copy.HomeIsland.UpgradeLevel != 5 || copy.HomeIsland.Objects[len(copy.HomeIsland.Objects)-1].X != 54 {
		t.Fatal("expanded building was lost on profile normalization")
	}
}

func TestHomeExpansionRejectsUnauthorizedAndDistantPurchases(t *testing.T) {
	w, p := buildingWorld(t)
	p.profile.Coins = 5000
	for _, pos := range [][3]float64{{0, 0, 0}, {-5, 4, 24}} {
		p.x, p.y, p.z = pos[0], pos[1], pos[2]
		if err := w.Act("one", Action{Action: "upgrade_island", Target: "island_upgrade"}); err == nil {
			t.Fatal("remote expansion accepted")
		}
	}
	p.x, p.y, p.z = -5, 0, 24
	p.profile.Coins = 149
	if err := w.Act("one", Action{Action: "upgrade_island", Target: "island_upgrade"}); err == nil || p.profile.Coins != 149 {
		t.Fatal("unaffordable expansion changed wallet")
	}
	guest := addBuilder(t, w, p)
	guest.x, guest.z, guest.profile.Coins = -5, 24, 5000
	if err := w.Act("two", Action{Action: "upgrade_island", Target: "island_upgrade"}); err == nil {
		t.Fatal("guest builder expanded another player's island")
	}
}

func TestPondMigrationExpansionAndAtomicFootprint(t *testing.T) {
	w, p := buildingWorld(t)
	p.profile.Coins = 1000
	p.profile.HomeIsland.Objects = append(p.profile.HomeIsland.Objects, BuildObject{ID: "old_pond", Kind: "pond", X: 8, Adopted: "turtle"})
	normalizeIslandProfile(&p.profile)
	o := p.profile.HomeIsland.Objects[1]
	if o.PondCapacity != 1 || !reflect.DeepEqual(o.AdoptedAnimals, []string{"turtle"}) || o.Adopted != "turtle" {
		t.Fatalf("legacy rescue migration failed: %+v", o)
	}
	p.x, p.z = 8, 4.5
	for capacity := 2; capacity <= 4; capacity++ {
		mustAct(t, w, Action{Action: "upgrade_pond", Target: o.ID})
		o = p.profile.HomeIsland.Objects[1]
		r, _ := buildObjectRecipe(o)
		if o.PondCapacity != capacity || len(o.AdoptedAnimals) != 1 || r.Width != float64(4+capacity) || r.Depth != float64(3+capacity) {
			t.Fatalf("pond upgrade %d failed: %+v %+v", capacity, o, r)
		}
	}
	if p.profile.Coins != 600 || o.PondUpgradeCost != 0 {
		t.Fatal("pond price or maximum incorrect")
	}
	if err := w.Act("one", Action{Action: "upgrade_pond", Target: o.ID}); err == nil || p.profile.Coins != 600 {
		t.Fatal("fifth pond place accepted")
	}
	copy := cloneIsland(p.profile.HomeIsland)
	copy.Objects[1].AdoptedAnimals[0] = "fish"
	if p.profile.HomeIsland.Objects[1].AdoptedAnimals[0] != "turtle" {
		t.Fatal("pond snapshot aliases live rescued animals")
	}
	// The next size would overlap another appliance. Failure must keep both
	// the original animal/capacity and the wallet untouched.
	p.profile.HomeIsland.Objects[1] = BuildObject{ID: "small", Kind: "pond", X: 8, Adopted: "fish"}
	p.profile.HomeIsland.Objects = append(p.profile.HomeIsland.Objects, BuildObject{ID: "neighbor", Kind: "pond", X: 13.2})
	normalizePond(&p.profile.HomeIsland.Objects[1])
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "upgrade_pond", Target: "small"}); err == nil {
		t.Fatal("pond expansion through another appliance accepted")
	}
	if !reflect.DeepEqual(before, p.profile) {
		t.Fatal("failed pond expansion mutated profile")
	}
}

func TestPondPackedCapacityAndGroundPlacement(t *testing.T) {
	w, p := buildingWorld(t)
	o := BuildObject{ID: "wide_pond", Kind: "pond", X: 8, PondCapacity: 4}
	normalizePond(&o)
	p.profile.HomeIsland.Objects = append(p.profile.HomeIsland.Objects, o)
	mustAct(t, w, Action{Action: "build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "edit_build", Target: o.ID})
	mustAct(t, w, Action{Action: "pack_build", Target: o.ID})
	if p.profile.Inventory["build_pond:capacity4"] != 1 {
		t.Fatal("packing lost pond upgrade")
	}
	mustAct(t, w, Action{Action: "build", Item: "pond", From: "build_pond:capacity4", X: 8})
	if p.profile.HomeIsland.Objects[1].PondCapacity != 4 || p.profile.Inventory["build_pond:capacity4"] != 0 {
		t.Fatal("placing packed pond lost capacity or retained packed item")
	}
	for _, kind := range []string{"pond", "camel_pen"} {
		if err := w.validatePlacement(p, p.profile.HomeIsland, kind, "", Action{X: -8, Y: 1}); err == nil {
			t.Fatal("floating water or camel enclosure accepted")
		}
	}
}

func TestExpandedPondAdoptsFourAnimalsAndRejectsFifth(t *testing.T) {
	w, p := buildingWorld(t)
	p.profile.HomeIsland.Objects = append(p.profile.HomeIsland.Objects, BuildObject{ID: "rescue_pond", Kind: "pond", X: 8, PondCapacity: 4})
	p.island = ""
	var rescue *Animal
	for _, animal := range w.animals {
		if animal.Species == "fish" {
			rescue = animal
			break
		}
	}
	if rescue == nil {
		t.Fatal("missing ocean rescue fixture")
	}
	p.x, p.z = rescue.X, rescue.Z
	for i := 0; i < 4; i++ {
		rescue.Health, rescue.Need, rescue.Egg = 25, "wounded", false
		mustAct(t, w, Action{Action: "adopt_animal", Target: rescue.ID})
	}
	rescue.Health, rescue.Need = 25, "wounded"
	if err := w.Act("one", Action{Action: "adopt_animal", Target: rescue.ID}); err == nil || rescue.Health != 25 || p.profile.Rescues != 4 {
		t.Fatal("full pond took a fifth animal or modified the wild animal")
	}
	if len(p.profile.HomeIsland.Objects[1].AdoptedAnimals) != 4 || !buildHasContents(p.profile.HomeIsland, p.profile.HomeIsland.Objects[1]) {
		t.Fatal("four saved pond residents missing")
	}
}

func TestCamelEnclosureFenceGateAndRotatedSlots(t *testing.T) {
	o := BuildObject{ID: "pen", Kind: "camel_pen", X: 10, Z: 10}
	r, ok := buildObjectRecipe(o)
	if !ok {
		t.Fatal("camel enclosure is not buildable")
	}
	for _, spot := range [][3]float64{{10, 10, 0}, {10, 14, 0}, {6, 10, 1}, {14, 10, 1}, {10, 6, 1}, {13, 14, 1}} {
		if objectBlocks(o, r, spot[0], 0, spot[1], .42) != (spot[2] == 1) {
			t.Fatalf("wrong fence/gate collision at %+v", spot)
		}
	}
	for _, angle := range []float64{0, 45, 90, 225} {
		o.Rotation = angle
		slots := camelPenSlots(o)
		if len(slots) != 4 {
			t.Fatal("pen must hold exactly four camels")
		}
		for _, slot := range slots {
			x, z := objectLocal(o, slot.X, slot.Z)
			if math.Abs(math.Abs(x)-1.8) > 1e-8 || math.Abs(math.Abs(z)-1.6) > 1e-8 || objectBlocks(o, r, slot.X, 0, slot.Z, .7) {
				t.Fatalf("unsafe rotated pen slot: %+v", slot)
			}
		}
	}
}

func TestCamelRestingSlotsStayClearWhileAssignedCamelIsAway(t *testing.T) {
	w, p := buildingWorld(t)
	pen := BuildObject{ID: "home_pen", Kind: "camel_pen", X: 8, Z: 0, Rotation: 45}
	p.profile.HomeIsland.Objects = append(p.profile.HomeIsland.Objects, pen)
	p.profile.Camels = []Camel{{ID: "one_camel_1", X: 120, Z: -70, HomePenID: pen.ID, HomePenSlot: 0}}
	slot := camelPenSlots(pen)[0]
	if err := w.validatePlacement(p, p.profile.HomeIsland, "chair", "", Action{X: slot.X, Z: slot.Z}); err == nil {
		t.Fatal("furniture blocked a reserved home position while camel was on the mainland")
	}
	if err := w.validatePlacement(p, p.profile.HomeIsland, "chair", "", Action{X: -5, Z: 0}); err != nil {
		t.Fatal("unrelated furniture position rejected:", err)
	}
}
