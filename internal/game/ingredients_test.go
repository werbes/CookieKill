package game

import (
	"reflect"
	"testing"
)

func TestBakersPantrySellsFourDistinctIngredientsOnlyInsideShop(t *testing.T) {
	w, p := testWorld()
	p.profile.Coins = 500
	nodePosition(w, p, "ingredient_shop")
	cx, cz := p.x, p.z
	oldCookies := p.profile.Inventory["sugar"]
	for _, ingredient := range bakingIngredients {
		before := p.profile.Coins
		mustAct(t, w, Action{Action: "buy_ingredient", Target: "ingredient_shop", Item: ingredient.ID, Amount: 5})
		if p.profile.Inventory[ingredient.ID] != 5 || p.profile.Coins != before-5*ingredient.Price {
			t.Fatalf("wrong purchase for %s", ingredient.ID)
		}
	}
	if p.profile.Inventory["sugar"] != oldCookies {
		t.Fatal("ingredient sugar was confused with baked sugar cookies")
	}
	for _, point := range []Point{{cx, cz + 5}, {cx + 6.2, cz}, {cx, cz - 5}} {
		p.x, p.z = point.X, point.Z
		before := cloneProfile(p.profile)
		if err := w.Act("one", Action{Action: "buy_ingredient", Target: "ingredient_shop", Item: "flour"}); err == nil || !reflect.DeepEqual(before, p.profile) {
			t.Fatal("shop purchase reached through a wall")
		}
	}
	p.x, p.z = cx, cz
	for _, item := range []string{"dough", "sugar", "berry", "unknown"} {
		before := cloneProfile(p.profile)
		if err := w.Act("one", Action{Action: "buy_ingredient", Target: "ingredient_shop", Item: item}); err == nil || !reflect.DeepEqual(before, p.profile) {
			t.Fatalf("invalid shop item %s altered inventory", item)
		}
	}
	p.island = "one"
	if err := w.Act("one", Action{Action: "buy_ingredient", Target: "ingredient_shop", Item: "flour"}); err == nil {
		t.Fatal("island coordinates bypassed the shop interior")
	}
	p.island = ""
	p.profile.Coins = 0
	if err := w.Act("one", Action{Action: "buy_ingredient", Target: "ingredient_shop", Item: "flour"}); err == nil {
		t.Fatal("unaffordable ingredient purchase succeeded")
	}
	p.profile.Coins = 500
	for i := range p.profile.BagSlots {
		p.profile.BagSlots[i] = Stack{Item: "stone", Count: inventoryLimit}
	}
	for i := range p.profile.Hotbar {
		p.profile.Hotbar[i] = Stack{Item: "stone", Count: inventoryLimit}
	}
	syncInventory(&p.profile)
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "buy_ingredient", Target: "ingredient_shop", Item: "flour"}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("full inventory was charged for an ingredient")
	}
}

func TestDoughComesFromChestsAndCannotBeDugOrBought(t *testing.T) {
	w, p := testWorld()
	chest := ""
	for _, n := range w.nodes {
		if n.Kind == "dough" {
			t.Fatal("ground dough patch still exists")
		}
		if n.Kind == "chest" {
			chest = n.ID
		}
	}
	if chest == "" {
		t.Fatal("new bakers have no chest source for dough")
	}
	nodePosition(w, p, chest)
	before := p.profile.Inventory["dough"]
	mustAct(t, w, Action{Action: "interact", Target: chest})
	if p.profile.Bakery || p.profile.Inventory["dough"] != before+3 {
		t.Fatal("chest did not provide starter dough before bakery ownership")
	}
	w.Tick(1)
	w.nodes = append(w.nodes, &Node{ID: "legacy_dough", Kind: "dough", X: p.x, Z: p.z, Amount: 4, Available: true, respawn: 25})
	beforeProfile := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "interact", Target: "legacy_dough"}); err == nil || !reflect.DeepEqual(beforeProfile, p.profile) {
		t.Fatal("legacy dough patch remained harvestable")
	}
	for _, target := range []string{"desert_trader", "desert_trader_2", "desert_trader_3", "ingredient_shop"} {
		nodePosition(w, p, target)
		if err := w.Act("one", Action{Action: "buy_ingredient", Target: target, Item: "dough"}); err == nil {
			t.Fatal("shop still sells premixed dough")
		}
	}
	first := w.Snapshot("one")
	if len(first.BakingIngredients) != 4 {
		t.Fatal("authoritative baking catalog missing")
	}
	first.BakingIngredients[0].Price = 999
	if w.Snapshot("one").BakingIngredients[0].Price != 4 {
		t.Fatal("snapshot baking catalog aliases server state")
	}
}

func TestIngredientsOnlySoldInsideDesertTraderHouses(t *testing.T) {
	w, p := testWorld()
	p.profile.Coins = 1000
	for _, trader := range []string{"desert_trader", "desert_trader_2", "desert_trader_3"} {
		nodePosition(w, p, trader)
		for item, price := range desertIngredientPrices {
			before, count := p.profile.Coins, p.profile.Inventory[item]
			mustAct(t, w, Action{Action: "buy_ingredient", Target: trader, Item: item, Amount: 2})
			if p.profile.Coins != before-price*2 || p.profile.Inventory[item] != count+2 {
				t.Fatalf("wrong purchase at %s: %s", trader, item)
			}
		}
		// Outside the entrance and side wall are within the old seven-metre
		// trade radius, but cannot buy ingredients through the building.
		cx, cz := p.x, p.z
		for _, point := range []Point{{cx, cz + 4.2}, {cx + 5, cz}, {cx, cz - 5}} {
			p.x, p.z = point.X, point.Z
			coins := p.profile.Coins
			if err := w.Act("one", Action{Action: "buy_ingredient", Target: trader, Item: "berry"}); err == nil || p.profile.Coins != coins {
				t.Fatalf("ingredients purchased outside %s at %+v", trader, point)
			}
		}
		p.x, p.z = cx, cz
		p.island = "home"
		if err := w.Act("one", Action{Action: "buy_ingredient", Target: trader, Item: "nut"}); err == nil {
			t.Fatal("home island coordinates bypassed trader interior")
		}
		p.island = ""
	}
	for _, target := range []string{"peace", "general_shop", "vending", "kitchen_shop"} {
		p.x, p.z = 140, 22
		if target == "peace" || target == "kitchen_shop" {
			nodePosition(w, p, target)
		} else if target == "vending" {
			p.x, p.z = 49, 20
		}
		for _, item := range []string{"berry", "nut", "cactus", "sea_salt", "protein_powder", "protein_drink"} {
			if err := w.Act("one", Action{Action: "buy_ingredient", Target: target, Item: item}); err == nil {
				t.Fatalf("%s sold raw ingredients", target)
			}
		}
	}
	for _, n := range w.nodes {
		if n.Kind == "general_shop" || n.Kind == "vending" {
			t.Fatal("removed city ingredient seller still exists")
		}
	}
	for _, prop := range w.layout.Props {
		if prop.ID == "general_store" || prop.Kind == "general_shop" {
			t.Fatal("removed provisions building still exists")
		}
	}
	for _, target := range []string{"general_shop", "vending"} {
		if err := w.Act("one", Action{Action: "trade", Target: target, Item: "protein_powder"}); err == nil {
			t.Fatal("old city purchase route remains available")
		}
	}
}

func TestIngredientPurchaseRejectsUnknownAndUnaffordableOrders(t *testing.T) {
	w, p := testWorld()
	nodePosition(w, p, "desert_trader")
	p.profile.Coins = 1
	for _, a := range []Action{{Action: "buy_ingredient", Target: "desert_trader", Item: "berry"}, {Action: "buy_ingredient", Target: "desert_trader", Item: "pearl"}, {Action: "buy_ingredient", Target: "desert_trader", Item: "berry", Amount: 21}} {
		if err := w.Act("one", a); err == nil || p.profile.Coins != 1 || p.profile.Inventory["berry"] != 0 {
			t.Fatal("invalid purchase altered player inventory or coins")
		}
	}
}
