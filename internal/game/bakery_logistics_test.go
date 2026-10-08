package game

import (
	"math"
	"reflect"
	"testing"
)

func TestFridgeMigrationPreservesSuppliesAndExistingLayout(t *testing.T) {
	w, p := businessWorld(t)
	p.profile.BakeryPantry = map[string]int{"dough": 47, "berry": 11}
	p.profile.BakeryDisplay = map[string]int{"sugar": 87}
	objects := []BakeryFurniture{}
	for _, f := range p.profile.BakeryFurniture {
		if f.Kind != "fridge" {
			objects = append(objects, f)
		}
	}
	p.profile.BakeryFurniture = objects
	p.profile.BakeryFurnitureVersion = 3
	before := append([]BakeryFurniture{}, objects...)
	normalizeBakeryFurniture(&p.profile)
	if !reflect.DeepEqual(before, p.profile.BakeryFurniture[:len(before)]) || p.profile.BakeryPantry["dough"] != 47 || p.profile.BakeryPantry["berry"] != 11 || p.profile.BakeryDisplay["sugar"] != 87 {
		t.Fatal("fridge migration moved existing fixtures or lost stock")
	}
	if _, _, ok := w.bakeryStationPosition(p, "fridge"); !ok || !bakeryLayoutReachable(p.profile.BakeryFurniture) {
		t.Fatal("migration did not add an accessible physical fridge")
	}
	state := append([]BakeryFurniture{}, p.profile.BakeryFurniture...)
	normalizeBakeryFurniture(&p.profile)
	if !reflect.DeepEqual(state, p.profile.BakeryFurniture) {
		t.Fatal("fridge migration repeated")
	}
	r, _ := bakeryFurnitureRecipeFor("display")
	if r.Width != 4.2 || r.Depth != 1.1 || bakeryDisplayCapacity != 300 {
		t.Fatal("display did not grow physically and in capacity")
	}
}

func TestFridgeTransfersRequirePhysicalStationAndRemainAtomic(t *testing.T) {
	w, p := businessWorld(t)
	setItem(p, "flour", 8)
	nodePosition(w, p, "bakery_plot_0_mixer")
	if err := w.Act("one", Action{Action: "bakery_fridge_deposit", Item: "flour", Amount: 5}); err == nil {
		t.Fatal("mixer remotely accessed fridge")
	}
	nodePosition(w, p, "bakery_plot_0_fridge")
	mustAct(t, w, Action{Action: "bakery_fridge_deposit", Target: "bakery_plot_0_fridge", Item: "flour", Amount: 5})
	if p.profile.BakeryPantry["flour"] != 5 || p.profile.Inventory["flour"] != 3 {
		t.Fatal("fridge deposit did not transfer exact inventory")
	}
	mustAct(t, w, Action{Action: "bakery_pantry_withdraw", Item: "flour", Amount: 2})
	if p.profile.BakeryPantry["flour"] != 3 || p.profile.Inventory["flour"] != 5 {
		t.Fatal("compatible pantry alias did not use fridge storage")
	}
	p.profile.BakeryPantry = map[string]int{"flour": bakeryPantryCapacity}
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "bakery_fridge_deposit", Item: "flour", Amount: 1}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("full fridge lost deposited ingredient")
	}
	p.editingBakeryFurnitureID = "fridge"
	if err := w.Act("one", Action{Action: "bakery_fridge_withdraw", Item: "flour", Amount: 1}); err == nil {
		t.Fatal("edited fridge remained usable")
	}
}

func TestLargerDisplayMigrationMovesOnlyDisplayWhenOldFootprintNoLongerFits(t *testing.T) {
	w, p := businessWorld(t)
	for i := range p.profile.BakeryFurniture {
		if p.profile.BakeryFurniture[i].Kind == "display" {
			p.profile.BakeryFurniture[i].X = 7.9
		}
	}
	p.profile.BakeryFurnitureVersion = 3
	p.profile.BakeryDisplay["sugar"] = 99
	p.profile.BakeryCarry["sugar"] = 7
	before := append([]BakeryFurniture{}, p.profile.BakeryFurniture...)
	normalizeBakeryFurniture(&p.profile)
	for i, f := range p.profile.BakeryFurniture {
		if f.Kind == "display" {
			r, _ := bakeryFurnitureRecipeFor(f.Kind)
			if !bakeryFurnitureInRoom(f, r) {
				t.Fatal("larger display remained outside walls")
			}
			continue
		}
		if f != before[i] {
			t.Fatal("larger display migration moved an unrelated furnishing")
		}
	}
	if p.profile.BakeryDisplay["sugar"] != 99 || p.profile.BakeryCarry["sugar"] != 7 || !bakeryLayoutReachable(p.profile.BakeryFurniture) {
		t.Fatal("display migration lost goods or blocked staff routes")
	}
	if _, _, ok := w.bakeryStationPosition(p, "display"); !ok && p.profile.Recovery["bakery_display"] != 1 {
		t.Fatal("display neither relocated nor saved packed")
	}
}

func waitServerCarry(t *testing.T, w *World, p *player) {
	t.Helper()
	for i := 0; i < 400 && bakeryStockCount(p.profile.BakeryCarry) == 0; i++ {
		w.Tick(.05)
	}
	if bakeryStockCount(p.profile.BakeryCarry) == 0 {
		t.Fatalf("server did not collect cooling goods: %+v", p.bakery)
	}
}
func bakeryFinishedTotal(p *player) int {
	return bakeryStockCount(p.profile.BakeryCarry) + bakeryStockCount(p.profile.BakeryDisplay) + bakeryStockCount(productionState(&p.profile, "bakery_cooling_table").Output)
}

func TestServerPhysicallyCarriesStockAndConservesAcrossInterruptions(t *testing.T) {
	w, p := businessWorld(t)
	mustAct(t, w, Action{Action: "bakery_hire", Item: "server"})
	productionState(&p.profile, "bakery_cooling_table").Output = map[string]int{"sugar": 45, "berry_cookie": 3}
	waitServerCarry(t, w, p)
	if bakeryStockCount(p.profile.BakeryCarry) != 20 || bakeryFinishedTotal(p) != 48 || bakeryStockCount(p.profile.BakeryDisplay) != 0 {
		t.Fatal("pickup duplicated stock or teleported it into display")
	}
	worker := p.bakery.workers["server"]
	start := Point{worker.x, worker.z}
	for i := 0; i < 5; i++ {
		before := Point{worker.x, worker.z}
		w.Tick(.05)
		if distance(before.X, before.Z, worker.x, worker.z) > .131 {
			t.Fatal("server teleported instead of walking")
		}
	}
	if distance(start.X, start.Z, worker.x, worker.z) < .3 {
		t.Fatal("loaded server did not walk")
	}
	copy := w.Snapshot("one")
	copy.Me.BakeryCarry["sugar"] = 999
	if p.profile.BakeryCarry["sugar"] == 999 {
		t.Fatal("carry snapshot aliases saved stock")
	}
	p.editingBakeryFurnitureID = "display"
	carry := cloneMap(p.profile.BakeryCarry)
	advance(w, 3)
	if !reflect.DeepEqual(carry, p.profile.BakeryCarry) || bakeryFinishedTotal(p) != 48 {
		t.Fatal("display edit lost or delivered carried stock")
	}
	p.editingBakeryFurnitureID = ""
	w.bakeryFurnitureChanged(p)
	// Logout while carrying must preserve the tray across profile reload.
	w.Leave("one")
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	p = restored.players["one"]
	if !reflect.DeepEqual(carry, p.profile.BakeryCarry) || bakeryFinishedTotal(p) != 48 {
		t.Fatal("logout/restart lost carried cookies")
	}
	nodePosition(restored, p, "bakery_plot_1")
	mustAct(t, restored, Action{Action: "buy_land", Target: "bakery_plot_1"})
	advance(restored, 45)
	if p.profile.BakeryDisplay["sugar"] != 45 || p.profile.BakeryDisplay["berry_cookie"] != 3 || bakeryStockCount(p.profile.BakeryCarry) != 0 || bakeryFinishedTotal(p) != 48 {
		t.Fatalf("resumed server did not deliver every cookie once: carry=%v display=%v", p.profile.BakeryCarry, p.profile.BakeryDisplay)
	}
}

func TestServerFullDisplayKeepsRemainderAndChefRequiresReachableFridge(t *testing.T) {
	w, p := businessWorld(t)
	mustAct(t, w, Action{Action: "bakery_hire", Item: "server"})
	productionState(&p.profile, "bakery_cooling_table").Output["sugar"] = 20
	waitServerCarry(t, w, p)
	p.profile.BakeryDisplay["sugar"] = 299
	advance(w, 12)
	if p.profile.BakeryDisplay["sugar"] != 300 || p.profile.BakeryCarry["sugar"] != 19 {
		t.Fatal("server overflowed display or dropped carried remainder")
	}
	p.disconnected = true
	before := cloneProfile(p.profile)
	advance(w, 4)
	if !reflect.DeepEqual(before.BakeryCarry, p.profile.BakeryCarry) || !reflect.DeepEqual(before.BakeryDisplay, p.profile.BakeryDisplay) || !reflect.DeepEqual(before.Production, p.profile.Production) {
		t.Fatal("disconnected staff changed inventory")
	}
	p.disconnected = false
	p.profile.BakeryDisplay["sugar"] -= 19
	advance(w, 1)
	if p.profile.BakeryDisplay["sugar"] != 300 || len(p.profile.BakeryCarry) != 0 {
		t.Fatal("server failed to fill newly freed display space")
	}
	// A saved obstructed route must stop ingredient consumption, not permit a
	// remote fridge read through furniture.
	p.profile.BakeryStaff["chef"] = true
	p.profile.BakeryPantry["dough"] = 3
	for _, f := range p.profile.BakeryFurniture {
		if f.Kind == "fridge" {
			p.profile.BakeryFurniture = append(p.profile.BakeryFurniture, BakeryFurniture{ID: "bad_saved_counter", Kind: "counter", X: f.X, Z: f.Z + 1.8})
			break
		}
	}
	w.bakeryFurnitureChanged(p)
	advance(w, 12)
	if p.profile.BakeryPantry["dough"] != 3 || p.bakery.fridgeAccessible {
		t.Fatal("chef consumed inaccessible fridge ingredients")
	}
	if math.IsNaN(p.bakery.workers["server"].x) {
		t.Fatal("invalid staff route")
	}
}
