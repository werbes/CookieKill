package game

import (
	"errors"
	"math"
	"strings"
)

const productionCapacity = 200

type ProductionJob struct {
	Item        string  `json:"item"`
	Amount      int     `json:"amount"`
	Remaining   float64 `json:"remaining"`
	Total       float64 `json:"total"`
	Destination string  `json:"destination,omitempty"`
}
type StationProduction struct {
	Job    *ProductionJob `json:"job,omitempty"`
	Output map[string]int `json:"output"`
}

func preparedRecipe(item string) (Recipe, bool) {
	if !strings.HasPrefix(item, "prepared_") {
		return Recipe{}, false
	}
	return recipeFor(strings.TrimPrefix(item, "prepared_"))
}
func cloneProduction(src map[string]*StationProduction) map[string]*StationProduction {
	out := make(map[string]*StationProduction, len(src))
	for key, value := range src {
		if value == nil {
			continue
		}
		out[key] = cloneStationProduction(value)
	}
	return out
}
func cloneStationProduction(value *StationProduction) *StationProduction {
	if value == nil {
		return &StationProduction{Output: map[string]int{}}
	}
	copy := *value
	copy.Output = cloneMap(value.Output)
	if value.Job != nil {
		job := *value.Job
		copy.Job = &job
	}
	return &copy
}
func productionState(p *Profile, key string) *StationProduction {
	if p.Production == nil {
		p.Production = map[string]*StationProduction{}
	}
	if p.Production[key] == nil {
		p.Production[key] = &StationProduction{Output: map[string]int{}}
	}
	if p.Production[key].Output == nil {
		p.Production[key].Output = map[string]int{}
	}
	return p.Production[key]
}
func productionHasContents(p *Profile, key string) bool {
	s := p.Production[key]
	return s != nil && (s.Job != nil || bakeryStockCount(s.Output) > 0)
}
func normalizeProduction(p *Profile) {
	if p.Production == nil {
		p.Production = map[string]*StationProduction{}
	}
	for key, s := range p.Production {
		kind := strings.TrimPrefix(key, "bakery_")
		valid := strings.HasPrefix(key, "bakery_") && validChoice(kind, "mixer", "oven", "cooling_table")
		if strings.HasPrefix(key, "home:") && p.HomeIsland != nil {
			for _, o := range p.HomeIsland.Objects {
				if o.ID == strings.TrimPrefix(key, "home:") && validChoice(o.Kind, "mixer", "oven", "table") {
					kind, valid = o.Kind, true
					break
				}
			}
		}
		if !valid || s == nil {
			delete(p.Production, key)
			continue
		}
		kept := map[string]int{}
		remaining := productionCapacity
		items := []string{"dough"}
		for _, r := range recipes {
			items = append(items, "prepared_"+r.ID, r.ID)
		}
		validItem := func(item string) bool {
			_, prepared := preparedRecipe(item)
			_, baked := recipeFor(item)
			if kind == "mixer" {
				return item == "dough" || prepared
			}
			return baked
		}
		for _, item := range items {
			if validItem(item) && s.Output[item] > 0 {
				n := min(s.Output[item], remaining)
				if n > 0 {
					kept[item] = n
					remaining -= n
				}
			}
		}
		s.Output = kept
		if j := s.Job; j != nil {
			if kind == "cooling_table" || kind == "table" || !validItem(j.Item) || j.Amount < 1 || j.Amount > productionCapacity || !finite(j.Remaining) || !finite(j.Total) || j.Total <= 0 || j.Total > 1000 || j.Remaining < 0 || j.Remaining > j.Total {
				s.Job = nil
			}
		}
	}
}

func (w *World) productionStation(p *player, target, kind string) (string, error) {
	if !grounded(p) || p.ridingCamel {
		return "", errors.New("Stand on the ground beside the cooking station first.")
	}
	if p.island == "" {
		if math.Abs(playerY(p)) > .5 {
			return "", errors.New("Go inside the bakery to use this station.")
		}
		if err := w.requireBakeryStation(p, kind, target); err != nil {
			return "", err
		}
		plot, _ := w.bakeryPlot(p)
		for _, f := range p.profile.BakeryFurniture {
			if f.Kind == kind && (p.editingBakeryFurnitureID == f.ID || !w.bakeryFurnitureVisible(p, f, plot)) {
				return "", errors.New("Finish moving this station and approach its open side.")
			}
		}
		return "bakery_" + kind, nil
	}
	if p.island != p.profile.ID || p.profile.HomeIsland == nil {
		return "", errors.New("Use an appliance in your own home.")
	}
	for _, o := range p.profile.HomeIsland.Objects {
		if o.Kind == kind && (target == "" || target == o.ID) && distance(p.x, p.z, o.X, o.Z) <= 5 && math.Abs(playerY(p)-o.Y) <= 3 && w.buildEditor(p.profile.HomeIsland, o.ID) == "" {
			return "home:" + o.ID, nil
		}
	}
	return "", errors.New("Stand beside the matching appliance and finish moving it first.")
}
func heldProductionItem(p *player, item string, amount int) bool {
	i := p.profile.SelectedSlot
	return i >= 0 && i < len(p.profile.Hotbar) && p.profile.Hotbar[i].Item == item && p.profile.Hotbar[i].Count >= amount
}
func takeHeldProductionItem(p *player, item string, amount int) bool {
	if !heldProductionItem(p, item, amount) {
		return false
	}
	stack := &p.profile.Hotbar[p.profile.SelectedSlot]
	stack.Count -= amount
	if stack.Count == 0 {
		*stack = Stack{}
	}
	syncInventory(&p.profile)
	return true
}
func productionAddinCost(recipe Recipe, amount int, available map[string]int) map[string]int {
	cost := map[string]int{}
	for item, n := range recipe.Cost {
		cost[item] = n * amount
	}
	if recipe.ID == "protein_cookie" {
		powder := min(amount, available["protein_powder"])
		cost["protein_powder"], cost["protein_drink"] = powder, amount-powder
	}
	return cost
}
func emptyProductionMixer(s *StationProduction) bool {
	return s.Job == nil && bakeryStockCount(s.Output) == 0
}
func startProduction(s *StationProduction, item string, amount int, seconds float64) {
	s.Job = &ProductionJob{Item: item, Amount: amount, Remaining: seconds, Total: seconds}
}

func (w *World) productionAction(p *player, a Action) error {
	if a.Amount == 0 {
		a.Amount = 1
	}
	if a.Amount < 1 || a.Amount > 100 {
		return errors.New("Choose an amount between 1 and 100.")
	}
	if a.Action == "production_mix" {
		key, err := w.productionStation(p, a.Target, "mixer")
		if err != nil {
			return err
		}
		s := productionState(&p.profile, key)
		if !emptyProductionMixer(s) {
			return errors.New("Wait for this mixer and collect its finished dough before starting another batch.")
		}
		if a.Item == "dough" || a.Item == "" {
			if !p.profile.Bakery {
				return errors.New("Buy your first bakery before mixing new dough. Explore desert chests for dough until then.")
			}
			if a.Amount != 1 {
				return errors.New("Mix one 40-dough batch at a time.")
			}
			cost := map[string]int{"baking_sugar": 1, "butter": 1, "flour": 1, "egg": 1}
			if !afford(p, cost, 1) {
				return errors.New("A batch needs one sugar, one butter, one flour, and one egg.")
			}
			spend(p, cost, 1)
			startProduction(s, "dough", 40, 40)
			return nil
		}
		r, ok := recipeFor(strings.TrimPrefix(a.Item, "prepared_"))
		if !ok || a.Amount > 40 {
			return errors.New("Choose a recipe and mix up to 40 at a time.")
		}
		cost := productionAddinCost(r, a.Amount, p.profile.Inventory)
		if !heldProductionItem(p, "dough", cost["dough"]) {
			return errors.New("Hold enough plain dough in your selected hotbar slot, then put it back in the mixer.")
		}
		if !afford(p, cost, 1) {
			return errors.New("Collect the recipe's add-ins before mixing this dough.")
		}
		takeHeldProductionItem(p, "dough", cost["dough"])
		for item, n := range cost {
			if item != "dough" {
				take(p, item, n)
			}
		}
		startProduction(s, "prepared_"+r.ID, a.Amount, float64(cost["dough"]))
		return nil
	}
	if a.Action == "production_bake" {
		key, err := w.productionStation(p, a.Target, "oven")
		if err != nil {
			return err
		}
		if a.Amount > 40 {
			return errors.New("Bake up to 40 at a time.")
		}
		item := a.Item
		if item == "" {
			item = p.profile.Selected
		}
		recipe := "sugar"
		if item != "dough" {
			r, ok := preparedRecipe(item)
			if !ok {
				return errors.New("Hold plain or prepared dough before loading the oven.")
			}
			recipe = r.ID
		}
		if !heldProductionItem(p, item, a.Amount) {
			return errors.New("Hold enough dough in your selected hotbar slot before loading the oven.")
		}
		s := productionState(&p.profile, key)
		if s.Job != nil {
			return errors.New("This oven is still baking.")
		}
		destinationKey := "bakery_cooling_table"
		if key == "bakery_oven" {
			if _, _, ok := w.bakeryStationPosition(p, "cooling_table"); !ok {
				return errors.New("Place the oven's cooling table before baking.")
			}
		} else {
			destinationKey = w.homeOvenTable(p, key)
			if destinationKey == "" {
				return errors.New("Place a table within six metres of your home oven before baking.")
			}
		}
		destination := productionState(&p.profile, destinationKey)
		if bakeryStockCount(destination.Output)+a.Amount > productionCapacity {
			return errors.New("Collect the finished goods before baking another batch.")
		}
		if !takeHeldProductionItem(p, item, a.Amount) {
			return errors.New("You do not have that dough.")
		}
		startProduction(s, recipe, a.Amount, float64(a.Amount)*5)
		s.Job.Destination = destinationKey
		return nil
	}
	if a.Action == "production_collect" {
		kind := "mixer"
		if p.island == "" {
			if a.Target == bakeryStationID(p.profile.BakeryPlot, "cooling_table") {
				kind = "cooling_table"
			}
		} else if p.profile.HomeIsland != nil {
			for _, o := range p.profile.HomeIsland.Objects {
				if o.ID == a.Target && o.Kind == "table" {
					kind = "table"
				}
			}
		}
		key, err := w.productionStation(p, a.Target, kind)
		if err != nil {
			return err
		}
		s := productionState(&p.profile, key)
		if s.Output[a.Item] < a.Amount {
			return errors.New("That amount is not ready to collect yet.")
		}
		if !gain(p, a.Item, a.Amount) {
			return errors.New("Make room in your inventory before collecting.")
		}
		s.Output[a.Item] -= a.Amount
		if s.Output[a.Item] == 0 {
			delete(s.Output, a.Item)
		}
		return nil
	}
	return errors.New("Unknown cooking action.")
}

func (w *World) tickProduction(dt float64) {
	for _, p := range w.players {
		for key, state := range p.profile.Production {
			if state == nil || state.Job == nil {
				continue
			}
			destination := state
			if strings.HasPrefix(key, "bakery_") {
				kind := strings.TrimPrefix(key, "bakery_")
				if _, _, ok := w.bakeryStationPosition(p, kind); !ok {
					continue
				}
				if kind == "oven" {
					if _, _, ok := w.bakeryStationPosition(p, "cooling_table"); !ok {
						continue
					}
					destination = productionState(&p.profile, "bakery_cooling_table")
				}
			} else {
				present := false
				if p.profile.HomeIsland != nil {
					for _, o := range p.profile.HomeIsland.Objects {
						if "home:"+o.ID == key {
							present = true
							break
						}
					}
				}
				if !present {
					continue
				}
				if state.Job.Destination != "" {
					present = false
					if p.profile.HomeIsland != nil {
						for _, o := range p.profile.HomeIsland.Objects {
							if "home:"+o.ID == state.Job.Destination && o.Kind == "table" {
								present = true
								break
							}
						}
					}
					if !present {
						continue
					}
					destination = productionState(&p.profile, state.Job.Destination)
				}
			}
			job := state.Job
			job.Remaining = max(0, job.Remaining-dt)
			if job.Remaining > .000001 || bakeryStockCount(destination.Output)+job.Amount > productionCapacity {
				continue
			}
			destination.Output[job.Item] += job.Amount
			if _, ok := recipeFor(job.Item); ok {
				p.profile.Discovered[job.Item] = true
				awardHats(&p.profile)
			}
			state.Job = nil
		}
	}
}

func (w *World) homeOvenTable(p *player, ovenKey string) string {
	if p.profile.HomeIsland == nil {
		return ""
	}
	var oven *BuildObject
	for i := range p.profile.HomeIsland.Objects {
		if "home:"+p.profile.HomeIsland.Objects[i].ID == ovenKey {
			oven = &p.profile.HomeIsland.Objects[i]
			break
		}
	}
	if oven == nil {
		return ""
	}
	key, nearest := "", 6.00001
	for _, o := range p.profile.HomeIsland.Objects {
		if o.Kind == "table" && math.Abs(o.Y-oven.Y) <= 1 && w.buildEditor(p.profile.HomeIsland, o.ID) == "" {
			if d := distance(o.X, o.Z, oven.X, oven.Z); d < nearest {
				key, nearest = "home:"+o.ID, d
			}
		}
	}
	return key
}
func productionRemovalBlocked(p *Profile, key string) bool {
	if productionHasContents(p, key) {
		return true
	}
	for _, s := range p.Production {
		if s != nil && s.Job != nil && s.Job.Destination == key {
			return true
		}
	}
	return key == "bakery_cooling_table" && productionHasContents(p, "bakery_oven")
}

func (w *World) productionOwnerProfile(id string) *Profile {
	if p := w.players[id]; p != nil {
		return &p.profile
	}
	if p := w.offline[id]; p != nil {
		return &p.profile
	}
	if profile, ok := w.profiles[id]; ok {
		return &profile
	}
	return nil
}
