package game

import (
	"container/heap"
	"math"
)

// Use the same capsule, step height, doors and cave transitions as a player.
// Navigation probes copy only the actor and never alter world/player state.
func (w *World) johnStep(p player, x, z float64) (player, bool) {
	if p.island == CamelRaceIsland || johnSanctuary(&p) {
		return p, false
	}
	if p.island != "" {
		if w.islandBlocked(&p, x, z) {
			return p, false
		}
		p.x, p.z = x, z
		p.y = w.islandGround(&p)
		return p, true
	}
	if !w.moveMainland(&p, x, z, groundY(&p)) {
		return p, false
	}
	p.x, p.z = x, z
	if johnSanctuary(&p) {
		return p, false
	}
	return p, true
}

func (w *World) johnSegment(p player, to Point) (player, bool) {
	n := max(1, int(math.Ceil(distance(p.x, p.z, to.X, to.Z)/.2)))
	x, z := p.x, p.z
	for i := 1; i <= n; i++ {
		var ok bool
		p, ok = w.johnStep(p, x+(to.X-x)*float64(i)/float64(n), z+(to.Z-z)*float64(i)/float64(n))
		if !ok {
			return p, false
		}
	}
	return p, true
}

func johnChaseGoal(j, target *player) Point {
	if j.island == "" && j.inCave != target.inCave {
		if j.inCave {
			return Point{120, CaveMouthZ + 2}
		}
		return Point{120, CaveStairTopZ - 1}
	}
	return Point{target.x, target.z}
}

func (w *World) moveJohnPork(j *johnPork, target *player, budget float64) {
	if !w.johnEligible(target) || johnSanctuary(&j.actor) || j.actor.island == CamelRaceIsland {
		j.path = nil
		return
	}
	goal := johnChaseGoal(&j.actor, target)
	d := distance(j.actor.x, j.actor.z, goal.X, goal.Z)
	if d < .7 {
		return
	}
	// Straight pursuit needs no search. Keep long visibility probes bounded.
	probe := goal
	if d > 24 {
		probe = Point{j.actor.x + (goal.X-j.actor.x)*24/d, j.actor.z + (goal.Z-j.actor.z)*24/d}
	}
	if _, clear := w.johnSegment(j.actor, probe); clear {
		j.path = []Point{probe}
	} else if w.time >= j.nextRoute && (len(j.path) == 0 || distance(goal.X, goal.Z, j.goal.X, j.goal.Z) > 2) {
		j.path = w.johnRoute(j.actor, goal)
		j.nextRoute = w.time + .8
		j.goal = goal
	}
	for budget > 1e-8 && len(j.path) > 0 {
		point := j.path[0]
		dx, dz := point.X-j.actor.x, point.Z-j.actor.z
		length := math.Hypot(dx, dz)
		if length < .05 {
			j.path = j.path[1:]
			continue
		}
		step := min(budget, length, .25)
		// Stop just short of the target, without entering their furniture/chair.
		if j.actor.inCave == target.inCave && j.actor.island == target.island && distance(j.actor.x, j.actor.z, target.x, target.z) < .75 {
			return
		}
		p, ok := w.johnSegment(j.actor, Point{j.actor.x + dx/length*step, j.actor.z + dz/length*step})
		if !ok {
			j.path = nil
			return
		}
		p.yaw = math.Atan2(-dx, -dz)
		j.actor = p
		budget -= step
	}
}

type johnCell struct {
	x, z, y int
	cave    bool
}
type johnRouteNode struct {
	x, y, z        float64
	inCave         bool
	cell           johnCell
	parent         *johnRouteNode
	cost, priority float64
}
type johnQueue []*johnRouteNode

func (q johnQueue) Len() int           { return len(q) }
func (q johnQueue) Less(i, j int) bool { return q[i].priority < q[j].priority }
func (q johnQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *johnQueue) Push(n any)        { *q = append(*q, n.(*johnRouteNode)) }
func (q *johnQueue) Pop() any          { old := *q; n := old[len(old)-1]; *q = old[:len(old)-1]; return n }

func (w *World) johnRoute(start player, goal Point) []Point {
	// A local A* search routes around buildings and movable furniture. Replan
	// only on blocked/direct-path changes; large world travel uses successive
	// local sections. Filter scenery once instead of on every grid edge.
	nav := *w
	filter := func(src []Collider) []Collider {
		out := make([]Collider, 0, 48)
		for _, c := range src {
			reach := max(c.Radius, math.Hypot(c.Width, c.Depth)/2) + .5
			if math.Abs(c.X-start.x) <= 33+reach && math.Abs(c.Z-start.z) <= 33+reach {
				out = append(out, c)
			}
		}
		return out
	}
	if start.island == "" {
		nav.colliders = filter(w.colliders)
		nav.propertyColliders = filter(w.propertyColliders)
	}
	heuristic := func(p player) float64 { return distance(p.x, p.z, goal.X, goal.Z) }
	key := func(x, z int, p player) johnCell { return johnCell{x, z, int(math.Round(groundY(&p) * 4)), p.inCave} }
	pose := func(n *johnRouteNode) player { p := start; p.x, p.y, p.z, p.inCave = n.x, n.y, n.z, n.inCave; return p }
	rank := func(n *johnRouteNode) float64 { return distance(n.x, n.z, goal.X, goal.Z) }
	first := &johnRouteNode{x: start.x, y: start.y, z: start.z, inCave: start.inCave, cell: key(0, 0, start), priority: heuristic(start)}
	queue := johnQueue{first}
	best := first
	costs := map[johnCell]float64{first.cell: 0}
	directions := [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}}
	for visits := 0; queue.Len() > 0 && visits < 6000; visits++ {
		n := heap.Pop(&queue).(*johnRouteNode)
		if n.cost > costs[n.cell]+1e-8 {
			continue
		}
		if rank(n) < rank(best) {
			best = n
		}
		if rank(n) < 1.5 {
			if _, ok := nav.johnSegment(pose(n), goal); ok {
				best = &johnRouteNode{x: goal.X, z: goal.Z, parent: n}
				break
			}
		}
		for _, dir := range directions {
			x, z := n.cell.x+dir[0], n.cell.z+dir[1]
			if x < -32 || x > 32 || z < -32 || z > 32 {
				continue
			}
			to := Point{start.x + float64(x), start.z + float64(z)}
			p, ok := nav.johnSegment(pose(n), to)
			if !ok {
				continue
			}
			cell := key(x, z, p)
			cost := n.cost + math.Hypot(float64(dir[0]), float64(dir[1]))
			if old, exists := costs[cell]; exists && old <= cost {
				continue
			}
			costs[cell] = cost
			heap.Push(&queue, &johnRouteNode{x: p.x, y: p.y, z: p.z, inCave: p.inCave, cell: cell, parent: n, cost: cost, priority: cost + heuristic(p)})
		}
	}
	if best == first || rank(best) > heuristic(start)-.5 {
		return nil
	}
	path := []Point{}
	for n := best; n.parent != nil; n = n.parent {
		path = append(path, Point{n.x, n.z})
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}
