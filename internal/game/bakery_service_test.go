package game

import (
	"reflect"
	"testing"
)

func TestCustomerCanBeServedDirectlyBeyondDisplayInteractionRange(t *testing.T) {
	w, p := businessWorld(t)
	nodePosition(w, p, "bakery_plot_0_display")
	mustAct(t, w, Action{Action: "bakery_stock", Item: "sugar", Amount: 5})
	c := firstWaitingCustomer(t, w, p)
	p.x, p.z = c.x+3, c.z
	if w.requireBakeryStation(p, "display", "") == nil {
		t.Fatal("fixture must be out of display interaction range")
	}
	coins := p.profile.Coins
	mustAct(t, w, Action{Action: "bakery_serve", Target: c.ID})
	if p.profile.Coins != coins+10 || p.profile.BakeryDisplay["sugar"] != 0 || c.Phase != "leaving" {
		t.Fatal("direct service did not exchange display goods for the order price")
	}
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "bakery_serve", Target: c.ID}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("a repeated serve paid twice")
	}
}

func TestDirectCustomerServiceRejectsRemoteHiddenAndForeignOrders(t *testing.T) {
	w, p := businessWorld(t)
	p.profile.BakeryDisplay["sugar"] = 5
	c := &bakeryCustomer{BakeryCustomerView: BakeryCustomerView{ID: "order", Item: "sugar", Amount: 5, Phase: "waiting"}, x: 32.5, z: 76.8}
	p.bakery.customers, p.bakery.routesDirty = []*bakeryCustomer{c}, false
	for _, scenario := range []struct {
		name    string
		x, y, z float64
		island  string
		build   bool
	}{
		{"far", 27, 0, 76.8, "", false},
		{"outside wall", 35, 0, 76.8, "", false},
		{"above", 31, 5, 76.8, "", false},
		{"other island", 31, 0, 76.8, "one", false},
		{"building", 31, 0, 76.8, "", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			p.x, p.jumpOffset, p.z, p.island, p.bakeryBuildMode = scenario.x, scenario.y, scenario.z, scenario.island, scenario.build
			before := cloneProfile(p.profile)
			if err := w.Act("one", Action{Action: "bakery_serve", Target: c.ID}); err == nil || !reflect.DeepEqual(before, p.profile) {
				t.Fatal("inaccessible order was served or mutated inventory")
			}
		})
	}
	w.Join("two", "Visitor")
	visitor := w.players["two"]
	visitor.x, visitor.z = 31, 76.8
	if err := w.Act("two", Action{Action: "bakery_serve", Target: c.ID}); err == nil || visitor.profile.Coins != 0 || c.Phase != "waiting" {
		t.Fatal("visitor collected the owner's order")
	}
}
