package game

import (
	"errors"
	"fmt"
	"strings"
)

const bakeryChefHandlingSeconds = 2.0
const bakeryChefBatchLimit = 40

// The legacy single recipe migrates only when no selected list was saved.
// Retaining a nonempty, deduplicated list makes an accidental empty click safe.
func normalizeBakeryChefRecipes(p *Profile) {
	selected := []string{}
	seen := map[string]bool{}
	for _, id := range p.BakeryChefRecipes {
		if _, ok := recipeFor(id); ok && (id == "sugar" || p.Discovered[id]) && !seen[id] {
			selected = append(selected, id)
			seen[id] = true
		}
	}
	if len(selected) == 0 {
		id := p.BakeryChefRecipe
		if _, ok := recipeFor(id); !ok || id != "sugar" && !p.Discovered[id] {
			id = "sugar"
		}
		selected = append(selected, id)
	}
	p.BakeryChefRecipes = selected
	p.BakeryChefRecipe = selected[0]
}

func (w *World) bakeryChefRecipeAction(p *player, a Action) error {
	if err := w.requireBakeryStation(p, "oven", a.Target); err != nil {
		return err
	}
	if _, ok := recipeFor(a.Item); !ok || a.Item != "sugar" && !p.profile.Discovered[a.Item] {
		return errors.New("Bake this recipe yourself to teach it to your chefs first.")
	}
	normalizeBakeryChefRecipes(&p.profile)
	selected := append([]string{}, p.profile.BakeryChefRecipes...)
	if a.Action == "bakery_chef_recipe" {
		selected = []string{a.Item}
	} else {
		index := -1
		for i, id := range selected {
			if id == a.Item {
				index = i
				break
			}
		}
		if a.Enabled && index < 0 {
			selected = append(selected, a.Item)
		}
		if !a.Enabled && index >= 0 {
			if len(selected) == 1 {
				return errors.New("Keep at least one recipe selected. Use staff breaks to pause your chefs.")
			}
			selected = append(selected[:index:index], selected[index+1:]...)
		}
	}
	p.profile.BakeryChefRecipes, p.profile.BakeryChefRecipe = selected, selected[0]
	if p.bakery != nil {
		p.bakery.chefRecipeCursor = 0
	}
	return nil
}

type bakeryChefStation struct {
	key   string
	state *StationProduction
}

// Each appliance is checked independently. An edited or obstructed first oven
// must not hide an accessible second oven, or an unused mixer stop plain bakes.
func (w *World) bakeryChefKitchen(p *player, worker *bakeryWorker) (mixers, ovens []bakeryChefStation, cooling *StationProduction, reason string) {
	fridges := []Point{}
	for _, f := range p.profile.BakeryFurniture {
		if f.Kind != "fridge" || f.ID == p.editingBakeryFurnitureID {
			continue
		}
		point, ok := w.bakeryFurnitureServicePoint(p, f, false)
		if !ok {
			continue
		}
		if _, reachable := w.bakeryRoute(p, Point{worker.x, worker.z}, point); reachable {
			fridges = append(fridges, point)
		}
	}
	if len(fridges) == 0 {
		return nil, nil, nil, "Place the fridge and clear its kitchen route"
	}
	for _, f := range p.profile.BakeryFurniture {
		if !validChoice(f.Kind, "mixer", "oven", "cooling_table") || f.ID == p.editingBakeryFurnitureID {
			continue
		}
		point, ok := w.bakeryFurnitureServicePoint(p, f, false)
		if !ok {
			continue
		}
		accessible := false
		for _, fridge := range fridges {
			if _, ok := w.bakeryRoute(p, fridge, point); ok {
				accessible = true
				break
			}
		}
		if !accessible {
			continue
		}
		station := bakeryChefStation{key: bakeryProductionKey(f), state: productionState(&p.profile, bakeryProductionKey(f))}
		switch f.Kind {
		case "oven":
			ovens = append(ovens, station)
		case "mixer":
			mixers = append(mixers, station)
		case "cooling_table":
			if cooling == nil {
				cooling = station.state
			}
		}
	}
	if cooling == nil {
		return mixers, ovens, nil, "Place the cooling table and clear its kitchen route"
	}
	if len(ovens) == 0 {
		return mixers, nil, cooling, "Place an oven and clear its kitchen route"
	}
	return mixers, ovens, cooling, ""
}

func bakeryChefCanAfford(stock, cost map[string]int) bool {
	for item, amount := range cost {
		if amount > stock[item] {
			return false
		}
	}
	return true
}
func bakeryChefSpend(stock, cost map[string]int) {
	for item, amount := range cost {
		stock[item] -= amount
		if stock[item] <= 0 {
			delete(stock, item)
		}
	}
}
func bakeryChefPrepared(p *player, mixers []bakeryChefStation, item string) int {
	n := p.profile.BakeryPantry[item]
	for _, mixer := range mixers {
		state := mixer.state
		n += state.Output[item]
		if state.Job != nil && state.Job.Item == item {
			n += state.Job.Amount
		}
		for _, job := range state.Queue {
			if job.Item == item {
				n += job.Amount
			}
		}
	}
	return n
}

// Reserve the destination for every paid bakery oven batch, including jobs in
// edited or inaccessible ovens and queued manual jobs. A second chef observes
// the first chef's reservation immediately through the new production job.
func bakeryChefCoolingSpace(p *Profile, cooling *StationProduction) int {
	space := productionCapacity - bakeryStockCount(cooling.Output)
	for key, station := range p.Production {
		if station == nil || !strings.HasPrefix(key, "bakery_") && !strings.HasPrefix(key, "bakery:") {
			continue
		}
		if kind, valid := productionKind(p, key); !valid || kind != "oven" {
			continue
		}
		reserve := func(job ProductionJob) {
			if job.Destination == "" || job.Destination == "bakery_cooling_table" {
				space -= job.Amount
			}
		}
		if station.Job != nil {
			reserve(*station.Job)
		}
		for _, job := range station.Queue {
			reserve(job)
		}
	}
	return max(0, space)
}

func bakeryChefRecipeBatch(recipe Recipe, stock map[string]int, limit int) (int, map[string]int) {
	for amount := min(bakeryChefBatchLimit, limit); amount > 0; amount-- {
		// Protein powder and drinks are substitutes, so recalculate the full
		// cost for each batch size instead of multiplying a one-cookie cost.
		cost := productionAddinCost(recipe, amount, stock)
		if bakeryChefCanAfford(stock, cost) {
			return amount, cost
		}
	}
	return 0, nil
}

func bakeryChefJobStatus(p *player, worker *bakeryWorker) string {
	if s := p.profile.Production[worker.chefJobKey]; s != nil && s.Job != nil {
		if r, ok := recipeFor(s.Job.Item); ok {
			return fmt.Sprintf("Baking %d %s", s.Job.Amount, r.Name)
		}
		if r, ok := preparedRecipe(s.Job.Item); ok {
			return fmt.Sprintf("Preparing %d %s", s.Job.Amount, r.Name)
		}
		if s.Job.Item == "dough" {
			return fmt.Sprintf("Mixing %d fresh dough", s.Job.Amount)
		}
	}
	return "Waiting for an available appliance or fridge ingredients"
}

func (w *World) workBakeryChef(p *player) {
	b := p.bakery
	if b == nil || p.disconnected {
		return
	}
	ids := bakeryWorkerIDs(&p.profile, "chef")
	if len(ids) == 0 {
		b.chefStatus = "Not hired"
		return
	}
	normalizeBakeryChefRecipes(&p.profile)
	selected := p.profile.BakeryChefRecipes
	statuses := []string{}
	for _, id := range ids {
		worker := b.workers[id]
		status := "Walking to the kitchen"
		if bakeryWorkerOnBreak(&p.profile, id) {
			status = "On break; existing batches finish naturally"
		} else if worker != nil && w.bakeryWorkerReady(p, id) {
			status = w.workBakeryChefWorker(p, worker, selected)
		} else if worker != nil && worker.status != "" {
			status = worker.status
		}
		if worker != nil {
			worker.status = status
		}
		statuses = append(statuses, status)
	}
	b.chefStatus = strings.Join(statuses, " / ")
}

// A chef handles at most one new task per pass. The independent per-worker
// cooldown makes a second chef useful while existing appliances keep running.
func (w *World) workBakeryChefWorker(p *player, worker *bakeryWorker, selected []string) string {
	if w.time < worker.chefReady {
		return bakeryChefJobStatus(p, worker)
	}
	mixers, ovens, cooling, reason := w.bakeryChefKitchen(p, worker)
	if reason != "" {
		return reason
	}
	stock := p.profile.BakeryPantry
	for _, mixer := range mixers {
		// Stable output order avoids depending on map iteration when the fridge is nearly full.
		items := []string{"dough"}
		for _, r := range recipes {
			items = append(items, "prepared_"+r.ID)
		}
		for _, item := range items {
			n := min(mixer.state.Output[item], bakeryPantryCapacity-bakeryStockCount(stock))
			if n <= 0 {
				continue
			}
			stock[item] += n
			mixer.state.Output[item] -= n
			if mixer.state.Output[item] == 0 {
				delete(mixer.state.Output, item)
			}
		}
	}
	coolingSpace := bakeryChefCoolingSpace(&p.profile, cooling)
	if coolingSpace == 0 {
		return "Cooling table full or reserved: collect bakes or use a server"
	}
	// Advance the cursor only after loading an oven. Preparing the next
	// affordable recipe first reserves its full ingredient cost before a fast
	// fallback recipe can consume the dough it needs. Pending prep is skipped,
	// so another chef can use an oven or mixer for a different selection.
	for step := 0; step < len(selected); step++ {
		index := (p.bakery.chefRecipeCursor + step) % len(selected)
		r, _ := recipeFor(selected[index])
		input := "prepared_" + r.ID
		if r.ID == "sugar" {
			input = "dough"
		}
		if stock[input] > 0 {
			for _, oven := range ovens {
				if oven.state.Job != nil || len(oven.state.Queue) > 0 {
					continue
				}
				amount := min(bakeryChefBatchLimit, stock[input], coolingSpace)
				bakeryChefSpend(stock, map[string]int{input: amount})
				startProduction(oven.state, r.ID, amount, float64(amount)*5)
				oven.state.Job.Destination = "bakery_cooling_table"
				p.bakery.chefRecipeCursor = (index + 1) % len(selected)
				worker.chefJobKey, worker.chefReady = oven.key, w.time+bakeryChefHandlingSeconds
				return bakeryChefJobStatus(p, worker)
			}
		}
		if r.ID != "sugar" && bakeryChefPrepared(p, mixers, "prepared_"+r.ID) > 0 {
			continue
		}
		oneCost := productionAddinCost(r, 1, stock)
		plain := stock["dough"] < oneCost["dough"]
		if r.ID == "sugar" && !plain {
			continue
		}
		amount, cost := bakeryChefRecipeBatch(r, stock, coolingSpace)
		if plain {
			// Only make base dough for an otherwise affordable selected recipe.
			addins := cloneMap(oneCost)
			delete(addins, "dough")
			if !bakeryChefCanAfford(stock, addins) {
				continue
			}
			cost = map[string]int{"baking_sugar": 1, "butter": 1, "flour": 1, "egg": 1}
			amount = 40
		}
		if amount == 0 || !bakeryChefCanAfford(stock, cost) {
			continue
		}
		for _, mixer := range mixers {
			if !emptyProductionMixer(mixer.state) {
				continue
			}
			bakeryChefSpend(stock, cost)
			item, duration := "prepared_"+r.ID, float64(cost["dough"])
			if plain {
				item, duration = "dough", 40
			}
			startProduction(mixer.state, item, amount, duration)
			worker.chefJobKey, worker.chefReady = mixer.key, w.time+bakeryChefHandlingSeconds
			return bakeryChefJobStatus(p, worker)
		}
	}
	for _, mixer := range mixers {
		if bakeryStockCount(mixer.state.Output) > 0 {
			return "Fridge full: make room for mixed dough"
		}
	}
	return bakeryChefJobStatus(p, worker)
}
