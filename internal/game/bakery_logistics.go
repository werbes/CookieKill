package game

import "math"

const bakeryServerTrayCapacity = 20

// Existing stock remains in the saved pantry/display maps. A furnishing is
// relocated only when its larger footprint no longer fits the saved room.
func migrateBakeryLogisticsFurniture(p *Profile) {
	place := func(candidate BakeryFurniture, preferred []Point) bool {
		r, _ := bakeryFurnitureRecipeFor(candidate.Kind)
		for _, point := range preferred {
			candidate.X, candidate.Z = point.X, point.Z
			if !bakeryFurnitureInRoom(candidate, r) {
				continue
			}
			clear := true
			for _, other := range p.BakeryFurniture {
				if bakeryFurnitureOverlaps(candidate, other) {
					clear = false
					break
				}
			}
			layout := append(append([]BakeryFurniture{}, p.BakeryFurniture...), candidate)
			if clear && len(layout) <= 120 && bakeryLayoutReachable(layout) {
				p.BakeryFurniture = layout
				return true
			}
		}
		if p.Recovery == nil {
			p.Recovery = map[string]int{}
		}
		p.Recovery[packedBakeryName(candidate)]++
		return false
	}
	for i, f := range p.BakeryFurniture {
		if f.Kind != "display" {
			continue
		}
		p.BakeryFurniture = append(p.BakeryFurniture[:i:i], p.BakeryFurniture[i+1:]...)
		points := []Point{{f.X, f.Z}}
		for z := 1.; z <= 6; z++ {
			for x := -7.; x <= 7; x++ {
				points = append(points, Point{x, z})
			}
		}
		if place(f, points) {
			last := p.BakeryFurniture[len(p.BakeryFurniture)-1]
			copy(p.BakeryFurniture[i+1:], p.BakeryFurniture[i:len(p.BakeryFurniture)-1])
			p.BakeryFurniture[i] = last
		}
		break
	}
	for _, f := range p.BakeryFurniture {
		if f.Kind == "fridge" {
			return
		}
	}
	points := []Point{{8, -5.5}}
	for z := -6.; z <= -3; z++ {
		for x := -8.; x <= 8; x++ {
			points = append(points, Point{x, z})
		}
	}
	id := "fridge"
	for _, f := range p.BakeryFurniture {
		if f.ID == id {
			id = "starter_fridge"
			break
		}
	}
	place(BakeryFurniture{ID: id, Kind: "fridge"}, points)
}

func (w *World) bakeryFridgeAccessible(p *player) bool {
	b := p.bakery
	if b.fridgeAccessChecked {
		return b.fridgeAccessible
	}
	b.fridgeAccessChecked = true
	fridge, hasFridge := w.bakeryStationServicePoint(p, "fridge", false)
	oven, hasOven := w.bakeryStationServicePoint(p, "oven", false)
	if !hasFridge || !hasOven {
		return false
	}
	_, b.fridgeAccessible = w.bakeryRoute(p, oven, fridge)
	return b.fridgeAccessible
}

func (w *World) bakeryLogisticsPoint(p *player, kind string, back bool) (Point, bool) {
	for _, f := range p.profile.BakeryFurniture {
		if f.Kind == kind {
			if f.ID == p.editingBakeryFurnitureID {
				return Point{}, false
			}
			return w.bakeryFurnitureServicePoint(p, f, back)
		}
	}
	return Point{}, false
}

// Pickup debits the cooling output and credits the persisted tray atomically.
// Delivery reverses that transfer only after physically reaching the counter.
// Edits, a full display, leaving the plot, or logout keep every carried bake.
func (w *World) workBakeryServer(p *player, dt float64) {
	ids := bakeryWorkerIDs(&p.profile, "server")
	if len(ids) == 0 {
		p.bakery.serverStatus = "Not hired"
		return
	}
	for _, id := range ids {
		w.workBakeryServerID(p, id, dt)
	}
	p.bakery.serverStatus = p.bakery.workers[ids[0]].status
}

func (w *World) bakeryServerPoint(p *player, id, kind string, back bool) (BakeryFurniture, Point, bool) {
	f, ok := bakeryWorkerStation(p, id, kind)
	if !ok {
		return BakeryFurniture{}, Point{}, false
	}
	goal, ok := w.bakeryFurnitureServicePoint(p, f, back)
	if id == "server_2" {
		first, _ := bakeryWorkerStation(p, "server", kind)
		if first.ID == f.ID {
			a := f.Rotation * math.Pi / 180
			goal.X -= .8 * math.Cos(a)
			goal.Z += .8 * math.Sin(a)
		}
	}
	return f, goal, ok
}

func (w *World) workBakeryServerID(p *player, id string, dt float64) {
	b := p.bakery
	worker := b.workers[id]
	fresh := worker == nil
	if fresh {
		plot, _ := w.bakeryPlot(p)
		worker = &bakeryWorker{x: plot.X, z: plot.Z + 9.5}
		b.workers[id] = worker
	}
	if bakeryWorkerOnBreak(&p.profile, id) {
		worker.status = "On break"
		return
	}
	tray := bakeryServerTray(&p.profile, id)
	carrying := bakeryStockCount(tray) > 0
	kind := "cooling_table"
	if carrying {
		kind = "display"
	}
	station, goal, placed := w.bakeryServerPoint(p, id, kind, carrying)
	setStatus := func(status string) { worker.status = status }
	if !placed {
		worker.path = nil
		worker.hasRoute = false
		setStatus("Place and finish editing the " + kind)
		return
	}
	if fresh || b.routesDirty || goal != worker.goal || !worker.hasRoute && w.time >= worker.retryAt {
		worker.goal = goal
		worker.path, worker.hasRoute = w.bakeryRoute(p, Point{worker.x, worker.z}, goal)
		worker.retryAt = w.time + .5
	}
	if !worker.hasRoute {
		setStatus("Clear a route to the " + kind)
		return
	}
	advanceBakeryPath(&worker.x, &worker.z, &worker.yaw, &worker.path, dt*2.6)
	if len(worker.path) > 0 {
		if carrying {
			setStatus("Carrying fresh bakes to the display")
		} else {
			setStatus("Walking to the cooling table")
		}
		return
	}
	plot, _ := w.bakeryPlot(p)
	worker.yaw = math.Atan2(-(plot.X + station.X - worker.x), -(plot.Z + station.Z - worker.z))
	capacity := bakeryTotalDisplayCapacity(&p.profile)
	if carrying {
		space := max(0, capacity-bakeryStockCount(p.profile.BakeryDisplay))
		for _, r := range recipes {
			n := min(space, tray[r.ID])
			if n <= 0 {
				continue
			}
			p.profile.BakeryDisplay[r.ID] += n
			tray[r.ID] -= n
			space -= n
			if tray[r.ID] == 0 {
				delete(tray, r.ID)
			}
		}
		if bakeryStockCount(tray) > 0 {
			setStatus("Display full: keeping bakes safe on the tray")
		} else {
			setStatus("Display stocked")
			worker.hasRoute = false
		}
		return
	}
	if bakeryStockCount(p.profile.BakeryDisplay) >= capacity {
		setStatus("Waiting for display space")
		return
	}
	cooling := productionState(&p.profile, "bakery_cooling_table")
	if bakeryStockCount(cooling.Output) == 0 {
		setStatus("Waiting for finished bakes")
		return
	}
	_, display, available := w.bakeryServerPoint(p, id, "display", true)
	if !available {
		setStatus("Place and finish editing the display")
		return
	}
	if w.time < worker.retryAt && !b.routesDirty {
		setStatus("Waiting for a clear route to the display")
		return
	}
	worker.retryAt = w.time + .5
	if _, reachable := w.bakeryRoute(p, goal, display); !reachable {
		setStatus("Clear a route from the kitchen to the display")
		return
	}
	for _, r := range recipes {
		n := min(cooling.Output[r.ID], bakeryServerTrayCapacity, capacity-bakeryStockCount(p.profile.BakeryDisplay))
		if n <= 0 {
			continue
		}
		cooling.Output[r.ID] -= n
		if cooling.Output[r.ID] == 0 {
			delete(cooling.Output, r.ID)
		}
		tray[r.ID] += n
		worker.hasRoute = false
		setStatus("Collecting fresh bakes")
		return
	}
	setStatus("Waiting for finished bakes")
}
