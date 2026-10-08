package game

import (
	"math"
	"reflect"
	"testing"
)

func TestBakeryFurnitureMigratesOnceAndSnapshotsDetach(t *testing.T) {
	w := New(map[string]Profile{"one": {ID: "one", Bakery: true, Paint: map[string]string{"oven": "teal"}}})
	w.Join("one", "Baker")
	p := w.players["one"]
	if len(p.profile.BakeryFurniture) != 11 || p.profile.BakeryFurnitureVersion != 4 || p.profile.BakeryFurniture[0].Color != "teal" {
		t.Fatal("legacy bakery did not receive one painted layout")
	}
	nodePosition(w, p, "bakery_plot_0")
	mustAct(t, w, Action{Action: "buy_land", Target: "bakery_plot_0"})
	snapshot := w.Snapshot("one")
	snapshot.Me.BakeryFurniture[0].Color = "red"
	for i := range snapshot.Nodes {
		if snapshot.Nodes[i].Kind == "bakery_plot" && snapshot.Nodes[i].Owner == "one" {
			snapshot.Nodes[i].BakeryFurniture[0].X = 999
		}
	}
	if p.profile.BakeryFurniture[0].Color != "teal" || p.profile.BakeryFurniture[0].X != -5 {
		t.Fatal("snapshot furniture aliases live state")
	}
	p.profile.BakeryFurniture = nil
	restarted := New(w.Profiles())
	restarted.Join("one", "Baker")
	if len(restarted.players["one"].profile.BakeryFurniture) != 0 {
		t.Fatal("intentionally empty saved bakery was seeded again")
	}
}

func TestBakeryFurnitureRequiresOwnerBuildModeAndVisibleEdit(t *testing.T) {
	w, p := businessWorld(t)
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "bakery_place", Item: "rug", X: 3, Z: 6}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("placement outside explicit build mode changed the bakery")
	}
	w.Join("two", "Visitor")
	visitor := w.players["two"]
	visitor.x, visitor.z = p.x, p.z
	if err := w.Act("two", Action{Action: "bakery_build_mode", Enabled: true}); err == nil {
		t.Fatal("visitor entered the owner's bakery build mode")
	}
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	plot, _ := w.bakeryPlot(p)
	p.x, p.z = plot.X, plot.Z+3
	if err := w.Act("one", Action{Action: "bakery_edit", Target: "oven"}); err == nil {
		t.Fatal("editing reached the oven through the partition wall")
	}
	p.x, p.z = plot.X, plot.Z-3
	mustAct(t, w, Action{Action: "bakery_edit", Target: "oven"})
	if !reflect.DeepEqual(before, p.profile) {
		t.Fatal("taking edit control removed or changed the original furnishing")
	}
	mustAct(t, w, Action{Action: "bakery_cancel_edit"})
	if p.editingBakeryFurnitureID != "" || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("cancel did not preserve the original layout")
	}
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: false})
	if err := w.Act("one", Action{Action: "bakery_pack", Target: "oven"}); err == nil {
		t.Fatal("packing bypassed the build-mode gate")
	}
}

func TestBakeryInvalidPlacementIsAtomic(t *testing.T) {
	w, p := businessWorld(t)
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	for _, a := range []Action{
		{Item: "counter", X: 5, Z: -1}, // Embedded in the partition.
		{Item: "counter", X: 0, Z: 0},  // Kitchen doorway.
		{Item: "plant", X: 5, Z: 4.8},  // Display service point.
		{Item: "plant", X: 9.5, Z: 6},  // Outer wall.
		{Item: "chair", X: 3, Z: -3},   // Wrong room.
		{Item: "oven", X: -7, Z: -5},   // Duplicate core station.
		{Item: "plant", X: 7, Z: 6, Rotation: 22.5},
		{Item: "plant", X: math.NaN(), Z: 6},
		{Item: "plant", X: -5, Z: 3}, // Existing table.
	} {
		a.Action = "bakery_place"
		before := cloneProfile(p.profile)
		if err := w.Act("one", a); err == nil || !reflect.DeepEqual(before, p.profile) {
			t.Fatalf("invalid placement was accepted or charged: %+v", a)
		}
	}
	beforeCoins := p.profile.Coins
	mustAct(t, w, Action{Action: "bakery_place", Item: "plant", X: 8, Z: 6})
	if p.profile.Coins != beforeCoins-15 || len(p.profile.BakeryFurniture) != 12 {
		t.Fatal("valid furnishing did not charge its catalog price exactly once")
	}
}

func TestBakeryPackingPreservesColorStockAndDynamicStation(t *testing.T) {
	w, p := businessWorld(t)
	p.profile.BakeryDisplay["sugar"], p.profile.BakeryPantry["dough"] = 7, 8
	inventorySugar := p.profile.Inventory["sugar"]
	nodePosition(w, p, "bakery_plot_0_display")
	mustAct(t, w, Action{Action: "bakery_unstock", Target: "bakery_plot_0_display", Item: "sugar", Amount: 7})
	setItem(p, "paint_teal", 1)
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "bakery_edit", Target: "display"})
	mustAct(t, w, Action{Action: "bakery_paint", Target: "display", Item: "teal"})
	mustAct(t, w, Action{Action: "bakery_pack", Target: "display"})
	if p.profile.Inventory["bakery_display:teal"] != 1 || p.profile.Inventory["sugar"] != inventorySugar+7 || p.profile.BakeryDisplay["sugar"] != 0 || p.profile.BakeryPantry["dough"] != 8 {
		t.Fatal("packing lost contents or paint")
	}
	if _, _, ok := w.bakeryStationPosition(p, "display"); ok {
		t.Fatal("packed station still has an interaction point")
	}
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "bakery_pack", Target: "display"}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("replayed packing duplicated inventory")
	}
	mustAct(t, w, Action{Action: "bakery_place", Item: "display", From: "bakery_display:teal", X: 6, Z: 4, Rotation: 90})
	plot, _ := w.bakeryPlot(p)
	point, rotation, ok := w.bakeryStationPosition(p, "display")
	if !ok || point != (Point{plot.X + 6, plot.Z + 4}) || rotation != 90 || p.profile.Inventory["bakery_display:teal"] != 0 || p.profile.Coins != before.Coins {
		t.Fatal("packed replacement lost its transform or charged again")
	}
	if !w.solidBlockedAt(point.X, 0, point.Z, .1) || w.solidBlockedAt(plot.X+5, 0, plot.Z+3, .1) {
		t.Fatal("station colliders did not follow the moved furnishing")
	}
	nodePosition(w, p, "bakery_plot_0_display")
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: false})
	if err := w.requireBakeryStation(p, "display", "bakery_plot_0_display"); err != nil {
		t.Fatal(err)
	}
	mustAct(t, w, Action{Action: "bakery_stock", Target: "bakery_plot_0_display", Item: "sugar", Amount: 7})
	saved := w.Profiles()
	restarted := New(saved)
	restarted.Join("one", "Baker")
	q := restarted.players["one"]
	if !reflect.DeepEqual(q.profile.BakeryFurniture, p.profile.BakeryFurniture) || q.profile.BakeryDisplay["sugar"] != 7 || q.profile.BakeryPantry["dough"] != 8 {
		t.Fatal("restart lost the furnished layout or stored bakery supplies")
	}
}

func TestBakeryChairOccupancyMovementAndRelease(t *testing.T) {
	w, p := businessWorld(t)
	plot, _ := w.bakeryPlot(p)
	w.Join("two", "Visitor")
	v := w.players["two"]
	v.x, v.z = plot.X-2.2, plot.Z+3
	seat := plot.ID + "_seat_chair_2"
	if err := w.Act("two", Action{Action: "bakery_sit", Target: seat}); err != nil {
		t.Fatal(err)
	}
	w.Tick(.1)
	if v.bakerySeatID != seat || v.posture != "sitting" || v.x != plot.X-3.5 {
		t.Fatal("seated visitor drifted or was ejected by the chair collider")
	}
	p.x, p.z = plot.X-2.2, plot.Z+3
	if err := w.Act("one", Action{Action: "bakery_sit", Target: seat}); err == nil {
		t.Fatal("two people occupied the same chair")
	}
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	if err := w.Act("one", Action{Action: "bakery_edit", Target: "chair_2"}); err == nil {
		t.Fatal("occupied chair could be edited")
	}
	w.Input("two", Input{X: 1})
	w.Tick(.05)
	if v.bakerySeatID != "" || v.posture == "sitting" || w.solidBlockedAt(v.x, 0, v.z, .42) {
		t.Fatal("standing left the visitor trapped in the solid chair")
	}
	v.x, v.z = plot.X-2.2, plot.Z+3
	if err := w.Act("two", Action{Action: "bakery_sit", Target: seat}); err != nil {
		t.Fatal(err)
	}
	w.ReleaseBakeryPlot("one")
	if v.bakerySeatID != "" || v.posture == "sitting" || w.solidBlockedAt(v.x, 0, v.z, .42) {
		t.Fatal("departed owner left a visitor sitting in a removed chair")
	}
}

func TestClaimingSavedBakeryClearsEveryRotatedFurnishing(t *testing.T) {
	w, p := testWorld()
	plot := w.layout.Plots[0]
	p.profile.Bakery, p.profile.BakeryFurnitureVersion = true, 1
	p.profile.BakeryFurniture = []BakeryFurniture{{ID: "counter", Kind: "counter", X: 0, Z: 0}, {ID: "oven", Kind: "oven", X: -5, Z: -5.5, Rotation: 45}}
	p.x, p.z = plot.X, plot.Z
	w.Join("two", "Visitor")
	v := w.players["two"]
	v.x, v.z = plot.X-5, plot.Z-5.5
	mustAct(t, w, Action{Action: "buy_land", Target: plot.ID})
	for _, moved := range []*player{p, v} {
		if w.solidBlockedAt(moved.x, 0, moved.z, .42) || moved.x == plot.X && moved.z == plot.Z {
			t.Fatal("claim did not clear all furniture or chose an obstructed center")
		}
	}
}
