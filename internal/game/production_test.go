package game

import (
	"math"
	"reflect"
	"testing"
)

func holdTestItem(t *testing.T, p *player, item string) {
	t.Helper()
	for i, s := range p.profile.Hotbar {
		if s.Item == item && s.Count > 0 {
			p.profile.SelectedSlot = i
			syncInventory(&p.profile)
			return
		}
	}
	for i, s := range p.profile.BagSlots {
		if s.Item == item && s.Count > 0 {
			p.profile.Hotbar[0], p.profile.BagSlots[i] = s, p.profile.Hotbar[0]
			p.profile.SelectedSlot = 0
			syncInventory(&p.profile)
			return
		}
	}
	t.Fatalf("missing held fixture %s", item)
}
func baseBakingIngredients(p *player, amount int) {
	for _, item := range []string{"baking_sugar", "butter", "flour", "egg"} {
		setItem(p, item, amount)
	}
}

func TestTimedProductionRequiresCollectionHeldDoughAndCoolingTable(t *testing.T) {
	w, p := businessWorld(t)
	p.profile.BakeryLevel = 100
	p.profile.Equipment["mixer"], p.profile.Equipment["oven"] = true, true
	baseBakingIngredients(p, 1)
	nodePosition(w, p, "bakery_plot_0_mixer")
	mustAct(t, w, Action{Action: "production_mix", Target: "bakery_plot_0_mixer", Item: "dough"})
	if job := p.profile.Production["bakery_mixer"].Job; job == nil || job.Amount != 40 || job.Remaining != 40 || p.profile.Inventory["dough"] != 0 {
		t.Fatal("mixing skipped time, created inventory dough, or applied an upgrade yield bonus")
	}
	for _, item := range []string{"baking_sugar", "butter", "flour", "egg"} {
		if p.profile.Inventory[item] != 0 {
			t.Fatal("mixing failed to consume all four real ingredients")
		}
	}
	advance(w, 39)
	if err := w.Act("one", Action{Action: "production_collect", Target: "bakery_plot_0_mixer", Item: "dough", Amount: 1}); err == nil {
		t.Fatal("unfinished dough could be collected")
	}
	advance(w, 1)
	if state := p.profile.Production["bakery_mixer"]; state.Job != nil || state.Output["dough"] != 40 || p.profile.Inventory["dough"] != 0 {
		t.Fatal("completed dough did not remain in mixer hopper")
	}
	mustAct(t, w, Action{Action: "production_collect", Target: "bakery_plot_0_mixer", Item: "dough", Amount: 40})
	setItem(p, "berry", 4)
	if err := w.Act("one", Action{Action: "production_mix", Target: "bakery_plot_0_mixer", Item: "berry_cookie", Amount: 2}); err == nil {
		t.Fatal("mixer accepted dough that was not held")
	}
	holdTestItem(t, p, "dough")
	mustAct(t, w, Action{Action: "production_mix", Target: "bakery_plot_0_mixer", Item: "berry_cookie", Amount: 2})
	if p.profile.Hotbar[p.profile.SelectedSlot].Count != 38 || p.profile.Inventory["berry"] != 0 || p.profile.Production["bakery_mixer"].Job.Total != 2 {
		t.Fatal("add-ins did not consume the actual held dough or use one second per dough")
	}
	advance(w, 2)
	if p.profile.Discovered["berry_cookie"] {
		t.Fatal("mixing discovered an unbaked recipe")
	}
	mustAct(t, w, Action{Action: "production_collect", Target: "bakery_plot_0_mixer", Item: "prepared_berry_cookie", Amount: 2})
	holdTestItem(t, p, "prepared_berry_cookie")
	nodePosition(w, p, "bakery_plot_0_oven")
	mustAct(t, w, Action{Action: "production_bake", Target: "bakery_plot_0_oven", Item: "prepared_berry_cookie", Amount: 2})
	advance(w, 9)
	if p.profile.Inventory["berry_cookie"] != 0 || p.profile.Production["bakery_cooling_table"].Output["berry_cookie"] != 0 {
		t.Fatal("baking delivered early")
	}
	advance(w, 1)
	if p.profile.Inventory["berry_cookie"] != 0 || p.profile.Production["bakery_cooling_table"].Output["berry_cookie"] != 2 || !p.profile.Discovered["berry_cookie"] {
		t.Fatal("finished cookies did not appear only on the linked cooling table")
	}
	nodePosition(w, p, "bakery_plot_0_cooling_table")
	mustAct(t, w, Action{Action: "production_collect", Target: "bakery_plot_0_cooling_table", Item: "berry_cookie", Amount: 2})
	if p.profile.Inventory["berry_cookie"] != 2 || p.profile.BakeryDisplay["berry_cookie"] != 0 {
		t.Fatal("pickup automatically sold or displayed goods")
	}
}

func TestProductionPermissionsReplayCapacityAndPackingAreAtomic(t *testing.T) {
	w, p := businessWorld(t)
	baseBakingIngredients(p, 2)
	nodePosition(w, p, "bakery_plot_0_mixer")
	w.Join("two", "Visitor")
	v := w.players["two"]
	v.x, v.z = p.x, p.z
	baseBakingIngredients(v, 1)
	if err := w.Act("two", Action{Action: "production_mix", Target: "bakery_plot_0_mixer", Item: "dough"}); err == nil {
		t.Fatal("visitor operated another baker's mixer")
	}
	mustAct(t, w, Action{Action: "production_mix", Target: "bakery_plot_0_mixer", Item: "dough"})
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "production_mix", Target: "bakery_plot_0_mixer", Item: "dough"}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("replayed start charged or duplicated a job")
	}
	mustAct(t, w, Action{Action: "bakery_build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "bakery_edit", Target: "mixer"})
	if err := w.Act("one", Action{Action: "bakery_pack", Target: "mixer"}); err == nil {
		t.Fatal("active mixer was packed with its ingredients")
	}
	mustAct(t, w, Action{Action: "bakery_cancel_edit"})
	advance(w, 40)
	for i := range p.profile.BagSlots {
		p.profile.BagSlots[i] = Stack{"stone", inventoryLimit}
	}
	for i := range p.profile.Hotbar {
		p.profile.Hotbar[i] = Stack{"stone", inventoryLimit}
	}
	syncInventory(&p.profile)
	before = cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "production_collect", Target: "bakery_plot_0_mixer", Item: "dough", Amount: 40}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("full-bag pickup lost hopper contents")
	}
	p.profile.BagSlots[0] = Stack{}
	syncInventory(&p.profile)
	mustAct(t, w, Action{Action: "production_collect", Target: "bakery_plot_0_mixer", Item: "dough", Amount: 40})
	before = cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "production_collect", Target: "bakery_plot_0_mixer", Item: "dough", Amount: 40}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("replayed collection duplicated output")
	}
}

func TestProductionPersistencePausesWithoutClaimAndSnapshotsDetach(t *testing.T) {
	w, p := businessWorld(t)
	baseBakingIngredients(p, 1)
	nodePosition(w, p, "bakery_plot_0_mixer")
	mustAct(t, w, Action{Action: "production_mix", Item: "dough"})
	advance(w, 12)
	remaining := p.profile.Production["bakery_mixer"].Job.Remaining
	snap := w.Snapshot("one")
	snap.Me.Production["bakery_mixer"].Job.Remaining = 999
	for _, n := range snap.Nodes {
		if n.ID == "bakery_plot_0_mixer" {
			n.Production.Job.Amount = 999
			n.Production.Output["dough"] = 999
		}
	}
	if math.Abs(p.profile.Production["bakery_mixer"].Job.Remaining-remaining) > .0001 || p.profile.Production["bakery_mixer"].Job.Amount != 40 {
		t.Fatal("snapshot production aliases live jobs")
	}
	w.Leave("one")
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	q := restored.players["one"]
	advance(restored, 100)
	if q.profile.Production["bakery_mixer"].Job.Remaining != remaining {
		t.Fatal("closed bakery advanced its saved production")
	}
	nodePosition(restored, q, "bakery_plot_1")
	mustAct(t, restored, Action{Action: "buy_land", Target: "bakery_plot_1"})
	advance(restored, 28)
	if q.profile.Production["bakery_mixer"].Output["dough"] != 40 {
		t.Fatal("reclaimed bakery did not resume the exact saved batch")
	}
}

func TestHomeProductionLinksTableAndCannotCreateFreeDough(t *testing.T) {
	w, p := testWorld()
	p.profile.HomeIsland = &Island{Owner: "one", Builders: map[string]bool{}, Chest: map[string]int{}, Objects: []BuildObject{{ID: "mix", Kind: "mixer", X: 0, Z: 0}, {ID: "oven", Kind: "oven", X: -4, Z: 0}, {ID: "table", Kind: "table", X: -4, Z: 3}}}
	p.island = "one"
	p.x, p.z = 0, 2
	baseBakingIngredients(p, 1)
	if err := w.Act("one", Action{Action: "production_mix", Target: "mix", Item: "dough"}); err == nil {
		t.Fatal("home mixer bypassed the bakery purchase requirement")
	}
	mustAct(t, w, Action{Action: "use_build", Target: "mix"})
	if p.profile.Inventory["dough"] != 0 {
		t.Fatal("legacy home interaction produced free dough")
	}
	setItem(p, "dough", 3)
	setItem(p, "berry", 2)
	setItem(p, "nut", 2)
	holdTestItem(t, p, "dough")
	mustAct(t, w, Action{Action: "production_mix", Target: "mix", Item: "cake"})
	advance(w, 3)
	mustAct(t, w, Action{Action: "production_collect", Target: "mix", Item: "prepared_cake"})
	holdTestItem(t, p, "prepared_cake")
	p.x, p.z = -2, 0
	mustAct(t, w, Action{Action: "production_bake", Target: "oven", Item: "prepared_cake"})
	if p.profile.Production["home:oven"].Job.Destination != "home:table" {
		t.Fatal("home oven did not link the nearby table")
	}
	mustAct(t, w, Action{Action: "build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "edit_build", Target: "table"})
	if err := w.Act("one", Action{Action: "pack_build", Target: "table"}); err == nil {
		t.Fatal("linked output table was removed during baking")
	}
	mustAct(t, w, Action{Action: "cancel_build"})
	advance(w, 5)
	p.x, p.z = -2, 3
	mustAct(t, w, Action{Action: "production_collect", Target: "table", Item: "cake"})
	if p.profile.Inventory["cake"] != 1 || p.profile.Production["home:oven"].Output["cake"] != 0 {
		t.Fatal("home baking did not use table pickup")
	}
}

func TestCoolingTableMigrationPreservesCustomLayoutAndPackedStations(t *testing.T) {
	custom := []BakeryFurniture{{ID: "counter", Kind: "counter", X: -1.8, Z: -5.5, Color: "teal"}, {ID: "display", Kind: "display", X: 5, Z: 3}}
	p := Profile{Bakery: true, BakeryFurnitureVersion: 1, BakeryFurniture: append([]BakeryFurniture{}, custom...)}
	normalizeBakeryFurniture(&p)
	if p.BakeryFurnitureVersion != 2 || len(p.BakeryFurniture) != 3 || !reflect.DeepEqual(p.BakeryFurniture[:2], custom) {
		t.Fatal("migration moved furnishings or restored an intentionally packed station")
	}
	cooling := p.BakeryFurniture[2]
	if cooling.Kind != "cooling_table" || bakeryFurnitureOverlaps(cooling, custom[0]) {
		t.Fatal("migration embedded its table in an existing counter")
	}
	p.BakeryFurniture = p.BakeryFurniture[:2]
	normalizeBakeryFurniture(&p)
	if len(p.BakeryFurniture) != 2 {
		t.Fatal("packed cooling table was recreated on reload")
	}
}

func TestCoolingTableMigrationKeepsNarrowKitchenRouteOpen(t *testing.T) {
	custom := []BakeryFurniture{
		{ID: "oven", Kind: "oven", X: -7, Z: -6.4, Rotation: 90},
		{ID: "mixer", Kind: "mixer", X: 6, Z: -6.5, Rotation: 270},
		{ID: "display", Kind: "display", X: 5, Z: 3},
	}
	for i, x := range []float64{-8, -4.9, 1.3, 4.4, 8} {
		custom = append(custom, BakeryFurniture{ID: string(rune('a' + i)), Kind: "counter", X: x, Z: -4.5})
	}
	if !bakeryLayoutReachable(custom) {
		t.Fatal("fixture's original kitchen must be accessible")
	}
	blocked := append(append([]BakeryFurniture{}, custom...), BakeryFurniture{ID: "cooling_table", Kind: "cooling_table", X: -1.8, Z: -5.5})
	if bakeryLayoutReachable(blocked) {
		t.Fatal("fixture's default cooling table must obstruct its sole aisle")
	}
	p := Profile{Bakery: true, BakeryFurnitureVersion: 1, BakeryFurniture: append([]BakeryFurniture{}, custom...)}
	normalizeBakeryFurniture(&p)
	if !reflect.DeepEqual(p.BakeryFurniture[:len(custom)], custom) || !bakeryLayoutReachable(p.BakeryFurniture) {
		t.Fatal("table migration changed the existing layout or sealed its kitchen aisle")
	}
	if len(p.BakeryFurniture) != len(custom)+1 && p.Recovery["bakery_cooling_table"] != 1 {
		t.Fatal("migration neither placed an accessible table nor preserved it as a packed item")
	}
}

func TestChefUsesTheSameTimedPaidProductionWithoutPersonalInventory(t *testing.T) {
	w, p := businessWorld(t)
	p.bakery.nextCustomer = 10000
	mustAct(t, w, Action{Action: "bakery_hire", Item: "chef"})
	for _, item := range []string{"baking_sugar", "butter", "flour", "egg"} {
		p.profile.BakeryPantry[item] = 1
	}
	bag, hotbar := p.profile.BagSlots, p.profile.Hotbar
	for i := 0; i < 200 && (p.profile.Production["bakery_mixer"] == nil || p.profile.Production["bakery_mixer"].Job == nil); i++ {
		w.Tick(.1)
	}
	mixer := p.profile.Production["bakery_mixer"]
	if mixer == nil || mixer.Job == nil || mixer.Job.Total != 40 || mixer.Job.Amount != 40 || bakeryStockCount(p.profile.BakeryPantry) != 0 {
		t.Fatal("chef did not consume the same four ingredients into a timed40-dough batch")
	}
	advance(w, 39)
	if p.profile.BakeryDisplay["sugar"] != 0 || p.profile.Production["bakery_cooling_table"].Output["sugar"] != 0 {
		t.Fatal("chef bypassed the mixer timer")
	}
	advance(w, 7)
	if p.profile.BakeryDisplay["sugar"] < 1 || p.profile.BagSlots != bag || p.profile.Hotbar != hotbar {
		t.Fatal("chef failed to bake through table output or used the player's bag")
	}
	total := p.profile.BakeryPantry["dough"] + p.profile.BakeryDisplay["sugar"] + p.profile.Production["bakery_cooling_table"].Output["sugar"] + p.profile.Production["bakery_mixer"].Output["dough"]
	if job := p.profile.Production["bakery_oven"].Job; job != nil {
		total += job.Amount
	}
	if total != 40 {
		t.Fatalf("chef changed the paid batch yield: got%d want40", total)
	}
}
