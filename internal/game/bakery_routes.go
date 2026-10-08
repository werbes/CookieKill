package game

import "math"

type bakeryWorker struct {
	x, z, yaw  float64
	goal       Point
	path       []Point
	hasRoute   bool
	status     string
	retryAt    float64
	readyAt    float64
	chefJobKey string
	chefReady  float64
	displayID  string
	stationID  string
}

// Furniture edits invalidate paths while preserving waiting orders and stock.
func (w *World) bakeryFurnitureChanged(p *player) {
	if p.bakery != nil {
		p.bakery.routesDirty = true
		p.bakery.fridgeAccessChecked = false
	}
}

func (w *World) bakeryNPCPositions(p *player) []Point {
	points := []Point{}
	if p.bakery == nil {
		return points
	}
	for _, c := range p.bakery.customers {
		points = append(points, Point{c.x, c.z})
	}
	for _, worker := range p.bakery.workers {
		points = append(points, Point{worker.x, worker.z})
	}
	return points
}

func (w *World) bakeryChefStationStatus(p *player) string {
	for _, kind := range []string{"oven", "fridge", "cooling_table"} {
		if _, placed := bakeryWorkerStation(p, "chef", kind); !placed {
			return "Place and finish editing the " + kind + " to resume baking"
		}
	}
	return ""
}

func (w *World) bakeryWorkerReady(p *player, id string) bool {
	if p.bakery == nil || bakeryWorkerOnBreak(&p.profile, id) {
		return false
	}
	worker := p.bakery.workers[id]
	return worker != nil && worker.hasRoute && len(worker.path) == 0 && distance(worker.x, worker.z, worker.goal.X, worker.goal.Z) < .05
}

func (w *World) updateBakeryWorkers(p *player, dt float64) {
	b := p.bakery
	plot, _ := w.bakeryPlot(p)
	if b.workers == nil {
		b.workers = map[string]*bakeryWorker{}
	}
	for _, role := range []string{"chef", "cashier"} {
		second := "chef_2"
		if role == "cashier" {
			second = "cashier_2"
		}
		for _, id := range []string{role, second} {
			if !p.profile.BakeryStaff[id] {
				continue
			}
			worker := b.workers[id]
			fresh := worker == nil
			if fresh {
				worker = &bakeryWorker{x: plot.X, z: plot.Z + 9.5, status: "Starting shift"}
				b.workers[id] = worker
			}
			if bakeryWorkerOnBreak(&p.profile, id) {
				worker.status = "On break"
				continue
			}
			kind := "display"
			if role == "chef" {
				kind = "oven"
			}
			station, placed := bakeryWorkerStation(p, id, kind)
			if role == "chef" && !fresh && !b.routesDirty && worker.stationID != "" {
				if cached, ok := bakeryFurnitureByID(&p.profile, worker.stationID); ok && cached.Kind == kind && cached.ID != p.editingBakeryFurnitureID {
					station, placed = cached, true
				}
			}
			reason := ""
			if role == "chef" {
				reason = w.bakeryChefStationStatus(p)
			} else if !placed {
				reason = "Place and finish editing the display to resume serving"
			}
			if !placed || reason != "" {
				worker.path, worker.hasRoute = nil, false
				worker.status = reason
				continue
			}
			goal, _ := w.bakeryFurnitureServicePoint(p, station, role == "cashier")
			if role == "cashier" {
				worker.displayID = station.ID
			}
			if id == role+"_2" {
				first, _ := bakeryWorkerStation(p, role, kind)
				if first.ID == station.ID {
					a := station.Rotation * math.Pi / 180
					goal.X += .8 * math.Cos(a)
					goal.Z -= .8 * math.Sin(a)
				}
			}
			if fresh || b.routesDirty || goal != worker.goal || !worker.hasRoute && w.time >= worker.retryAt {
				worker.goal = goal
				worker.path, worker.hasRoute = w.bakeryRoute(p, Point{worker.x, worker.z}, goal)
				if role == "chef" && !worker.hasRoute {
					for _, alternate := range p.profile.BakeryFurniture {
						if alternate.Kind != kind || alternate.ID == station.ID || alternate.ID == p.editingBakeryFurnitureID {
							continue
						}
						alternateGoal, _ := w.bakeryFurnitureServicePoint(p, alternate, false)
						path, reachable := w.bakeryRoute(p, Point{worker.x, worker.z}, alternateGoal)
						if reachable {
							station, goal = alternate, alternateGoal
							worker.goal = goal
							worker.path, worker.hasRoute = path, true
							break
						}
					}
				}
				worker.stationID = station.ID
				worker.retryAt = w.time + .5
			}
			if !worker.hasRoute {
				worker.status = "Clear a route to the " + kind
			} else {
				advanceBakeryPath(&worker.x, &worker.z, &worker.yaw, &worker.path, dt*2.4)
				if len(worker.path) > 0 {
					worker.status = "Walking to the " + kind
				} else {
					worker.yaw = math.Atan2(-(plot.X + station.X - worker.x), -(plot.Z + station.Z - worker.z))
					if role == "cashier" {
						worker.status = "Ready to serve"
					} else if worker.chefJobKey == "" {
						worker.status = "Ready to bake"
					}
				}
			}
			if role == "chef" && id == "chef" && !w.bakeryWorkerReady(p, id) {
				b.chefStatus = worker.status
			}
		}
	}
}

// Queue points rotate with the counter. If furniture occupies the next place,
// use another reachable point along its front instead of clipping through it.
func (w *World) bakeryQueuePoint(p *player, slot int) (Point, bool) {
	displays := bakeryDisplays(&p.profile)
	if len(displays) == 0 {
		return Point{}, false
	}
	return w.bakeryDisplayQueuePoint(p, displays[0].ID, slot)
}
func (w *World) bakeryDisplayQueuePoint(p *player, displayID string, slot int) (Point, bool) {
	f, placed := bakeryFurnitureByID(&p.profile, displayID)
	if !placed || f.Kind != "display" || f.ID == p.editingBakeryFurnitureID {
		return Point{}, false
	}
	front, placed := w.bakeryFurnitureServicePoint(p, f, false)
	plot, ok := w.bakeryPlot(p)
	if !placed || !ok {
		return Point{}, false
	}
	rad := f.Rotation * math.Pi / 180
	for _, offset := range []Point{{0, 0}, {-1.1, 0}, {1.1, 0}, {0, 1.1}, {-1.1, 1.1}, {1.1, 1.1}} {
		goal := Point{front.X + offset.X*math.Cos(rad) + offset.Z*math.Sin(rad), front.Z - offset.X*math.Sin(rad) + offset.Z*math.Cos(rad)}
		if _, reachable := w.bakeryRoute(p, Point{plot.X, plot.Z + 9.5}, goal); reachable {
			if slot == 0 {
				return goal, true
			}
			slot--
		}
	}
	return Point{}, false
}
func (w *World) routeBakeryCustomer(p *player, c *bakeryCustomer) {
	plot, ok := w.bakeryPlot(p)
	if !ok {
		c.path, c.hasRoute = nil, false
		return
	}
	goal := Point{plot.X, plot.Z + 9.5}
	if c.Phase != "leaving" {
		if c.DisplayID == "" {
			if displays := bakeryDisplays(&p.profile); len(displays) > 0 {
				c.DisplayID = displays[0].ID
			}
		}
		if queue, reachable := w.bakeryDisplayQueuePoint(p, c.DisplayID, c.slot); reachable {
			goal = queue
		} else {
			c.Phase, c.Amount = "leaving", 0
		}
	}
	if c.Phase == "waiting" && distance(c.x, c.z, goal.X, goal.Z) > .05 {
		c.Phase = "arriving"
	}
	c.goal = goal
	c.path, c.hasRoute = w.bakeryRoute(p, Point{c.x, c.z}, goal)
}

func advanceBakeryPath(x, z, yaw *float64, path *[]Point, budget float64) {
	for budget > 0 && len(*path) > 0 {
		target := (*path)[0]
		d := distance(*x, *z, target.X, target.Z)
		step := min(d, budget)
		if d > 0 {
			*yaw = math.Atan2(-(target.X - *x), -(target.Z - *z))
			*x += (target.X - *x) / d * step
			*z += (target.Z - *z) / d * step
		}
		budget -= step
		if d <= step+.001 {
			*x, *z = target.X, target.Z
			*path = (*path)[1:]
		}
	}
}
