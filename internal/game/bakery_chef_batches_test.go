package game

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestChefOvenChoosesLargestAffordablePlainAndPreparedBatch(t *testing.T) {
	for _, tc := range []struct {
		item, input    string
		supplied, want int
	}{
		{"sugar", "dough", 1, 1}, {"sugar", "dough", 7, 7}, {"sugar", "dough", 40, 40}, {"sugar", "dough", 83, 40},
		{"berry_cookie", "prepared_berry_cookie", 3, 3}, {"cake", "prepared_cake", 40, 40}, {"cake", "prepared_cake", 55, 40},
	} {
		t.Run(fmt.Sprintf("%s_%d", tc.item, tc.supplied), func(t *testing.T) {
			w, p := businessWorld(t)
			chefTestReady(t, w, p, "chef")
			chefTestSelections(p, tc.item)
			p.profile.BakeryPantry = map[string]int{tc.input: tc.supplied}
			bag, hotbar := p.profile.BagSlots, p.profile.Hotbar
			w.workBakeryChef(p)
			oven := productionState(&p.profile, "bakery_oven")
			if oven.Job == nil || oven.Job.Item != tc.item || oven.Job.Amount != tc.want || oven.Job.Total != float64(tc.want*5) || oven.Job.Destination != "bakery_cooling_table" {
				t.Fatalf("wrong paid oven batch: %+v", oven.Job)
			}
			if p.profile.BakeryPantry[tc.input] != tc.supplied-tc.want || p.profile.BagSlots != bag || p.profile.Hotbar != hotbar {
				t.Fatal("batch debit escaped the fridge or paid the wrong amount")
			}
			if !strings.Contains(p.bakery.chefStatus, fmt.Sprintf("Baking %d ", tc.want)) {
				t.Fatal("chef status omitted batch size")
			}
			w.tickProduction(float64(tc.want*5) - .01)
			if productionState(&p.profile, "bakery_cooling_table").Output[tc.item] != 0 {
				t.Fatal("part of a batch appeared before its full oven timer")
			}
			w.tickProduction(.01)
			if oven.Job != nil || productionState(&p.profile, "bakery_cooling_table").Output[tc.item] != tc.want {
				t.Fatal("completed batch did not deliver exactly once")
			}
		})
	}
}

func TestChefRecipeBatchesScaleAddinsDoughTimeAndProteinSubstitutes(t *testing.T) {
	for _, tc := range []struct {
		name, item  string
		stock, left map[string]int
		amount      int
		duration    float64
	}{
		{"full_berry", "berry_cookie", map[string]int{"dough": 100, "berry": 160}, map[string]int{"dough": 60, "berry": 80}, 40, 40},
		{"full_cake", "cake", map[string]int{"dough": 121, "berry": 80, "nut": 80}, map[string]int{"dough": 1}, 40, 120},
		{"partial_cake", "cake", map[string]int{"dough": 15, "berry": 11, "nut": 5}, map[string]int{"dough": 9, "berry": 7, "nut": 1}, 2, 6},
		{"mixed_protein", "protein_cookie", map[string]int{"dough": 100, "protein_powder": 3, "protein_drink": 7}, map[string]int{"dough": 90}, 10, 10},
		{"full_protein", "protein_cookie", map[string]int{"dough": 50, "protein_powder": 3, "protein_drink": 45}, map[string]int{"dough": 10, "protein_drink": 8}, 40, 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, p := businessWorld(t)
			chefTestReady(t, w, p, "chef")
			chefTestSelections(p, tc.item)
			p.profile.BakeryPantry = cloneMap(tc.stock)
			before := cloneMap(p.profile.Inventory)
			w.workBakeryChef(p)
			mixer := productionState(&p.profile, "bakery_mixer")
			if mixer.Job == nil || mixer.Job.Item != "prepared_"+tc.item || mixer.Job.Amount != tc.amount || mixer.Job.Total != tc.duration {
				t.Fatalf("wrong mixer batch: %+v", mixer.Job)
			}
			if !reflect.DeepEqual(p.profile.BakeryPantry, tc.left) || !reflect.DeepEqual(before, p.profile.Inventory) {
				t.Fatalf("wrong complete recipe debit: got%v want%v", p.profile.BakeryPantry, tc.left)
			}
			if !strings.Contains(p.bakery.chefStatus, fmt.Sprintf("Preparing %d ", tc.amount)) {
				t.Fatal("preparation status omitted amount")
			}
			p.profile.BakeryStaffBreaks["chef"] = true
			w.tickProduction(tc.duration)
			if mixer.Job != nil || mixer.Output["prepared_"+tc.item] != tc.amount {
				t.Fatal("prepared batch changed yield during timer/break")
			}
		})
	}
}

func TestChefBatchSharedFridgeAndCoolingReservationsAreAtomicAcrossTwoWorkers(t *testing.T) {
	for _, tc := range []struct {
		input, item                        string
		stock, cooled, first, second, left int
	}{
		{"dough", "sugar", 75, 0, 40, 35, 0},
		{"prepared_berry_cookie", "berry_cookie", 55, 0, 40, 15, 0},
		{"dough", "sugar", 75, 140, 40, 20, 15},
	} {
		t.Run(fmt.Sprintf("%s_%d", tc.item, tc.cooled), func(t *testing.T) {
			w, p := businessWorld(t)
			chefTestExtraOven(p)
			chefTestReady(t, w, p, "chef", "chef_2")
			chefTestSelections(p, tc.item)
			p.profile.BakeryPantry = map[string]int{tc.input: tc.stock}
			if tc.cooled > 0 {
				productionState(&p.profile, "bakery_cooling_table").Output["cake"] = tc.cooled
			}
			w.workBakeryChef(p)
			first, second := productionState(&p.profile, "bakery_oven").Job, productionState(&p.profile, "bakery:extra_oven").Job
			if first == nil || second == nil || first.Amount != tc.first || second.Amount != tc.second || p.profile.BakeryPantry[tc.input] != tc.left {
				t.Fatalf("two-chef debit/reservation not atomic: first%+v second%+v fridge%v", first, second, p.profile.BakeryPantry)
			}
			if p.bakery.workers["chef"].chefJobKey == p.bakery.workers["chef_2"].chefJobKey {
				t.Fatal("workers claimed same batch")
			}
			w.tickProduction(200)
			table := productionState(&p.profile, "bakery_cooling_table")
			if table.Output[tc.item] != tc.first+tc.second || bakeryStockCount(table.Output) > productionCapacity {
				t.Fatal("two loaded batches overflowed or lost cooling goods")
			}
		})
	}
}

func TestChefCoolingReservationsIncludeUnreachableLegacyQueuedAndCompletedJobs(t *testing.T) {
	w, p := businessWorld(t)
	chefTestExtraOven(p)
	chefTestReady(t, w, p, "chef")
	chefTestSelections(p, "sugar")
	p.profile.BakeryPantry = map[string]int{"dough": 20}
	table := productionState(&p.profile, "bakery_cooling_table")
	table.Output["cake"] = 185
	other := productionState(&p.profile, "bakery:extra_oven")
	other.Job = &ProductionJob{Item: "sugar", Amount: 4, Total: 20, Remaining: 0} // Legacy destination and already finished.
	other.Queue = []ProductionJob{{Item: "berry_cookie", Amount: 3, Total: 15, Remaining: 15, Destination: "bakery_cooling_table"}}
	p.editingBakeryFurnitureID = "extra_oven"
	p.profile.HomeIsland = &Island{Owner: "one", Objects: []BuildObject{{ID: "oven", Kind: "oven"}}}
	productionState(&p.profile, "home:oven").Job = &ProductionJob{Item: "sugar", Amount: 99, Total: 495, Remaining: 495}
	w.workBakeryChef(p)
	job := productionState(&p.profile, "bakery_oven").Job
	if job == nil || job.Amount != 8 || job.Total != 40 || p.profile.BakeryPantry["dough"] != 12 {
		t.Fatalf("incoming city reservations or home exclusion wrong: job%+v fridge%v", job, p.profile.BakeryPantry)
	}
	if bakeryChefCoolingSpace(&p.profile, table) != 0 {
		t.Fatal("new chef batch did not reserve its destination immediately")
	}
	// Manual queues can already overbook. Preserve them and consume nothing else.
	table.Output["cake"] = 198
	first := cloneProfile(p.profile)
	p.bakery.workers["chef"].chefReady = 0
	w.workBakeryChef(p)
	if !reflect.DeepEqual(first.BakeryPantry, p.profile.BakeryPantry) || !reflect.DeepEqual(first.Production, p.profile.Production) {
		t.Fatal("overbooked manual work caused extra chef consumption or lost queued jobs")
	}
	w.tickProduction(50)
	if other.Job == nil || other.Job.Remaining != 0 || other.Job.Amount != 4 || bakeryStockCount(table.Output) != 198 {
		t.Fatal("already-overbooked batch did not remain safely held")
	}
}

func TestChefPartialCoolingCapacityLimitsRecipePreparationWithoutLosingAddins(t *testing.T) {
	w, p := businessWorld(t)
	chefTestReady(t, w, p, "chef")
	chefTestSelections(p, "berry_cookie")
	p.profile.BakeryPantry = map[string]int{"dough": 50, "berry": 100}
	productionState(&p.profile, "bakery_cooling_table").Output["sugar"] = 197
	w.workBakeryChef(p)
	job := productionState(&p.profile, "bakery_mixer").Job
	if job == nil || job.Amount != 3 || job.Total != 3 || p.profile.BakeryPantry["dough"] != 47 || p.profile.BakeryPantry["berry"] != 94 {
		t.Fatal("small cooling gap caused oversized preparation or partial cost loss")
	}
	w.tickProduction(3)
	w.time += 3
	w.workBakeryChef(p)
	job = productionState(&p.profile, "bakery_oven").Job
	if job == nil || job.Amount != 3 || job.Total != 15 {
		t.Fatal("prepared partial batch did not fit its remaining cooling capacity")
	}
	w.tickProduction(15)
	if table := productionState(&p.profile, "bakery_cooling_table"); table.Output["berry_cookie"] != 3 || bakeryStockCount(table.Output) != 200 {
		t.Fatal("partial batch did not fill table exactly")
	}
}

func TestChefFullBatchesPersistAcrossBreakDisconnectAndRestart(t *testing.T) {
	w, p := businessWorld(t)
	chefTestExtraOven(p)
	chefTestReady(t, w, p, "chef", "chef_2")
	chefTestSelections(p, "sugar")
	p.profile.BakeryPantry = map[string]int{"dough": 72}
	w.workBakeryChef(p)
	p.profile.BakeryStaffBreaks["chef"], p.profile.BakeryStaffBreaks["chef_2"] = true, true
	w.tickProduction(11)
	before := cloneProfile(p.profile)
	w.Leave("one")
	advance(w, 30)
	if !reflect.DeepEqual(before.Production, w.Profiles()["one"].Production) {
		t.Fatal("disconnected bakery advanced or lost paid batches")
	}
	w = New(w.Profiles())
	w.Join("one", "Baker")
	p = w.players["one"]
	if !reflect.DeepEqual(before.Production, p.profile.Production) || !reflect.DeepEqual(before.BakeryStaffBreaks, p.profile.BakeryStaffBreaks) {
		t.Fatal("restart changed paid batches or worker breaks")
	}
	nodePosition(w, p, "bakery_plot_1")
	mustAct(t, w, Action{Action: "buy_land", Target: "bakery_plot_1"})
	advance(w, 195)
	if productionState(&p.profile, "bakery_cooling_table").Output["sugar"] != 72 || len(p.profile.BakeryPantry) != 0 || productionState(&p.profile, "bakery_oven").Job != nil || productionState(&p.profile, "bakery:extra_oven").Job != nil {
		t.Fatal("reopened bakery did not finish both saved batches exactly once")
	}
	if before.BagSlots != p.profile.BagSlots || before.Hotbar != p.profile.Hotbar {
		t.Fatal("batch restart used personal inventory")
	}
}
