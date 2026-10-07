package game

import "testing"

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
