package game

import (
	"reflect"
	"testing"
)

func TestIslandCoinCatalogPurchasesAreAtomicAndMaterialFree(t *testing.T) {
	paid := 0
	for _, recipe := range buildRecipes {
		if recipe.Category == "building" {
			if recipe.CoinCost != 0 || len(recipe.Cost) == 0 {
				t.Fatalf("structural building changed currency: %+v", recipe)
			}
			continue
		}
		paid++
		t.Run(recipe.ID, func(t *testing.T) {
			if recipe.CoinCost <= 0 || len(recipe.Cost) != 0 {
				t.Fatalf("purchased furnishing still requires materials: %+v", recipe)
			}
			w, p := buildingWorld(t)
			mustAct(t, w, Action{Action: "build_mode", Enabled: true})
			inventory := cloneMap(p.profile.Inventory)
			p.profile.HomeIsland.Chest = map[string]int{"wood": 100, "stone": 100}
			chest := cloneMap(p.profile.HomeIsland.Chest)
			p.profile.Coins = recipe.CoinCost - 1
			purchase := Action{Action: "build", Item: recipe.ID, X: 8}
			if err := w.Act("one", purchase); err == nil || p.profile.Coins != recipe.CoinCost-1 || len(p.profile.HomeIsland.Objects) != 1 {
				t.Fatal("insufficient balance bought an object or changed the balance")
			}
			p.profile.Coins = recipe.CoinCost
			invalid := purchase
			invalid.X = 200
			if err := w.Act("one", invalid); err == nil || p.profile.Coins != recipe.CoinCost || len(p.profile.HomeIsland.Objects) != 1 {
				t.Fatal("failed placement spent coins or placed an object")
			}
			mustAct(t, w, purchase)
			if p.profile.Coins != 0 || len(p.profile.HomeIsland.Objects) != 2 {
				t.Fatal("purchase did not exchange the exact price for one furnishing")
			}
			if !reflect.DeepEqual(inventory, p.profile.Inventory) || !reflect.DeepEqual(chest, p.profile.HomeIsland.Chest) {
				t.Fatal("coin purchase consumed personal or shared materials")
			}
			restored := New(w.Profiles()).Profiles()["one"]
			if restored.Coins != 0 || !reflect.DeepEqual(restored.HomeIsland.Objects, cloneIsland(p.profile.HomeIsland).Objects) {
				t.Fatal("purchase balance and furnishing did not persist together")
			}
			catalog := w.Snapshot("one").BuildCatalog
			for _, entry := range catalog {
				if entry.ID == recipe.ID && (entry.CoinCost != recipe.CoinCost || len(entry.Cost) != 0) {
					t.Fatalf("snapshot disagrees with the charged price: %+v", entry)
				}
			}
		})
	}
	if paid != 15 {
		t.Fatalf("expected all 15 decor, appliance, and garden items to use coins, got %d", paid)
	}
}

func TestIslandGuestCoinPurchasesUsePlacerBalanceAndPermissions(t *testing.T) {
	for _, kind := range []string{"chair", "oven", "tree"} {
		t.Run(kind, func(t *testing.T) {
			w, owner := buildingWorld(t)
			guest := addBuilder(t, w, owner)
			r, _ := buildRecipeFor(kind)
			guest.profile.Coins = r.CoinCost
			ownerCoins := owner.profile.Coins
			ownerInventory := cloneMap(owner.profile.Inventory)
			guestInventory := cloneMap(guest.profile.Inventory)
			owner.profile.HomeIsland.Chest = map[string]int{}
			owner.profile.HomeIsland.Builders[guest.profile.ID] = false
			purchase := Action{Action: "build", Item: kind, X: 8}
			if err := w.Act("two", purchase); err == nil || guest.profile.Coins != r.CoinCost {
				t.Fatal("coin balance bypassed permission or denied placement charged guest")
			}
			owner.profile.HomeIsland.Builders[guest.profile.ID] = true
			if err := w.Act("two", purchase); err != nil {
				t.Fatal(err)
			}
			if guest.profile.Coins != 0 || owner.profile.Coins != ownerCoins || len(owner.profile.HomeIsland.Objects) != 2 {
				t.Fatal("guest placement charged the wrong balance or did not place exactly one object")
			}
			if !reflect.DeepEqual(ownerInventory, owner.profile.Inventory) || !reflect.DeepEqual(guestInventory, guest.profile.Inventory) || len(owner.profile.HomeIsland.Chest) != 0 {
				t.Fatal("guest coin purchase used material inventories")
			}
		})
	}
}

func TestIslandPurchasedFurnishingMoveCancelPackAndReplacementAreFree(t *testing.T) {
	w, p := buildingWorld(t)
	mustAct(t, w, Action{Action: "build_mode", Enabled: true})
	r, _ := buildRecipeFor("chair")
	p.profile.Coins = r.CoinCost
	mustAct(t, w, Action{Action: "build", Item: "chair", X: 4})
	id := p.profile.HomeIsland.Objects[1].ID
	mustAct(t, w, Action{Action: "edit_build", Target: id})
	mustAct(t, w, Action{Action: "cancel_build"})
	mustAct(t, w, Action{Action: "edit_build", Target: id})
	mustAct(t, w, Action{Action: "move_build", Target: id, X: 6})
	mustAct(t, w, Action{Action: "edit_build", Target: id})
	mustAct(t, w, Action{Action: "pack_build", Target: id})
	if p.profile.Coins != 0 || p.profile.Inventory["build_chair"] != 1 {
		t.Fatal("editing or packing changed coin balance or lost the furnishing")
	}
	if err := w.Act("one", Action{Action: "build", Item: "chair", From: "build_chair", X: 200}); err == nil || p.profile.Inventory["build_chair"] != 1 {
		t.Fatal("failed packed placement consumed the packed furnishing")
	}
	mustAct(t, w, Action{Action: "build", Item: "chair", From: "build_chair", X: 8})
	if p.profile.Coins != 0 || p.profile.Inventory["build_chair"] != 0 || len(p.profile.HomeIsland.Objects) != 2 {
		t.Fatal("packed placement charged coins or duplicated the furnishing")
	}
	if err := w.Act("one", Action{Action: "build", Item: "chair", From: "build_chair", X: -8}); err == nil {
		t.Fatal("replaying packed placement created a free furnishing")
	}
	inventory := cloneMap(p.profile.Inventory)
	id = p.profile.HomeIsland.Objects[1].ID
	mustAct(t, w, Action{Action: "edit_build", Target: id})
	mustAct(t, w, Action{Action: "destroy_build", Target: id})
	if p.profile.Coins != 0 || !reflect.DeepEqual(inventory, p.profile.Inventory) || len(p.profile.HomeIsland.Objects) != 1 {
		t.Fatal("destroying a coin furnishing created a refund or retained the object")
	}
}
