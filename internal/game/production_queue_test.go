package game

import (
	"math"
	"reflect"
	"testing"
)

func addQueueTestStation(t *testing.T, w *World, p *player, kind string, x, z float64) BakeryFurniture {
	t.Helper()
	plot, _ := w.bakeryPlot(p)
	p.x, p.z = plot.X, plot.Z
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "bakery_place", Item: kind, X: x, Z: z})
	f := p.profile.BakeryFurniture[len(p.profile.BakeryFurniture)-1]
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: false})
	return f
}
func queueTestStationPosition(t *testing.T, w *World, p *player, f BakeryFurniture) string {
	t.Helper()
	point, ok := w.bakeryFurnitureServicePoint(p, f, false)
	if !ok {
		t.Fatal("station fixture has no plot")
	}
	p.x, p.z = point.X, point.Z
	return bakeryFurnitureNodeID(p.profile.BakeryPlot, f)
}

func TestProductionQueuePaysUpfrontAndRunsInFIFOOrder(t *testing.T) {
	w, p := businessWorld(t)
	baseBakingIngredients(p, 1)
	setItem(p, "dough", 3)
	setItem(p, "berry", 2)
	setItem(p, "nut", 2)
	holdTestItem(t, p, "dough")
	nodePosition(w, p, "bakery_plot_0_mixer")
	mustAct(t, w, Action{Action: "production_mix", Item: "cake"})
	mustAct(t, w, Action{Action: "production_mix", Item: "dough"})
	s := p.profile.Production["bakery_mixer"]
	if s.Job.Item != "prepared_cake" || s.Job.Remaining != 3 || len(s.Queue) != 1 || s.Queue[0].Item != "dough" || s.Queue[0].Remaining != 40 || p.profile.Inventory["dough"] != 0 || p.profile.Inventory["flour"] != 0 {
		t.Fatal("FIFO jobs did not reserve each batch's actual ingredients upfront")
	}
	snapshot := w.Snapshot("one")
	snapshot.Me.Production["bakery_mixer"].Queue[0].Amount = 999
	for _, n := range snapshot.Nodes {
		if n.ID == "bakery_plot_0_mixer" {
			n.Production.Queue[0].Remaining = 0
		}
	}
	if s.Queue[0].Amount != 40 || s.Queue[0].Remaining != 40 {
		t.Fatal("queue snapshots alias live state")
	}
	advance(w, 3)
	if s.Output["prepared_cake"] != 1 || s.Output["dough"] != 0 || s.Job.Item != "dough" || math.Abs(s.Job.Remaining-40) > .00001 || len(s.Queue) != 0 {
		t.Fatal("mixer skipped FIFO order or changed a queued timer")
	}
	mustAct(t, w, Action{Action: "production_collect", Target: "bakery_plot_0_mixer", Item: "prepared_cake"})
	advance(w, 40)
	if s.Output["dough"] != 40 || s.Job != nil {
		t.Fatal("queued dough did not finish at its original paid yield")
	}
}

func TestProductionQueueLimitRejectsWithoutTakingIngredients(t *testing.T) {
	w, p := businessWorld(t)
	baseBakingIngredients(p, productionQueueLimit+2)
	nodePosition(w, p, "bakery_plot_0_mixer")
	for i := 0; i < productionQueueLimit+1; i++ {
		mustAct(t, w, Action{Action: "production_mix", Item: "dough"})
	}
	s := p.profile.Production["bakery_mixer"]
	if len(s.Queue) != productionQueueLimit || s.Job.Remaining != 40 || p.profile.Inventory["egg"] != 1 {
		t.Fatal("queue count or upfront payment is incorrect")
	}
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "production_mix", Item: "dough"}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("full queue charged ingredients or lost existing jobs")
	}
	if w.Snapshot("one").Me.Production["bakery_mixer"].QueueLimit != productionQueueLimit {
		t.Fatal("queue capacity is absent from the protocol")
	}
}

func TestOvenQueueRetainsCompletedBatchWhenCoolingTableIsFull(t *testing.T) {
	w, p := businessWorld(t)
	productionState(&p.profile, "bakery_cooling_table").Output["sugar"] = productionCapacity
	setItem(p, "prepared_berry_cookie", 2)
	holdTestItem(t, p, "prepared_berry_cookie")
	nodePosition(w, p, "bakery_plot_0_oven")
	for i := 0; i < 2; i++ {
		mustAct(t, w, Action{Action: "production_bake", Item: "prepared_berry_cookie"})
	}
	advance(w, 15)
	s := p.profile.Production["bakery_oven"]
	if s.Job == nil || s.Job.Remaining != 0 || len(s.Queue) != 1 || s.Queue[0].Remaining != 5 || p.profile.Inventory["prepared_berry_cookie"] != 0 {
		t.Fatal("full cooling table lost or advanced paid queued work")
	}
	nodePosition(w, p, "bakery_plot_0_cooling_table")
	mustAct(t, w, Action{Action: "production_collect", Target: "bakery_plot_0_cooling_table", Item: "sugar"})
	w.Tick(.05)
	if p.profile.Production["bakery_cooling_table"].Output["berry_cookie"] != 1 || s.Job == nil || s.Job.Remaining < 4.9 || len(s.Queue) != 0 {
		t.Fatal("freeing one output slot did not unblock exactly the first batch")
	}
	advance(w, 5)
	if s.Job == nil || s.Job.Remaining != 0 {
		t.Fatal("second batch did not wait for its own output slot")
	}
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "bakery_edit", Target: "cooling_table"})
	if err := w.Act("one", Action{Action: "bakery_pack", Target: "cooling_table"}); err == nil {
		t.Fatal("table with queued output could be removed")
	}
	s = p.profile.Production["bakery_oven"]
	mustAct(t, w, Action{Action: "bakery_cancel_edit"})
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: false})
	mustAct(t, w, Action{Action: "production_collect", Target: "bakery_plot_0_cooling_table", Item: "sugar"})
	w.Tick(.05)
	if s.Job != nil || p.profile.Production["bakery_cooling_table"].Output["berry_cookie"] != 2 {
		t.Fatal("paused oven queue lost its second output")
	}
}

func TestMultipleAppliancesHaveIndependentPersistentQueues(t *testing.T) {
	w, p := businessWorld(t)
	oven := addQueueTestStation(t, w, p, "oven", -7.5, -5.5)
	mixer := addQueueTestStation(t, w, p, "mixer", 2.5, -5.5)
	if p.profile.Coins != 540 || bakeryProductionKey(oven) == "bakery_oven" || bakeryProductionKey(mixer) == "bakery_mixer" {
		t.Fatal("additional appliances reused the first station or skipped their catalog price")
	}
	counts := map[string]int{}
	for _, n := range w.Snapshot("one").Nodes {
		if n.Owner == "one" && n.Available && validChoice(n.Kind, "bakery_oven", "bakery_mixer") {
			counts[n.Kind]++
			if n.FurnitureID == "" || n.ProductionKey == "" || n.Label == "" {
				t.Fatal("station node omitted its identity or label")
			}
		}
	}
	if counts["bakery_oven"] != 2 || counts["bakery_mixer"] != 2 {
		t.Fatal("additional appliances were absent from the public snapshot")
	}
	baseBakingIngredients(p, 2)
	nodePosition(w, p, "bakery_plot_0_mixer")
	mustAct(t, w, Action{Action: "production_mix", Target: "bakery_plot_0_mixer", Item: "dough"})
	target := queueTestStationPosition(t, w, p, mixer)
	mustAct(t, w, Action{Action: "production_mix", Target: target, Item: "dough"})
	advance(w, 40)
	if p.profile.Production["bakery_mixer"].Output["dough"] != 40 || p.profile.Production[bakeryProductionKey(mixer)].Output["dough"] != 40 {
		t.Fatal("separate mixers did not work simultaneously")
	}
	mustAct(t, w, Action{Action: "production_collect", Target: target, Item: "dough", Amount: 40})
	holdTestItem(t, p, "dough")
	nodePosition(w, p, "bakery_plot_0_oven")
	mustAct(t, w, Action{Action: "production_bake", Target: "bakery_plot_0_oven", Item: "dough"})
	mustAct(t, w, Action{Action: "production_bake", Target: "bakery_plot_0_oven", Item: "dough"})
	target = queueTestStationPosition(t, w, p, oven)
	mustAct(t, w, Action{Action: "production_bake", Target: target, Item: "dough"})
	advance(w, 2)
	if math.Abs(p.profile.Production["bakery_oven"].Job.Remaining-3) > .00001 || math.Abs(p.profile.Production[bakeryProductionKey(oven)].Job.Remaining-3) > .00001 {
		t.Fatal("ovens did not advance in parallel")
	}
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	q := restored.players["one"]
	nodePosition(restored, q, "bakery_plot_1")
	mustAct(t, restored, Action{Action: "buy_land", Target: "bakery_plot_1"})
	if len(q.profile.Production["bakery_oven"].Queue) != 1 || q.profile.Production[bakeryProductionKey(oven)].Job == nil {
		t.Fatal("restart lost an appliance's paid queue or changed its key")
	}
	advance(restored, 8)
	if q.profile.Production["bakery_cooling_table"].Output["sugar"] != 3 {
		t.Fatal("restored appliances did not finish their independent queues into the shared table")
	}
}

func TestHomeProductionQueuePausesImmediatelyOfflineAndSurvivesRestart(t *testing.T) {
	w, p := testWorld()
	p.profile.HomeIsland = &Island{Owner: "one", Builders: map[string]bool{}, Chest: map[string]int{}, Objects: []BuildObject{{ID: "mix", Kind: "mixer", X: 0, Z: 0}, {ID: "oven", Kind: "oven", X: -4, Z: 0}, {ID: "table", Kind: "table", X: -4, Z: 3}}}
	p.island, p.x, p.z = "one", -2, 0
	setItem(p, "dough", 3)
	holdTestItem(t, p, "dough")
	for i := 0; i < 3; i++ {
		mustAct(t, w, Action{Action: "production_bake", Target: "oven", Item: "dough"})
	}
	advance(w, 2)
	before := cloneProduction(p.profile.Production)
	w.SetConnected("one", false)
	advance(w, 10)
	if !reflect.DeepEqual(before, p.profile.Production) {
		t.Fatal("disconnected home production advanced during reconnect grace")
	}
	w.SetConnected("one", true)
	advance(w, 1)
	state := p.profile.Production["home:oven"]
	if math.Abs(state.Job.Remaining-2) > .00001 || len(state.Queue) != 2 || p.profile.Inventory["dough"] != 0 {
		t.Fatal("reconnection failed to resume the same paid queue")
	}
	w.Leave("one")
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	q := restored.players["one"]
	state = q.profile.Production["home:oven"]
	if state == nil || state.Job == nil || len(state.Queue) != 2 || state.Job.Destination != "home:table" {
		t.Fatal("restart lost home station identity, queue, or linked table")
	}
	advance(restored, 12)
	if state.Job != nil || len(state.Queue) != 0 || q.profile.Production["home:table"].Output["sugar"] != 3 {
		t.Fatal("home FIFO queue did not preserve its exact paid output through restart")
	}
}

func TestAdditionalApplianceQueueCannotBePackedOrUsedByAnotherOwner(t *testing.T) {
	w, p := businessWorld(t)
	oven := addQueueTestStation(t, w, p, "oven", -7.5, -5.5)
	target := queueTestStationPosition(t, w, p, oven)
	setItem(p, "dough", 2)
	holdTestItem(t, p, "dough")
	mustAct(t, w, Action{Action: "production_bake", Target: target, Item: "dough"})
	mustAct(t, w, Action{Action: "production_bake", Target: target, Item: "dough"})
	w.Join("two", "Visitor")
	v := w.players["two"]
	v.x, v.z = p.x, p.z
	setItem(v, "dough", 1)
	holdTestItem(t, v, "dough")
	before := cloneProfile(v.profile)
	if err := w.Act("two", Action{Action: "production_bake", Target: target, Item: "dough"}); err == nil || !reflect.DeepEqual(before, v.profile) {
		t.Fatal("another owner accessed or paid into a private appliance queue")
	}
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "bakery_edit", Target: oven.ID})
	before = cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "bakery_pack", Target: oven.ID}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("packing a queued extra oven lost or refunded its paid contents")
	}
	mustAct(t, w, Action{Action: "bakery_cancel_edit"})
	nodePosition(w, p, "bakery_plot_0_oven")
	mustAct(t, w, Action{Action: "bakery_edit", Target: "oven"})
	mustAct(t, w, Action{Action: "bakery_pack", Target: "oven"})
	state := p.profile.Production[bakeryProductionKey(oven)]
	if state == nil || state.Job == nil || len(state.Queue) != 1 {
		t.Fatal("packing the idle original oven changed another appliance's queue")
	}
	advance(w, 10)
	if p.profile.Production["bakery_cooling_table"].Output["sugar"] != 2 {
		t.Fatal("the remaining oven stopped when the original was packed")
	}
}

func TestLegacyStationJobsBindToTheirExistingFurnitureDuringMigration(t *testing.T) {
	w, p := businessWorld(t)
	for i := range p.profile.BakeryFurniture {
		f := &p.profile.BakeryFurniture[i]
		if f.Kind == "oven" {
			f.ID = "old_replaced_oven"
			f.ProductionKey = ""
		}
	}
	p.profile.BakeryFurnitureVersion = 2
	s := productionState(&p.profile, "bakery_oven")
	s.Job = &ProductionJob{Item: "cake", Amount: 2, Total: 10, Remaining: 7, Destination: "bakery_cooling_table"}
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	q := restored.players["one"]
	nodePosition(restored, q, "bakery_plot_0")
	mustAct(t, restored, Action{Action: "buy_land", Target: "bakery_plot_0"})
	for _, n := range restored.Snapshot("one").Nodes {
		if n.ID == "bakery_plot_0_oven" {
			if !n.Available || n.FurnitureID != "old_replaced_oven" || n.ProductionKey != "bakery_oven" || n.Production.Job.Remaining != 7 {
				t.Fatal("legacy oven job was detached from its existing appliance")
			}
		}
	}
	advance(restored, 7)
	if q.profile.Production["bakery_cooling_table"].Output["cake"] != 2 {
		t.Fatal("legacy paid batch was lost during migration")
	}
}

func TestChefUsesAdditionalMixersAndOvens(t *testing.T) {
	w, p := businessWorld(t)
	oven := addQueueTestStation(t, w, p, "oven", -7.5, -5.5)
	mixer := addQueueTestStation(t, w, p, "mixer", 2.5, -5.5)
	plot, _ := w.bakeryPlot(p)
	p.x, p.z = plot.X, plot.Z
	mustAct(t, w, Action{Action: "bakery_hire", Item: "chef"})
	for _, item := range []string{"baking_sugar", "butter", "flour", "egg"} {
		p.profile.BakeryPantry[item] = 2
	}
	p.bakery.nextCustomer = 10000
	advance(w, 12)
	if p.profile.Production["bakery_mixer"].Job == nil || p.profile.Production[bakeryProductionKey(mixer)].Job == nil || p.profile.BakeryPantry["egg"] != 0 {
		t.Fatal("chef ignored an additional mixer or did not pay for both batches")
	}
	for i := 0; i < 600; i++ {
		first, second := p.profile.Production["bakery_oven"], p.profile.Production[bakeryProductionKey(oven)]
		if first != nil && first.Job != nil && second != nil && second.Job != nil {
			break
		}
		w.Tick(.1)
	}
	if p.profile.Production["bakery_oven"].Job == nil || p.profile.Production[bakeryProductionKey(oven)].Job == nil {
		t.Fatal("chef did not load both available ovens")
	}
	total := p.profile.BakeryPantry["dough"] + p.profile.BakeryDisplay["sugar"] + p.profile.Production["bakery_cooling_table"].Output["sugar"]
	for _, key := range []string{"bakery_mixer", bakeryProductionKey(mixer), "bakery_oven", bakeryProductionKey(oven)} {
		state := p.profile.Production[key]
		total += state.Output["dough"]
		if state.Job != nil {
			total += state.Job.Amount
		}
	}
	if total != 80 {
		t.Fatalf("parallel chef production changed its paid yield: got%d want80", total)
	}
}
