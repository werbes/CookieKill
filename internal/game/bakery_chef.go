package game

func (w *World) workBakeryChef(p *player) {
	b := p.bakery
	if !p.profile.BakeryStaff["chef"] {
		b.chefStatus = "Not hired"
		return
	}
	if reason := w.bakeryChefStationStatus(p); reason != "" {
		b.chefStatus = reason
		return
	}
	if !w.bakeryWorkerReady(p, "chef") {
		b.chefStatus = "Walking to the oven"
		return
	}
	if !w.bakeryFridgeAccessible(p) {
		b.chefStatus = "Clear a route from the fridge to the kitchen"
		return
	}
	mixers, ovens := []*StationProduction{}, []*StationProduction{}
	for _, f := range p.profile.BakeryFurniture {
		if f.Kind == "mixer" {
			mixers = append(mixers, productionState(&p.profile, bakeryProductionKey(f)))
		}
		if f.Kind == "oven" {
			ovens = append(ovens, productionState(&p.profile, bakeryProductionKey(f)))
		}
	}
	cooling := productionState(&p.profile, "bakery_cooling_table")
	for _, mixer := range mixers {
		for item, n := range mixer.Output {
			n = min(n, bakeryPantryCapacity-bakeryStockCount(p.profile.BakeryPantry))
			if n > 0 {
				p.profile.BakeryPantry[item] += n
				mixer.Output[item] -= n
				if mixer.Output[item] == 0 {
					delete(mixer.Output, item)
				}
			}
		}
	}
	if bakeryStockCount(cooling.Output) >= productionCapacity {
		b.chefStatus = "Cooling table full: collect bakes or hire a server"
		return
	}
	r, ok := recipeFor(p.profile.BakeryChefRecipe)
	if !ok {
		b.chefStatus = "Choose a recipe"
		return
	}
	input := "prepared_" + r.ID
	if r.ID == "sugar" {
		input = "dough"
	}
	for _, oven := range ovens {
		if oven.Job == nil && len(oven.Queue) == 0 && p.profile.BakeryPantry[input] > 0 && bakeryStockCount(cooling.Output) < productionCapacity {
			p.profile.BakeryPantry[input]--
			if p.profile.BakeryPantry[input] == 0 {
				delete(p.profile.BakeryPantry, input)
			}
			startProduction(oven, r.ID, 1, 5)
			oven.Job.Destination = "bakery_cooling_table"
		}
	}
	b.chefStatus = "Needs ingredients"
	for _, mixer := range mixers {
		if !emptyProductionMixer(mixer) {
			continue
		}
		cost := productionAddinCost(r, 1, p.profile.BakeryPantry)
		plain := p.profile.BakeryPantry["dough"] < cost["dough"]
		if r.ID == "sugar" && !plain {
			continue
		}
		if plain {
			cost = map[string]int{"baking_sugar": 1, "butter": 1, "flour": 1, "egg": 1}
		}
		afford := true
		for item, n := range cost {
			if p.profile.BakeryPantry[item] < n {
				afford = false
				break
			}
		}
		if !afford {
			continue
		}
		for item, n := range cost {
			p.profile.BakeryPantry[item] -= n
			if p.profile.BakeryPantry[item] == 0 {
				delete(p.profile.BakeryPantry, item)
			}
		}
		if plain {
			startProduction(mixer, "dough", 40, 40)
		} else {
			startProduction(mixer, "prepared_"+r.ID, 1, float64(cost["dough"]))
		}
	}
	for _, mixer := range mixers {
		if mixer.Job != nil || len(mixer.Queue) > 0 {
			b.chefStatus = "Mixing dough"
		} else if len(mixer.Output) > 0 {
			b.chefStatus = "Fridge full"
		}
	}
	for _, oven := range ovens {
		if oven.Job != nil || len(oven.Queue) > 0 {
			b.chefStatus = "Baking " + r.Name
			break
		}
	}
}

