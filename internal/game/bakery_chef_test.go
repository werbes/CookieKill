package game

import (
	"reflect"
	"strings"
	"testing"
)

func chefTestReady(t *testing.T, w *World, p *player, ids ...string) {
	t.Helper()
	point, ok := w.bakeryStationServicePoint(p, "oven", false)
	if !ok {
		t.Fatal("missing oven")
	}
	for _, id := range ids {
		p.profile.BakeryStaff[id] = true
		p.bakery.workers[id] = &bakeryWorker{x: point.X, z: point.Z, goal: point, hasRoute: true}
	}
}
func chefTestExtraOven(p *player) {
	p.profile.BakeryFurniture = append(p.profile.BakeryFurniture, BakeryFurniture{ID: "extra_oven", Kind: "oven", X: 1.5, Z: -5.5})
}
func chefTestSelections(p *player, ids ...string) {
	p.profile.BakeryChefRecipes = append([]string{}, ids...)
	for _, id := range ids {
		p.profile.Discovered[id] = true
	}
	normalizeBakeryChefRecipes(&p.profile)
}

func TestChefRecipeSelectionsMigrateValidatePersistAndStayDetached(t *testing.T) {
	legacy := Profile{BakeryChefRecipe: "berry_cookie", Discovered: map[string]bool{"berry_cookie": true}}
	normalizeBakeryChefRecipes(&legacy)
	if !reflect.DeepEqual(legacy.BakeryChefRecipes, []string{"berry_cookie"}) {
		t.Fatal("legacy choice lost")
	}
	legacy.BakeryChefRecipes = []string{"sugar", "cake", "unknown", "berry_cookie", "sugar"}
	normalizeBakeryChefRecipes(&legacy)
	if !reflect.DeepEqual(legacy.BakeryChefRecipes, []string{"sugar", "berry_cookie"}) {
		t.Fatal("invalid, unknown or duplicate selection survived")
	}
	w, p := businessWorld(t)
	p.profile.Discovered["berry_cookie"] = true
	nodePosition(w, p, "bakery_plot_0_oven")
	mustAct(t, w, Action{Action: "bakery_chef_recipe_set", Item: "berry_cookie", Enabled: true})
	if !reflect.DeepEqual(p.profile.BakeryChefRecipes, []string{"sugar", "berry_cookie"}) {
		t.Fatal("adding a recipe replaced existing selection")
	}
	mustAct(t, w, Action{Action: "bakery_chef_recipe_set", Item: "sugar", Enabled: false})
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "bakery_chef_recipe_set", Item: "berry_cookie", Enabled: false}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("removing last selection was not an atomic rejection")
	}
	if err := w.Act("one", Action{Action: "bakery_chef_recipe_set", Item: "cake", Enabled: true}); err == nil {
		t.Fatal("unknown recipe accepted")
	}
	mustAct(t, w, Action{Action: "bakery_chef_recipe", Item: "sugar"})
	if !reflect.DeepEqual(p.profile.BakeryChefRecipes, []string{"sugar"}) {
		t.Fatal("legacy single select did not replace list")
	}
	mustAct(t, w, Action{Action: "bakery_chef_recipe_set", Item: "berry_cookie", Enabled: true})
	snapshot := w.Snapshot("one")
	snapshot.Me.BakeryChefRecipes[0] = "cake"
	copy := cloneProfile(p.profile)
	copy.BakeryChefRecipes[1] = "cake"
	if !reflect.DeepEqual(p.profile.BakeryChefRecipes, []string{"sugar", "berry_cookie"}) {
		t.Fatal("snapshot/profile clone aliases selected recipes")
	}
	w.Leave("one")
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	if !reflect.DeepEqual(restored.players["one"].profile.BakeryChefRecipes, []string{"sugar", "berry_cookie"}) {
		t.Fatal("selected recipes did not survive restart")
	}
}

func TestChefUnavailableCakeFallsBackAndReadsOnlyFridge(t *testing.T) {
	w, p := businessWorld(t)
	chefTestReady(t, w, p, "chef")
	chefTestSelections(p, "cake", "sugar")
	p.profile.BakeryPantry = map[string]int{"dough": 4, "berry": 2}
	setItem(p, "nut", 10)
	setItem(p, "dough", 8)
	p.profile.BakeryDisplay = map[string]int{"cake": 5}
	p.profile.HomeIsland = &Island{Owner: "one", Chest: map[string]int{"nut": 9}, Objects: []BuildObject{{ID: "home_mixer", Kind: "mixer", X: 1, Z: 1}}}
	productionState(&p.profile, "home:home_mixer").Output["prepared_cake"] = 3
	before := cloneProfile(p.profile)
	advance(w, 30)
	if got := productionState(&p.profile, "bakery_cooling_table").Output; got["sugar"] != 4 || got["cake"] != 0 {
		t.Fatalf("unaffordable cake stopped plain baking: %v", got)
	}
	if !reflect.DeepEqual(before.Inventory, p.profile.Inventory) || !reflect.DeepEqual(before.BakeryDisplay, p.profile.BakeryDisplay) || !reflect.DeepEqual(before.HomeIsland.Chest, p.profile.HomeIsland.Chest) || !reflect.DeepEqual(before.Production["home:home_mixer"], p.profile.Production["home:home_mixer"]) {
		t.Fatalf("chef consumed personal, display, home or chest supplies: inventory=%v/%v display=%v/%v home=%v/%v", before.Inventory, p.profile.Inventory, before.BakeryDisplay, p.profile.BakeryDisplay, before.Production["home:home_mixer"], p.profile.Production["home:home_mixer"])
	}
	if p.profile.BakeryPantry["berry"] != 2 {
		t.Fatal("failed recipe wasted affordable subset of ingredients")
	}
}

func TestTwoChefsStartDistinctParallelJobsAndOneChefCanOverseeTwoOvens(t *testing.T) {
	for _, two := range []bool{false, true} {
		w, p := businessWorld(t)
		chefTestExtraOven(p)
		chefTestReady(t, w, p, "chef")
		if two {
			chefTestReady(t, w, p, "chef_2")
		}
		chefTestSelections(p, "cake", "berry_cookie", "sugar")
		p.profile.BakeryPantry = map[string]int{"prepared_cake": 2, "prepared_berry_cookie": 2, "dough": 2}
		w.workBakeryChef(p)
		first, second := productionState(&p.profile, "bakery_oven"), productionState(&p.profile, "bakery:extra_oven")
		if first.Job == nil || first.Job.Item != "cake" {
			t.Fatal("round-robin did not start first feasible selected recipe")
		}
		if !two {
			if second.Job != nil {
				t.Fatal("one chef started multiple tasks in one pass")
			}
			w.time += bakeryChefHandlingSeconds
			w.workBakeryChef(p)
		}
		if second.Job == nil || second.Job.Item != "berry_cookie" {
			t.Fatal("independent work failed to bake two kinds concurrently")
		}
		if two && p.bakery.workers["chef"].chefJobKey == p.bakery.workers["chef_2"].chefJobKey {
			t.Fatal("both chefs claimed same job")
		}
		advance(w, 25)
		if got := productionState(&p.profile, "bakery_cooling_table").Output; got["cake"] != 2 || got["berry_cookie"] != 2 || got["sugar"] != 2 {
			t.Fatalf("recipe fairness starved a selected kind: %v", got)
		}
	}
}

func TestChefBreakStopsOnlyThatWorkerAndKeepsBatchesOnSelectionChange(t *testing.T) {
	w, p := businessWorld(t)
	chefTestExtraOven(p)
	chefTestReady(t, w, p, "chef", "chef_2")
	chefTestSelections(p, "sugar")
	p.profile.Discovered["berry_cookie"] = true
	p.profile.BakeryPantry = map[string]int{"dough": 8, "berry": 4}
	p.profile.BakeryStaffBreaks["chef_2"] = true
	w.workBakeryChef(p)
	if productionState(&p.profile, "bakery_oven").Job == nil || productionState(&p.profile, "bakery:extra_oven").Job != nil {
		t.Fatal("worker on break started work")
	}
	delete(p.profile.BakeryStaffBreaks, "chef_2")
	w.workBakeryChef(p)
	if productionState(&p.profile, "bakery:extra_oven").Job == nil {
		t.Fatal("resumed second chef did not independently use available oven")
	}
	p.profile.BakeryStaffBreaks["chef"], p.profile.BakeryStaffBreaks["chef_2"] = true, true
	before := cloneMap(p.profile.BakeryPantry)
	nodePosition(w, p, "bakery_plot_0_oven")
	mustAct(t, w, Action{Action: "bakery_chef_recipe", Item: "berry_cookie"})
	advance(w, 8)
	if !reflect.DeepEqual(before, p.profile.BakeryPantry) || productionState(&p.profile, "bakery_cooling_table").Output["sugar"] != 2 {
		t.Fatal("break or recipe change lost, replaced or duplicated already-loaded batches")
	}
	if !strings.Contains(p.bakery.chefStatus, "On break") {
		t.Fatal("break status omitted")
	}
	delete(p.profile.BakeryStaffBreaks, "chef_2")
	advance(w, 20)
	if got := productionState(&p.profile, "bakery_cooling_table").Output; got["sugar"] != 2 || got["berry_cookie"] != 2 {
		t.Fatalf("resume failed to continue selected recipe from fridge: %v", got)
	}
}

func TestChefBlockedApplianceAndMissingMixerDoNotStopAlternateOven(t *testing.T) {
	for _, blocked := range []string{"editing", "obstructed", "mixer_packed"} {
		w, p := businessWorld(t)
		chefTestExtraOven(p)
		chefTestReady(t, w, p, "chef")
		chefTestSelections(p, "sugar")
		p.profile.BakeryPantry = map[string]int{"dough": 2}
		switch blocked {
		case "editing":
			p.editingBakeryFurnitureID = "oven"
		case "obstructed":
			p.profile.BakeryFurniture = append(p.profile.BakeryFurniture, BakeryFurniture{ID: "obstruction", Kind: "counter", X: -5, Z: -3.7})
			point, _ := w.bakeryFurnitureServicePoint(p, p.profile.BakeryFurniture[len(p.profile.BakeryFurniture)-2], false)
			worker := p.bakery.workers["chef"]
			worker.x, worker.z, worker.goal = point.X, point.Z, point
		case "mixer_packed":
			kept := []BakeryFurniture{}
			for _, f := range p.profile.BakeryFurniture {
				if f.Kind != "mixer" {
					kept = append(kept, f)
				}
			}
			p.profile.BakeryFurniture = kept
		}
		w.bakeryFurnitureChanged(p)
		advance(w, 25)
		if productionState(&p.profile, "bakery_cooling_table").Output["sugar"] != 2 {
			t.Fatalf("%s blocked unrelated usable appliances: %s", blocked, p.bakery.chefStatus)
		}
	}
}

func TestChefMixesAffordableSelectedRecipesAndRetainsPreparedBatches(t *testing.T) {
	w, p := businessWorld(t)
	chefTestExtraOven(p)
	chefTestReady(t, w, p, "chef", "chef_2")
	chefTestSelections(p, "cake", "sugar")
	p.profile.BakeryPantry = map[string]int{"dough": 8, "berry": 4, "nut": 4}
	advance(w, 35)
	got := productionState(&p.profile, "bakery_cooling_table").Output
	if got["cake"] != 2 || got["sugar"] != 2 {
		t.Fatalf("fresh add-in mixing failed fair selected production: %v, fridge=%v", got, p.profile.BakeryPantry)
	}
	if len(p.profile.BakeryPantry) != 0 {
		t.Fatal("unexpected ingredient waste or leftovers")
	}
	// Two dough is insufficient for cake, but must not block making a fresh base batch.
	w, p = businessWorld(t)
	chefTestReady(t, w, p, "chef")
	chefTestSelections(p, "cake")
	p.profile.BakeryPantry = map[string]int{"dough": 2, "berry": 2, "nut": 2, "baking_sugar": 1, "butter": 1, "flour": 1, "egg": 1}
	w.workBakeryChef(p)
	if job := productionState(&p.profile, "bakery_mixer").Job; job == nil || job.Item != "dough" || job.Amount != 40 {
		t.Fatal("partly stocked dough prevented an affordable base batch")
	}
	p.profile.BakeryStaffBreaks["chef"] = true
	advance(w, 45)
	if productionState(&p.profile, "bakery_mixer").Output["dough"] != 40 || p.profile.BakeryPantry["dough"] != 2 {
		t.Fatal("break lost completed mixer output or silently used it")
	}
	delete(p.profile.BakeryStaffBreaks, "chef")
	advance(w, 20)
	if productionState(&p.profile, "bakery_cooling_table").Output["cake"] != 1 || p.profile.BakeryPantry["dough"] != 39 {
		t.Fatal("resume did not retain and use completed base dough exactly")
	}
}

func TestChefRecipeSwitchAndRestartPreserveAlreadyPaidPreparedCake(t *testing.T) {
	w, p := businessWorld(t)
	chefTestReady(t, w, p, "chef")
	chefTestSelections(p, "cake")
	p.profile.BakeryPantry = map[string]int{"dough": 5, "berry": 2, "nut": 2}
	w.workBakeryChef(p)
	job := productionState(&p.profile, "bakery_mixer").Job
	if job == nil || job.Item != "prepared_cake" || p.profile.BakeryPantry["dough"] != 2 {
		t.Fatal("cake preparation did not reserve its exact complete cost")
	}
	p.profile.BakeryStaffBreaks["chef"] = true
	nodePosition(w, p, "bakery_plot_0_oven")
	mustAct(t, w, Action{Action: "bakery_chef_recipe", Item: "sugar"})
	if productionState(&p.profile, "bakery_mixer").Job != job {
		t.Fatal("selection change replaced an in-progress paid batch")
	}
	w.Leave("one")
	w = New(w.Profiles())
	w.Join("one", "Baker")
	p = w.players["one"]
	nodePosition(w, p, "bakery_plot_1")
	mustAct(t, w, Action{Action: "buy_land", Target: "bakery_plot_1"})
	advance(w, 4)
	if !p.profile.BakeryStaffBreaks["chef"] || productionState(&p.profile, "bakery_mixer").Output["prepared_cake"] != 1 || p.profile.BakeryPantry["dough"] != 2 {
		t.Fatal("restart/break did not preserve and finish the paid prepared cake")
	}
	delete(p.profile.BakeryStaffBreaks, "chef")
	advance(w, 20)
	if p.profile.BakeryPantry["prepared_cake"] != 1 || productionState(&p.profile, "bakery_cooling_table").Output["sugar"] != 2 {
		t.Fatal("new selection consumed the old prepared cake or stopped sugar production")
	}
	nodePosition(w, p, "bakery_plot_1_oven")
	mustAct(t, w, Action{Action: "bakery_chef_recipe", Item: "cake"})
	advance(w, 10)
	if p.profile.BakeryPantry["prepared_cake"] != 0 || productionState(&p.profile, "bakery_cooling_table").Output["cake"] != 1 {
		t.Fatal("reselecting did not bake the saved cake exactly once")
	}
}
