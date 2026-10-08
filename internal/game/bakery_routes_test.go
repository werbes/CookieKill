package game

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestBakerySettledWorkersDoNotSearchRoutesEveryTick(t *testing.T) {
	w, p := businessWorld(t)
	for _, role := range []string{"chef", "cashier"} {
		mustAct(t, w, Action{Action: "bakery_hire", Item: role})
	}
	advance(w, 10)
	if !w.bakeryWorkerReady(p, "chef") || !w.bakeryWorkerReady(p, "cashier") {
		t.Fatal("staff route fixture did not settle")
	}
	if allocations := testing.AllocsPerRun(100, func() { w.updateBakeryWorkers(p, .05) }); allocations != 0 {
		t.Fatalf("steady workers should reuse their route and goal without searches or allocations: %v", allocations)
	}
}

func BenchmarkBakeryFurnitureReplan(b *testing.B) {
	w, p := testWorld()
	p.profile.Bakery, p.profile.BakeryPlot = true, w.layout.Plots[0].ID
	p.profile.BakeryFurniture = defaultBakeryFurniture()
	p.profile.BakeryFurniture = append(p.profile.BakeryFurniture, BakeryFurniture{ID: "detour", Kind: "counter", X: 2.6, Z: 5.8})
	for len(p.profile.BakeryFurniture) < 120 {
		p.profile.BakeryFurniture = append(p.profile.BakeryFurniture, BakeryFurniture{ID: fmt.Sprint(len(p.profile.BakeryFurniture)), Kind: "rug", X: -7, Z: -3})
	}
	w.startBakeryBusiness(p)
	plot, _ := w.bakeryPlot(p)
	for slot := 0; slot < 3; slot++ {
		p.bakery.customers = append(p.bakery.customers, &bakeryCustomer{BakeryCustomerView: BakeryCustomerView{Phase: "arriving"}, slot: slot, x: plot.X, z: plot.Z + 9.5})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, c := range p.bakery.customers {
			w.routeBakeryCustomer(p, c)
		}
	}
}

func assertBakeryActorsClear(t *testing.T, w *World, p *player) {
	t.Helper()
	for _, point := range w.bakeryNPCPositions(p) {
		if w.solidBlockedAt(point.X, 0, point.Z, .34) {
			t.Fatalf("bakery NPC clipped authoritative furniture or shell at %+v", point)
		}
	}
}

func TestBakeryStaffAndCustomersWalkAroundFurnitureThroughRealDoorways(t *testing.T) {
	w, p := businessWorld(t)
	for _, role := range []string{"chef", "cashier"} {
		mustAct(t, w, Action{Action: "bakery_hire", Item: role})
	}
	p.profile.BakeryDisplay["sugar"] = 20
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "bakery_place", Item: "counter", X: 2.6, Z: 5.8})
	p.x, p.z = -35, -35
	waited := false
	for i := 0; i < 700; i++ {
		w.Tick(.05)
		assertBakeryActorsClear(t, w, p)
		for _, c := range p.bakery.customers {
			waited = waited || c.Phase == "waiting"
		}
	}
	for _, role := range []string{"chef", "cashier"} {
		if !w.bakeryWorkerReady(p, role) {
			t.Fatalf("%s did not reach their station: %+v", role, p.bakery.workers[role])
		}
	}
	if !waited || p.profile.Coins <= 205 {
		t.Fatal("customer did not reach the display and complete a real sale around the added counter")
	}
}

func TestMovingDisplayRepathsPendingOrderWithoutRemoteSale(t *testing.T) {
	w, p := businessWorld(t)
	p.profile.BakeryDisplay["sugar"] = 10
	c := firstWaitingCustomer(t, w, p)
	order, amount, expires := c.Item, c.Amount, c.expires
	plot, _ := w.bakeryPlot(p)
	p.x, p.z = plot.X, plot.Z
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "bakery_edit", Target: "display"})
	mustAct(t, w, Action{Action: "bakery_move", Target: "display", X: 6, Z: 4, Rotation: 90})
	before := p.profile.Coins
	if err := w.serveBakeryCustomer(p, c); err == nil || p.profile.Coins != before {
		t.Fatal("customer was served at the abandoned counter position")
	}
	w.Tick(.05)
	if c.Phase != "arriving" || c.Item != order || c.Amount != amount || c.expires != expires {
		t.Fatal("moving the display lost or renewed the pending order")
	}
	for i := 0; i < 300 && c.Phase != "waiting"; i++ {
		w.Tick(.05)
		assertBakeryActorsClear(t, w, p)
	}
	want, _ := w.bakeryStationServicePoint(p, "display", false)
	if c.Phase != "waiting" || distance(c.x, c.z, want.X, want.Z) > .01 {
		t.Fatalf("customer did not find moved, rotated display: %+v want %+v", c, want)
	}
	if math.Abs(math.Sin(c.yaw)-1) > .001 {
		t.Fatal("customer does not face the rotated counter")
	}
	if err := w.serveBakeryCustomer(p, c); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 250 && len(p.bakery.customers) > 0; i++ {
		w.Tick(.05)
		assertBakeryActorsClear(t, w, p)
	}
}

func TestPackedStationsPauseStaffAndCustomersUntilReplaced(t *testing.T) {
	w, p := businessWorld(t)
	mustAct(t, w, Action{Action: "bakery_hire", Item: "chef"})
	p.profile.BakeryDisplay["sugar"] = 10
	c := firstWaitingCustomer(t, w, p)
	plot, _ := w.bakeryPlot(p)
	p.x, p.z = plot.X, plot.Z
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "bakery_edit", Target: "display"})
	mustAct(t, w, Action{Action: "bakery_pack", Target: "display"})
	stock, coins := bakeryStockCount(p.profile.BakeryDisplay), p.profile.Coins
	w.Tick(.05)
	if c.Phase != "leaving" || c.Amount != 0 || !w.bakeryWorkerReady(p, "chef") {
		t.Fatal("packed display failed to send customers out while retaining the independent kitchen chef")
	}
	advance(w, 45)
	if len(p.bakery.customers) != 0 || bakeryStockCount(p.profile.BakeryDisplay) != stock || p.profile.Coins != coins {
		t.Fatal("packed station kept attracting customers or processing stock")
	}
	mustAct(t, w, Action{Action: "bakery_place", Item: "display", From: "bakery_display", X: 5, Z: 3})
	advance(w, 30)
	if !w.bakeryWorkerReady(p, "chef") || len(p.bakery.customers) == 0 || strings.Contains(p.bakery.chefStatus, "Place") {
		t.Fatal("replacing the display did not resume staff and customers")
	}
}

func TestMovedOvenMakesChefWalkThroughKitchenWithoutClipping(t *testing.T) {
	w, p := businessWorld(t)
	mustAct(t, w, Action{Action: "bakery_hire", Item: "chef"})
	advance(w, 10)
	if !w.bakeryWorkerReady(p, "chef") {
		t.Fatal("chef failed to reach initial oven")
	}
	plot, _ := w.bakeryPlot(p)
	p.x, p.z = plot.X, plot.Z-3
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "bakery_edit", Target: "oven"})
	mustAct(t, w, Action{Action: "bakery_move", Target: "oven", X: -7, Z: -5.5, Rotation: 90})
	start := Point{p.bakery.workers["chef"].x, p.bakery.workers["chef"].z}
	w.Tick(.05)
	worker := p.bakery.workers["chef"]
	if w.bakeryWorkerReady(p, "chef") || distance(start.X, start.Z, worker.x, worker.z) > .121 {
		t.Fatal("chef teleported to the moved oven")
	}
	for i := 0; i < 240 && !w.bakeryWorkerReady(p, "chef"); i++ {
		w.Tick(.05)
		assertBakeryActorsClear(t, w, p)
	}
	want, _ := w.bakeryStationServicePoint(p, "oven", false)
	if !w.bakeryWorkerReady(p, "chef") || distance(worker.x, worker.z, want.X, want.Z) > .01 {
		t.Fatal("chef never reached moved oven")
	}
}
