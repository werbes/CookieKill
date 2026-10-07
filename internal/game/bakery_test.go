package game

import (
	"fmt"
	"reflect"
	"testing"
)

func TestBakeryDisconnectReleasesPlotAndFreeClaimRestoresProgress(t *testing.T) {
	w, p := testWorld()
	p.profile.Coins = 1000
	nodePosition(w, p, "bakery_plot_0")
	mustAct(t, w, Action{Action: "buy_land", Target: "bakery_plot_0"})
	mustAct(t, w, Action{Action: "upgrade"})
	nodePosition(w, p, "kitchen_shop")
	for _, item := range []string{"oven", "mixer", "display", "paint_teal"} {
		mustAct(t, w, Action{Action: "trade", Target: "kitchen_shop", Item: item})
	}
	nodePosition(w, p, "bakery_plot_0")
	mustAct(t, w, Action{Action: "paint", Target: "walls", Item: "teal"})
	saved := cloneProfile(p.profile)
	w.Join("one", "Replacement socket")
	if w.plotOwner("bakery_plot_0") != "one" {
		t.Fatal("replacing a live connection released its occupied plot")
	}
	nodePosition(w, p, "bakery_plot_1")
	if err := w.Act("one", Action{Action: "buy_land", Target: "bakery_plot_1"}); err == nil {
		t.Fatal("a connected baker claimed a second plot")
	}
	w.ReleaseBakeryPlot("one")
	if w.players["one"] == nil || w.plotOwner("bakery_plot_0") != "" || p.profile.BakeryPlot != "" {
		t.Fatal("disconnect must release occupancy immediately while preserving combat grace")
	}
	for _, n := range w.Snapshot("one").Nodes {
		if n.ID == "bakery_plot_0" && (n.Owner != "" || len(n.Paint) != 0 || len(n.Equipment) != 0 || n.BakeryLevel != 0) {
			t.Fatal("unclaimed plot still displays its departed baker's progress")
		}
		if n.PlotID == "bakery_plot_0" && (n.Owner != "" || n.Available) {
			t.Fatal("departed baker's stations remain active")
		}
	}
	w.Join("two", "New baker")
	other := w.players["two"]
	other.profile.Coins = 250
	nodePosition(w, other, "bakery_plot_0")
	if err := w.Act("two", Action{Action: "buy_land", Target: "bakery_plot_0"}); err != nil {
		t.Fatal(err)
	}
	w.Leave("one")
	w.Join("one", "Returning baker")
	p = w.players["one"]
	nodePosition(w, p, "bakery_plot_0")
	if err := w.Act("one", Action{Action: "buy_land", Target: "bakery_plot_0"}); err == nil {
		t.Fatal("returning baker displaced the new owner of the old plot")
	}
	nodePosition(w, p, "bakery_plot_1")
	mustAct(t, w, Action{Action: "buy_land", Target: "bakery_plot_1"})
	if !p.profile.Bakery || p.profile.Coins != saved.Coins || p.profile.BakeryLevel != saved.BakeryLevel || !reflect.DeepEqual(p.profile.Equipment, saved.Equipment) || !reflect.DeepEqual(p.profile.Paint, saved.Paint) {
		t.Fatal("free reopening charged again or lost bakery progress")
	}
	if w.plotOwner("bakery_plot_0") != "two" || w.plotOwner("bakery_plot_1") != "one" {
		t.Fatal("old disconnect or free reopening created duplicate ownership")
	}
	for _, n := range w.Snapshot("one").Nodes {
		if n.PlotID == "bakery_plot_1" && (n.Owner != "one" || !n.Available || n.BakeryLevel != saved.BakeryLevel || n.Paint["walls"] != "teal") {
			t.Fatal("reopened stations did not load the saved bakery")
		}
	}
}

func TestRestartFreesAllBakeryPlotsWithoutDeletingPurchases(t *testing.T) {
	saved := map[string]Profile{}
	for i := 0; i < 16; i++ {
		id := fmt.Sprintf("baker-%d", i)
		saved[id] = Profile{ID: id, Name: id, Bakery: true, BakeryPlot: "bakery_plot_0", BakeryLevel: i + 1, Equipment: map[string]bool{"oven": true}, Paint: map[string]string{"walls": "blue"}}
	}
	w := New(saved)
	for _, plot := range w.layout.Plots {
		if w.plotOwner(plot.ID) != "" {
			t.Fatal("restart reserved a city plot for an offline account")
		}
	}
	for i := 0; i < 16; i++ {
		id := fmt.Sprintf("baker-%d", i)
		w.Join(id, id)
		p := w.players[id]
		if !p.profile.Bakery || p.profile.BakeryPlot != "" || p.profile.BakeryLevel != i+1 || !p.profile.Equipment["oven"] || p.profile.Paint["walls"] != "blue" {
			t.Fatal("restart assigned a stale plot or lost saved bakery progress")
		}
	}
	// Even a saved baker with no coins can reopen at a different free plot.
	p := w.players["baker-15"]
	nodePosition(w, p, "bakery_plot_8")
	if err := w.Act("baker-15", Action{Action: "buy_land", Target: "bakery_plot_8"}); err != nil || p.profile.Coins != 0 {
		t.Fatalf("free reopening after restart: %v", err)
	}
}

func TestBakeryStationsHaveDistinctLocationsAndEnforceOwnershipAndRange(t *testing.T) {
	w, p := testWorld()
	p.profile.Coins = 250
	nodePosition(w, p, "bakery_plot_0")
	mustAct(t, w, Action{Action: "buy_land", Target: "bakery_plot_0"})
	positions := map[string][2]float64{}
	for _, n := range w.Snapshot("one").Nodes {
		if n.PlotID == "bakery_plot_0" {
			if !n.Available || n.Owner != "one" {
				t.Fatal("claimed base station is unavailable")
			}
			positions[n.Kind] = [2]float64{n.X, n.Z}
		}
	}
	if len(positions) != 3 || positions["bakery_oven"] == positions["bakery_mixer"] || positions["bakery_mixer"] == positions["bakery_display"] {
		t.Fatal("oven, mixer, and display must expose distinct interaction points")
	}
	setItem(p, "dough", 20)
	for _, tc := range []struct {
		name, standing string
		action         Action
	}{
		{"no mixing at oven", "oven", Action{Action: "make_dough", Target: "bakery_plot_0_mixer"}},
		{"no selling at oven", "oven", Action{Action: "trade", Target: "bakery_plot_0_display", Item: "sugar"}},
		{"no baking at mixer", "mixer", Action{Action: "craft", Target: "bakery_plot_0_oven", Item: "sugar"}},
		{"no selling at mixer", "mixer", Action{Action: "trade", Target: "bakery_plot_0_display", Item: "sugar"}},
		{"no baking at display", "display", Action{Action: "craft", Item: "sugar"}},
		{"no mixing at display", "display", Action{Action: "make_dough"}},
		{"no selling through plot sign", "display", Action{Action: "trade", Target: "bakery_plot_0", Item: "sugar"}},
		{"no building through oven", "oven", Action{Action: "craft", Target: "bakery_plot_0_oven", Item: "fire"}},
		{"no bakery ingredient delivery", "mixer", Action{Action: "order_dough"}},
		{"no target spoofing", "oven", Action{Action: "craft", Target: "bakery_plot_1_oven", Item: "sugar"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodePosition(w, p, "bakery_plot_0_"+tc.standing)
			before := cloneProfile(p.profile)
			if err := w.Act("one", tc.action); err == nil {
				t.Fatal("station restriction was bypassed")
			}
			if !reflect.DeepEqual(before, p.profile) {
				t.Fatal("rejected action changed the player's saved items or money")
			}
		})
	}
	nodePosition(w, p, "bakery_plot_0_oven")
	mustAct(t, w, Action{Action: "craft", Target: "bakery_plot_0_oven", Item: "sugar"})
	nodePosition(w, p, "bakery_plot_0_mixer")
	mustAct(t, w, Action{Action: "make_dough", Target: "bakery_plot_0_mixer"})
	nodePosition(w, p, "bakery_plot_0_display")
	mustAct(t, w, Action{Action: "trade", Target: "bakery_plot_0_display", Item: "sugar"})
	w.Join("two", "Visitor")
	other := w.players["two"]
	setItem(other, "dough", 20)
	for _, station := range []string{"oven", "mixer", "display"} {
		nodePosition(w, other, "bakery_plot_0_"+station)
		if err := w.requireBakeryStation(other, station, "bakery_plot_0_"+station); err == nil {
			t.Fatal("visitor used another player's bakery station")
		}
	}
	p.island = "one"
	nodePosition(w, p, "bakery_plot_0_mixer")
	if err := w.Act("one", Action{Action: "make_dough"}); err == nil {
		t.Fatal("home-island coordinates reached a mainland bakery")
	}
}

func TestClaimingBakeryDoesNotTrapPlayersInNewDisplay(t *testing.T) {
	for _, returning := range []bool{false, true} {
		t.Run(fmt.Sprintf("returning=%v", returning), func(t *testing.T) {
			w, p := testWorld()
			plot := w.layout.Plots[0]
			p.profile.Bakery, p.profile.Coins = returning, 250
			p.x, p.z = plot.X+2.4, plot.Z+2
			if w.solidBlocked(p.x, p.z, .42) {
				t.Fatal("future display footprint should be empty before claiming")
			}
			w.Join("visitor", "Visitor")
			visitor := w.players["visitor"]
			visitor.x, visitor.z = p.x+.3, p.z
			w.Join("observer", "Observer")
			observer := w.players["observer"]
			observer.x, observer.z = plot.X-1, plot.Z+1
			w.Input("one", Input{X: 1, Sprint: true, Jump: true})
			w.Input("visitor", Input{X: -1, Jump: true})
			w.Input("observer", Input{Z: 1})
			mustAct(t, w, Action{Action: "buy_land", Target: plot.ID})
			for _, moved := range []*player{p, visitor} {
				if w.solidBlocked(moved.x, moved.z, .42) {
					t.Fatalf("claim trapped %s in the newly installed display", moved.profile.ID)
				}
				if moved.x != plot.X || moved.z != plot.Z || moved.input != (Input{}) || moved.jumpQueued {
					t.Fatal("intersecting player was not safely moved to the clear interior with pending movement cleared")
				}
				startZ := moved.z
				w.Input(moved.profile.ID, Input{Z: -1})
				w.Tick(.1)
				if moved.z >= startZ-.5 {
					t.Fatal("relocated player remains unable to walk")
				}
			}
			if observer.x != plot.X-1 || observer.input.Z != 1 {
				t.Fatal("claim changed an uninvolved player's position or input")
			}
		})
	}
}

func TestClaimingBakeryPreservesClearClaimantPositionAndMovement(t *testing.T) {
	w, p := testWorld()
	plot := w.layout.Plots[0]
	p.profile.Bakery = true
	p.x, p.z = plot.X-1, plot.Z+1
	w.Input("one", Input{X: 1, Jump: true})
	mustAct(t, w, Action{Action: "buy_land", Target: plot.ID})
	if p.x != plot.X-1 || p.z != plot.Z+1 || p.input.X != 1 || !p.input.Jump || !p.jumpQueued {
		t.Fatal("claim unnecessarily moved or reset a player outside the display footprint")
	}
}
