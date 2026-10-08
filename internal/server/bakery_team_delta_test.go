package server

import (
	"testing"

	"cookiekill/internal/game"
)

func TestBakeryTeamChangesSurviveDeltaReconstruction(t *testing.T) {
	encoder := &stateEncoder{delta: true}
	receiver := &testDeltaReceiver{}
	before := game.Snapshot{}
	before.Me.BakeryStaff = map[string]bool{"chef": true, "server": true, "cashier": true}
	before.Me.BakeryChefRecipes = []string{"sugar"}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, before)), before)
	encoder.commit(before)
	after := before
	after.Me.BakeryStaff = map[string]bool{"chef": true, "chef_2": true, "server": true, "server_2": true, "cashier": true, "cashier_2": true}
	after.Me.BakeryStaffBreaks = map[string]bool{"chef_2": true, "cashier": true}
	after.Me.BakeryChefRecipes = []string{"sugar", "berry_cookie"}
	after.Me.BakeryServerCarry = map[string]map[string]int{"server_2": {"berry_cookie": 7}}
	after.Nodes = []game.Node{{ID: "plot_display", Kind: "bakery_display", Stock: map[string]int{"sugar": 300}}, {ID: "plot_station_second", Kind: "bakery_display", Stock: map[string]int{"sugar": 15}}, {ID: "plot_chef_2", Kind: "bakery_chef", Phase: "On break"}}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, after)), after)
	encoder.commit(after)
	resumed := after
	resumed.Me.BakeryStaffBreaks = map[string]bool{}
	resumed.Me.BakeryChefRecipes = []string{"berry_cookie"}
	resumed.Me.BakeryServerCarry = map[string]map[string]int{}
	resumed.Nodes = []game.Node{{ID: "plot_display", Kind: "bakery_display", Stock: map[string]int{"sugar": 300}}, {ID: "plot_station_second", Kind: "bakery_display", Stock: map[string]int{"sugar": 15, "berry_cookie": 7}}, {ID: "plot_chef_2", Kind: "bakery_chef", Phase: "Ready to bake"}}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, resumed)), resumed)
}
