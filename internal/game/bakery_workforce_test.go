package game

import (
	"reflect"
	"strings"
	"testing"
)

func workforceWorld(t *testing.T) (*World, *player, string) {
	t.Helper()
	w, p := businessWorld(t)
	p.profile.Coins = 5000
	kept := p.profile.BakeryFurniture[:0]
	for _, f := range p.profile.BakeryFurniture {
		if f.Kind != "chair" && f.Kind != "table" {
			kept = append(kept, f)
		}
	}
	p.profile.BakeryFurniture = kept
	w.rebuildPropertyColliders()
	plot, _ := w.bakeryPlot(p)
	p.x, p.z = plot.X, plot.Z
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "bakery_place", Item: "display", X: -5, Z: 3})
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: false})
	return w, p, bakeryDisplays(&p.profile)[1].ID
}

func TestBakeryTwoDisplaysHaveIndependentNodesAndSharedCapacity(t *testing.T) {
	w, p, second := workforceWorld(t)
	plot, _ := w.bakeryPlot(p)
	p.profile.BakeryDisplay = map[string]int{"sugar": 499, "berry_cookie": 100}
	setItem(p, "sugar", 2)
	p.x, p.z = plot.X-5, plot.Z+3
	nodeID := plot.ID + "_station_" + second
	mustAct(t, w, Action{Action: "bakery_stock", Target: nodeID, Item: "sugar"})
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "bakery_stock", Target: nodeID, Item: "sugar"}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("two-display capacity overflow was not atomic")
	}
	s := w.Snapshot("one")
	stock := map[string]int{}
	nodes := map[string]bool{}
	for _, n := range s.Nodes {
		if n.Kind != "bakery_display" || !n.Available || n.Owner != "one" {
			continue
		}
		if nodes[n.ID] || n.FurnitureID == "" || n.Amount > 300 {
			t.Fatal("display node identity or partition capacity is invalid")
		}
		nodes[n.ID] = true
		for item, count := range n.Stock {
			stock[item] += count
		}
		n.Stock["sugar"] = 999
	}
	if len(nodes) != 2 || !nodes[nodeID] || !reflect.DeepEqual(stock, p.profile.BakeryDisplay) || s.Me.BakeryBusiness.DisplayCapacity != 600 {
		t.Fatal("display nodes duplicate or omit stock")
	}
	p.x, p.z = plot.X, plot.Z
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	before = cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "bakery_place", Item: "display", X: 0, Z: 3}); err == nil || !strings.Contains(err.Error(), "two displays") || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("third display was not rejected without payment")
	}
	normalizeBakeryBusiness(&p.profile)
	normalizeBakeryFurniture(&p.profile)
	if len(bakeryDisplays(&p.profile)) != 2 || bakeryStockCount(p.profile.BakeryDisplay) != 600 {
		t.Fatal("normalization lost the second display or its goods")
	}
}

func TestBakeryWorkforceLimitBreaksAndSnapshotOwnership(t *testing.T) {
	w, p, _ := workforceWorld(t)
	for _, role := range []string{"chef", "server", "cashier"} {
		for i := 0; i < 2; i++ {
			mustAct(t, w, Action{Action: "bakery_hire", Item: role})
		}
		before := cloneProfile(p.profile)
		if err := w.Act("one", Action{Action: "bakery_hire", Item: role}); err == nil || !reflect.DeepEqual(before, p.profile) {
			t.Fatal("third worker was charged or hired")
		}
	}
	for _, id := range []string{"chef_2", "server_2", "cashier_2"} {
		mustAct(t, w, Action{Action: "bakery_staff_break", Target: p.profile.BakeryPlot, Item: id, Enabled: true})
	}
	w.Tick(.1)
	s := w.Snapshot("one")
	if s.Me.BakeryBusiness.StaffLimit != 2 || len(s.Me.BakeryBusiness.Workers) != 6 {
		t.Fatal("business omitted an individually hired worker")
	}
	actors := map[string]bool{}
	for _, n := range s.Nodes {
		if !validChoice(n.Kind, "bakery_chef", "bakery_server", "bakery_cashier") || n.Owner != "one" {
			continue
		}
		if actors[n.ID] {
			t.Fatal("staff shared a node identity")
		}
		actors[n.ID] = true
		if strings.HasSuffix(n.ID, "_2") && n.Phase != "On break" {
			t.Fatal("paused worker did not display their own break")
		}
	}
	if len(actors) != 6 {
		t.Fatal("six hired workers did not appear")
	}
	s.Me.BakeryStaffBreaks["server_2"] = false
	if !p.profile.BakeryStaffBreaks["server_2"] {
		t.Fatal("snapshot break map aliases profile")
	}
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "bakery_staff_break", Target: "bakery_plot_1", Item: "server", Enabled: true}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("other bakery sign changed worker break")
	}
}

func TestBakeryDisplayRemovalNeverOverflowsRemainingCounters(t *testing.T) {
	w, p, second := workforceWorld(t)
	plot, _ := w.bakeryPlot(p)
	p.profile.BakeryDisplay["sugar"] = 450
	p.x, p.z = plot.X-5, plot.Z+5
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "bakery_edit", Target: second})
	for _, action := range []string{"bakery_pack", "bakery_destroy"} {
		before := cloneProfile(p.profile)
		if err := w.Act("one", Action{Action: action, Target: second}); err == nil || !reflect.DeepEqual(before, p.profile) {
			t.Fatal("removing display overflowed remaining shelf or changed saved goods/payment")
		}
	}
	p.profile.BakeryDisplay["sugar"] = 300
	mustAct(t, w, Action{Action: "bakery_pack", Target: second})
	if len(bakeryDisplays(&p.profile)) != 1 || p.profile.BakeryDisplay["sugar"] != 300 {
		t.Fatal("safe removal did not preserve stock on remaining display")
	}
	p.x, p.z = plot.X+5, plot.Z+5
	mustAct(t, w, Action{Action: "bakery_edit", Target: "display"})
	for _, action := range []string{"bakery_pack", "bakery_destroy"} {
		before := cloneProfile(p.profile)
		if err := w.Act("one", Action{Action: action, Target: "display"}); err == nil || !reflect.DeepEqual(before, p.profile) {
			t.Fatal("removed last stocked display")
		}
	}
	// Old saves may already contain packed furniture. Keep the shared pool,
	// and never render its overflow as more than 300 cookies on one counter.
	p.profile.BakeryDisplay["sugar"] = 450
	normalizeBakeryBusiness(&p.profile)
	if p.profile.BakeryDisplay["sugar"] != 450 || bakeryStockCount(bakeryDisplayStock(&p.profile, "display")) != 300 {
		t.Fatal("legacy packed stock was lost or overflowed the physical shelf")
	}
}

func TestTwoBakeryServersPauseWithIndependentPersistedTrays(t *testing.T) {
	w, p, _ := workforceWorld(t)
	for i := 0; i < 2; i++ {
		mustAct(t, w, Action{Action: "bakery_hire", Item: "server"})
	}
	productionState(&p.profile, "bakery_cooling_table").Output = map[string]int{"sugar": 70}
	paused := map[string]Point{}
	for i := 0; i < 500 && len(paused) < 2; i++ {
		w.Tick(.05)
		assertBakeryActorsClear(t, w, p)
		for _, id := range []string{"server", "server_2"} {
			if bakeryStockCount(bakeryServerTray(&p.profile, id)) == 20 && !p.profile.BakeryStaffBreaks[id] {
				worker := p.bakery.workers[id]
				paused[id] = Point{worker.x, worker.z}
				mustAct(t, w, Action{Action: "bakery_staff_break", Target: p.profile.BakeryPlot, Item: id, Enabled: true})
			}
		}
	}
	if len(paused) != 2 || p.profile.BakeryCarry["sugar"] != 20 || p.profile.BakeryServerCarry["server_2"]["sugar"] != 20 {
		t.Fatal("servers failed to collect independent real trays")
	}
	advance(w, 3)
	for id, point := range paused {
		worker := p.bakery.workers[id]
		if worker.x != point.X || worker.z != point.Z || bakeryServerTray(&p.profile, id)["sugar"] != 20 {
			t.Fatal("break moved worker or lost carried goods")
		}
	}
	s := w.Snapshot("one")
	s.Me.BakeryServerCarry["server_2"]["sugar"] = 999
	if p.profile.BakeryServerCarry["server_2"]["sugar"] != 20 {
		t.Fatal("second server snapshot aliases saved tray")
	}
	w.Leave("one")
	w = New(w.Profiles())
	w.Join("one", "Baker")
	p = w.players["one"]
	if !p.profile.BakeryStaffBreaks["server"] || !p.profile.BakeryStaffBreaks["server_2"] || p.profile.BakeryCarry["sugar"] != 20 || p.profile.BakeryServerCarry["server_2"]["sugar"] != 20 {
		t.Fatal("restart lost independent tray/break states")
	}
	nodePosition(w, p, "bakery_plot_1")
	mustAct(t, w, Action{Action: "buy_land", Target: "bakery_plot_1"})
	for _, id := range []string{"server", "server_2"} {
		mustAct(t, w, Action{Action: "bakery_staff_break", Target: p.profile.BakeryPlot, Item: id, Enabled: false})
	}
	advance(w, 35)
	if p.profile.BakeryDisplay["sugar"] != 70 || bakeryStockCount(p.profile.BakeryCarry) != 0 || bakeryStockCount(p.profile.BakeryServerCarry["server_2"]) != 0 || bakeryStockCount(productionState(&p.profile, "bakery_cooling_table").Output) != 0 {
		t.Fatal("resumed servers duplicated or lost finished bakes")
	}
}

func TestTwoBakeryCashiersServeDistinctPhysicalQueues(t *testing.T) {
	w, p, second := workforceWorld(t)
	for i := 0; i < 2; i++ {
		mustAct(t, w, Action{Action: "bakery_hire", Item: "cashier"})
	}
	p.profile.BakeryDisplay["sugar"] = 40
	for _, id := range []string{"cashier", "cashier_2"} {
		mustAct(t, w, Action{Action: "bakery_staff_break", Target: p.profile.BakeryPlot, Item: id, Enabled: true})
	}
	w.admitBakeryCustomer(p)
	w.admitBakeryCustomer(p)
	p.bakery.nextCustomer = w.time + 1000
	advance(w, 12)
	if len(p.bakery.customers) != 2 || p.bakery.customers[0].Phase != "waiting" || p.bakery.customers[1].Phase != "waiting" || p.bakery.customers[0].DisplayID == p.bakery.customers[1].DisplayID {
		t.Fatal("customers did not walk to both distinct display queues")
	}
	for _, id := range []string{"cashier", "cashier_2"} {
		mustAct(t, w, Action{Action: "bakery_staff_break", Target: p.profile.BakeryPlot, Item: id, Enabled: false})
	}
	before := p.profile.Coins
	advance(w, 12)
	if p.profile.Coins != before+20 || p.profile.BakeryDisplay["sugar"] != 30 {
		t.Fatalf("cashiers did not each sell one exact order: coins %d -> %d, stock %v, first %+v, second %+v", before, p.profile.Coins, p.profile.BakeryDisplay, p.bakery.workers["cashier"], p.bakery.workers["cashier_2"])
	}
	if p.bakery.workers["cashier"].displayID != "display" || p.bakery.workers["cashier_2"].displayID != second {
		t.Fatal("cashiers were not assigned their physical counters")
	}
}
