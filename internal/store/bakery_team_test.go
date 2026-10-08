package store

import (
	"reflect"
	"testing"

	"cookiekill/internal/game"
)

func TestBakeryTeamRecipesBreaksAndSeparateTraysSurviveRestart(t *testing.T) {
	db, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := game.Profile{ID: "bakery_team", Name: "Baker", Bakery: true, Coins: 400, InventoryVersion: 3,
		BakeryStaff:       map[string]bool{"chef": true, "chef_2": true, "server": true, "server_2": true, "cashier": true, "cashier_2": true},
		BakeryStaffBreaks: map[string]bool{"chef_2": true, "server": true, "cashier_2": true},
		BakeryChefRecipe:  "berry_cookie", BakeryChefRecipes: []string{"berry_cookie", "sugar"}, Discovered: map[string]bool{"berry_cookie": true},
		BakeryCarry: map[string]int{"berry_cookie": 8}, BakeryServerCarry: map[string]map[string]int{"server_2": {"sugar": 12}},
		BakeryDisplay: map[string]int{"sugar": 350, "berry_cookie": 100}, BakeryPantry: map[string]int{"dough": 15, "berry": 2},
		BakeryFurnitureVersion: 4, BakeryFurniture: []game.BakeryFurniture{
			{ID: "oven", Kind: "oven", X: -5, Z: -5.5, ProductionKey: "bakery_oven"},
			{ID: "mixer", Kind: "mixer", X: 5, Z: -5.5, ProductionKey: "bakery_mixer"},
			{ID: "fridge", Kind: "fridge", X: 8, Z: -5.5},
			{ID: "cooling_table", Kind: "cooling_table", X: -1.8, Z: -5.5},
			{ID: "display", Kind: "display", X: 5, Z: 3},
			{ID: "display_second", Kind: "display", X: -5, Z: 3},
		}}
	if err := db.Save(map[string]game.Profile{p.ID: p}); err != nil {
		t.Fatal(err)
	}
	loaded, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	w := game.New(loaded)
	w.Join(p.ID, p.Name)
	got := w.Snapshot(p.ID).Me
	if !reflect.DeepEqual(got.BakeryStaff, p.BakeryStaff) || !reflect.DeepEqual(got.BakeryStaffBreaks, p.BakeryStaffBreaks) || !reflect.DeepEqual(got.BakeryChefRecipes, p.BakeryChefRecipes) {
		t.Fatalf("staff or recipe choices changed after restart: %+v", got.Profile)
	}
	if !reflect.DeepEqual(got.BakeryCarry, p.BakeryCarry) || !reflect.DeepEqual(got.BakeryServerCarry, p.BakeryServerCarry) || !reflect.DeepEqual(got.BakeryDisplay, p.BakeryDisplay) || !reflect.DeepEqual(got.BakeryPantry, p.BakeryPantry) {
		t.Fatal("restart lost or duplicated trays, fridge ingredients, or display stock")
	}
	displays := 0
	for _, f := range got.BakeryFurniture {
		if f.Kind == "display" {
			displays++
		}
	}
	if displays != 2 {
		t.Fatalf("got %d saved displays", displays)
	}
	// Published profiles must not expose mutable maps or slices owned by the world.
	got.BakeryStaffBreaks["chef"] = true
	got.BakeryChefRecipes[0] = "cake"
	got.BakeryServerCarry["server_2"]["sugar"] = 999
	unchanged := w.Snapshot(p.ID).Me
	if unchanged.BakeryStaffBreaks["chef"] || unchanged.BakeryChefRecipes[0] != "berry_cookie" || unchanged.BakeryServerCarry["server_2"]["sugar"] != 12 {
		t.Fatal("snapshot mutated authoritative bakery team state")
	}
}
