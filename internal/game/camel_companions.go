package game

import (
	"container/heap"
	"errors"
	"math"
)

type camelCall struct {
	path                    []Point
	goal                    Point
	nextRoute, lastProgress float64
}

func heldCamelTreat(p *player) string {
	i := p.profile.SelectedSlot
	if i < 0 || i >= len(p.profile.Hotbar) {
		return ""
	}
	stack := p.profile.Hotbar[i]
	if _, ok := camelTreatFor(stack.Item); ok && stack.Count > 0 {
		return stack.Item
	}
	return ""
}

func ownCamel(p *player, id string) *Camel {
	for i := range p.profile.Camels {
		if p.profile.Camels[i].ID == id {
			return &p.profile.Camels[i]
		}
	}
	return nil
}

func camelHomePen(p *Profile, id string) (BuildObject, bool) {
	if p.HomeIsland != nil {
		for _, o := range p.HomeIsland.Objects {
			if o.ID == id && o.Kind == "camel_pen" {
				return o, true
			}
		}
	}
	return BuildObject{}, false
}

func normalizeCamelHomes(p *Profile) {
	used := map[string]map[int]bool{}
	for i := range p.Camels {
		c := &p.Camels[i]
		c.Following, c.EatingUntil = false, 0
		if _, ok := camelHomePen(p, c.HomePenID); !ok || c.RentalUntil > 0 {
			c.HomePenID, c.HomePenSlot = "", 0
			continue
		}
		if used[c.HomePenID] == nil {
			used[c.HomePenID] = map[int]bool{}
		}
		occupied := used[c.HomePenID]
		if c.HomePenSlot < 0 || c.HomePenSlot >= camelPenCapacity || occupied[c.HomePenSlot] {
			c.HomePenSlot = 0
			for c.HomePenSlot < camelPenCapacity && occupied[c.HomePenSlot] {
				c.HomePenSlot++
			}
		}
		if c.HomePenSlot >= camelPenCapacity {
			c.HomePenID, c.HomePenSlot = "", 0
			continue
		}
		occupied[c.HomePenSlot] = true
	}
}

func (w *World) camelPenOccupied(owner, penID string) bool {
	p, ok := w.profileByID(owner)
	if !ok {
		return false
	}
	for _, c := range p.Camels {
		if c.HomePenID == penID {
			return true
		}
	}
	return false
}

func (w *World) camelCompanionAction(p *player, a Action) error {
	w.expireCamels(p)
	c := ownCamel(p, a.Target)
	if c == nil {
		return errors.New("Choose one of your own camels.")
	}
	if c.Island != p.island || c.InCave != p.inCave || isCamelRaceIsland(p.island) || inWater(p) {
		return errors.New("Visit your camel's island and cave level before calling or housing it.")
	}
	if a.Action == "camel_call" {
		if p.ridingCamel && p.mountedCamelID == c.ID {
			return errors.New("You are already riding that camel.")
		}
		if p.camelCalls[c.ID] != nil {
			return errors.New("That camel is already coming to you.")
		}
		if w.time < p.cooldowns["camel_call"] {
			return errors.New("Give your camel a moment to hear your call.")
		}
		actor := camelActor(p, c)
		call := &camelCall{goal: Point{p.x, p.z}, lastProgress: w.time, nextRoute: w.time + .6}
		call.path = w.camelPath(actor, call.goal)
		if len(call.path) == 0 && !w.camelNearOwner(actor, p) {
			return errors.New("Your camel cannot reach you. Open a clear path and call again.")
		}
		if p.camelCalls == nil {
			p.camelCalls = map[string]*camelCall{}
		}
		p.camelCalls[c.ID] = call
		p.cooldowns["camel_call"] = w.time + .5
		return nil
	}
	if distance(c.X, c.Z, p.x, p.z) > 8 || math.Abs(c.Y-playerY(p)) > 2 || !grounded(p) {
		return errors.New("Come near your camel on level ground first.")
	}
	if a.Action == "camel_unhouse" {
		c.HomePenID, c.HomePenSlot = "", 0
		return nil
	}
	if a.Action != "camel_house" {
		return errors.New("Choose whether to call or house your camel.")
	}
	if p.island != p.profile.ID || c.RentalUntil > 0 {
		return errors.New("House permanently owned camels on your own home island.")
	}
	pen, ok := camelHomePen(&p.profile, a.Item)
	if !ok || distance(p.x, p.z, pen.X, pen.Z) > 8 || math.Abs(playerY(p)-pen.Y) > 2 || w.buildEditor(p.profile.HomeIsland, pen.ID) != "" {
		return errors.New("Stand near a camel pen on your own home island.")
	}
	slot := -1
	for i := 0; i < camelPenCapacity; i++ {
		used := false
		for _, other := range p.profile.Camels {
			if other.ID != c.ID && other.HomePenID == pen.ID && other.HomePenSlot == i {
				used = true
				break
			}
		}
		if !used {
			slot = i
			break
		}
	}
	if slot < 0 {
		return errors.New("This pen already houses four camels. Build another pen first.")
	}
	point := camelPenSlots(pen)[slot]
	actor := camelActor(p, c)
	path := w.camelPath(actor, point)
	if distance(c.X, c.Z, point.X, point.Z) > .3 && (len(path) == 0 || distance(path[len(path)-1].X, path[len(path)-1].Z, point.X, point.Z) > .3) {
		return errors.New("Open a path from your camel into the pen first.")
	}
	if !w.camelParkingClear(p, point, pen.Y) {
		return errors.New("Clear the camel's space inside this pen first.")
	}
	if p.ridingCamel && p.mountedCamelID == c.ID {
		syncMountedCamel(p)
		p.ridingCamel, p.mountedCamelID = false, ""
		resetMovement(p)
	}
	c.HomePenID, c.HomePenSlot = pen.ID, slot
	c.X, c.Y, c.Z, c.Island, c.InCave, c.Yaw = point.X, pen.Y, point.Z, p.profile.ID, false, pen.Rotation*math.Pi/180
	delete(p.camelCalls, c.ID)
	w.eventActor("Your camel has a home in this pen and will return here when you log out.", "camel", p.x, p.z, p.profile.ID)
	return nil
}

func camelActor(owner *player, c *Camel) player {
	return player{profile: owner.profile, island: c.Island, inCave: c.InCave, x: c.X, y: c.Y, z: c.Z, yaw: c.Yaw}
}

func (w *World) camelNearOwner(actor player, owner *player) bool {
	if distance(actor.x, actor.z, owner.x, owner.z) > 2.2 || math.Abs(groundY(&actor)-playerY(owner)) > 1 {
		return false
	}
	_, clear := w.camelSegment(actor, Point{owner.x, owner.z})
	return clear
}

func (w *World) camelParkingClear(p *player, point Point, y float64) bool {
	probe := *p
	probe.x, probe.y, probe.z, probe.island, probe.inCave = point.X, y, point.Z, p.profile.ID, false
	_, ok := w.camelSegment(probe, point)
	return ok
}

// Housing is an explicit owner assignment. Logout returns only assigned camels;
// all other camels retain the existing parking and mounted-reconnect behavior.
func (w *World) houseOfflineCamels(p *player) {
	if isCamelRaceIsland(p.island) || p.island == CamelTrainingIsland {
		return
	}
	syncMountedCamel(p)
	p.camelCalls = nil
	for i := range p.profile.Camels {
		c := &p.profile.Camels[i]
		delete(p.cooldowns, "camel_eating:"+c.ID)
		pen, ok := camelHomePen(&p.profile, c.HomePenID)
		if !ok || c.HomePenSlot < 0 || c.HomePenSlot >= camelPenCapacity {
			continue
		}
		point := camelPenSlots(pen)[c.HomePenSlot]
		if !w.camelParkingClear(p, point, pen.Y) {
			continue
		}
		if p.ridingCamel && p.mountedCamelID == c.ID {
			p.ridingCamel, p.mountedCamelID = false, ""
			resetMovement(p)
		}
		c.X, c.Y, c.Z, c.Island, c.InCave, c.Yaw = point.X, pen.Y, point.Z, p.profile.ID, false, pen.Rotation*math.Pi/180
	}
}

func (w *World) restoreCamelHomes() {
	for id, profile := range w.profiles {
		p := &player{profile: profile, cooldowns: map[string]float64{}}
		w.houseOfflineCamels(p)
		w.profiles[id] = p.profile
	}
}

func (w *World) decorateCamelCompanions(p *player, out *Snapshot) {
	if p == nil {
		return
	}
	for i := range out.Me.Camels {
		c := &out.Me.Camels[i]
		c.Following = p.camelCalls[c.ID] != nil
		if until := p.cooldowns["camel_eating:"+c.ID]; until > w.time {
			c.EatingUntil = until
		}
	}
}

func (w *World) camelNodePhase(owner string, c Camel) string {
	if p := w.players[owner]; p != nil {
		if p.cooldowns["camel_eating:"+c.ID] > w.time {
			return "eating"
		}
		if p.camelCalls[c.ID] != nil {
			return "following"
		}
	}
	if c.HomePenID != "" && c.Island == owner {
		profile, ok := w.profileByID(owner)
		if pen, exists := camelHomePen(&profile, c.HomePenID); ok && exists && c.HomePenSlot >= 0 && c.HomePenSlot < camelPenCapacity {
			spot := camelPenSlots(pen)[c.HomePenSlot]
			if distance(c.X, c.Z, spot.X, spot.Z) < .7 {
				return "housed"
			}
		}
	}
	return ""
}

func (w *World) tickCamelCompanions(dt float64) {
	for _, p := range w.players {
		if p.disconnected {
			w.houseOfflineCamels(p)
			continue
		}
		for id, call := range p.camelCalls {
			c := ownCamel(p, id)
			if c == nil || c.Island != p.island || c.InCave != p.inCave || isCamelRaceIsland(p.island) || inWater(p) || p.ridingCamel && p.mountedCamelID == id || c.RentalUntil > 0 && c.RentalUntil <= w.now().Unix() {
				delete(p.camelCalls, id)
				continue
			}
			if w.camelNearOwner(camelActor(p, c), p) {
				delete(p.camelCalls, id)
				continue
			}
			if p.cooldowns["camel_eating:"+id] > w.time {
				call.lastProgress = w.time
				continue
			}
			actor := camelActor(p, c)
			if w.time >= call.nextRoute && (len(call.path) == 0 || distance(call.goal.X, call.goal.Z, p.x, p.z) > 2) {
				call.goal = Point{p.x, p.z}
				call.path = w.camelPath(actor, call.goal)
				call.nextRoute = w.time + .7
			}
			camelTraits(c)
			budget := c.Speed * (.9 + .1*c.Happiness/100) * 1.2 * dt
			for budget > 1e-8 && len(call.path) > 0 {
				point := call.path[0]
				dx, dz := point.X-actor.x, point.Z-actor.z
				length := math.Hypot(dx, dz)
				if length < .05 {
					call.path = call.path[1:]
					continue
				}
				if w.camelNearOwner(actor, p) {
					delete(p.camelCalls, id)
					break
				}
				step := min(budget, length, .2)
				next, ok := w.camelSegment(actor, Point{actor.x + dx/length*step, actor.z + dz/length*step})
				if !ok {
					call.path = nil
					break
				}
				next.yaw = math.Atan2(-dx, -dz)
				actor = next
				budget -= step
				call.lastProgress = w.time
			}
			c.X, c.Y, c.Z, c.Yaw = actor.x, groundY(&actor), actor.z, actor.yaw
			if w.time-call.lastProgress > 8 {
				delete(p.camelCalls, id)
				w.eventActor("Your camel cannot reach you. Open a path and call again.", "camel", p.x, p.z, p.profile.ID)
			}
		}
	}
}

func (w *World) camelSegment(p player, to Point) (player, bool) {
	steps := max(1, int(math.Ceil(distance(p.x, p.z, to.X, to.Z)/.2)))
	fromX, fromZ := p.x, p.z
	for i := 1; i <= steps; i++ {
		x, z := fromX+(to.X-fromX)*float64(i)/float64(steps), fromZ+(to.Z-fromZ)*float64(i)/float64(steps)
		if isNutIsland(p.island) {
			if w.nutIslandBlocked(&p, x, z) {
				return p, false
			}
			p.x, p.y, p.z = x, 0, z
		} else if p.island != "" {
			h, ok := w.currentIsland(&p)
			if !ok || math.Hypot(x, z) > islandWalkRadius(h)-.6 {
				return p, false
			}
			for _, o := range h.Objects {
				r, _ := buildObjectRecipe(o)
				if objectBlocks(o, r, x, groundY(&p), z, .65) {
					return p, false
				}
			}
			before := p.y
			p.x, p.z = x, z
			p.y = w.islandGround(&p)
			if math.Abs(p.y-before) > .45 {
				return p, false
			}
		} else {
			if x < MinX+.65 || x > MaxX-.65 || z < MinZ+.65 || z > MaxZ-.65 || waterAt(x, z) {
				return p, false
			}
			cave := p.inCave
			if !w.moveMainland(&p, x, z, groundY(&p)) || p.inCave != cave || w.solidBlockedAt(x, groundY(&p), z, .65) {
				return p, false
			}
			p.x, p.z = x, z
		}
	}
	return p, true
}

type camelNavNode struct {
	point          Point
	actor          player
	parent         *camelNavNode
	x, z           int
	cost, priority float64
}
type camelNavQueue []*camelNavNode

func (q camelNavQueue) Len() int           { return len(q) }
func (q camelNavQueue) Less(i, j int) bool { return q[i].priority < q[j].priority }
func (q camelNavQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *camelNavQueue) Push(v any)        { *q = append(*q, v.(*camelNavNode)) }
func (q *camelNavQueue) Pop() any          { old := *q; v := old[len(old)-1]; *q = old[:len(old)-1]; return v }

func (w *World) camelPath(start player, goal Point) []Point {
	d := distance(start.x, start.z, goal.X, goal.Z)
	probe := goal
	if d > 24 {
		probe = Point{start.x + (goal.X-start.x)*24/d, start.z + (goal.Z-start.z)*24/d}
	}
	if _, ok := w.camelSegment(start, probe); ok {
		return []Point{probe}
	}
	// A bounded local A* routes around doors, rails and scenery. Its movement
	// probes use the same dry-ground collision checks as each actual run step.
	nav := *w
	filter := func(src []Collider) []Collider {
		out := []Collider{}
		for _, c := range src {
			reach := max(c.Radius, math.Hypot(c.Width, c.Depth)/2) + 1
			if math.Abs(c.X-start.x) < 33+reach && math.Abs(c.Z-start.z) < 33+reach {
				out = append(out, c)
			}
		}
		return out
	}
	if start.island == "" {
		nav.colliders = filter(w.colliders)
		nav.propertyColliders = filter(w.propertyColliders)
	}
	first := &camelNavNode{point: Point{start.x, start.z}, actor: start, priority: d}
	queue := camelNavQueue{first}
	best := first
	costs := map[[3]int]float64{{0, 0, int(math.Round(groundY(&start) * 4))}: 0}
	directions := [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	rank := func(n *camelNavNode) float64 { return distance(n.point.X, n.point.Z, goal.X, goal.Z) }
	for visits := 0; queue.Len() > 0 && visits < 4000; visits++ {
		n := heap.Pop(&queue).(*camelNavNode)
		if rank(n) < rank(best) {
			best = n
		}
		if rank(n) < 1.5 {
			if _, ok := nav.camelSegment(n.actor, goal); ok {
				best = &camelNavNode{point: goal, parent: n}
				break
			}
		}
		for _, dir := range directions {
			x, z := n.x+dir[0], n.z+dir[1]
			if x < -32 || x > 32 || z < -32 || z > 32 {
				continue
			}
			point := Point{start.x + float64(x), start.z + float64(z)}
			actor, ok := nav.camelSegment(n.actor, point)
			if !ok {
				continue
			}
			cost := n.cost + math.Hypot(float64(dir[0]), float64(dir[1]))
			key := [3]int{x, z, int(math.Round(groundY(&actor) * 4))}
			if old, exists := costs[key]; exists && old <= cost {
				continue
			}
			costs[key] = cost
			heap.Push(&queue, &camelNavNode{point: point, actor: actor, parent: n, x: x, z: z, cost: cost, priority: cost + distance(point.X, point.Z, goal.X, goal.Z)})
		}
	}
	if best == first || rank(best) > d-.3 {
		return nil
	}
	path := []Point{}
	for n := best; n.parent != nil; n = n.parent {
		path = append(path, n.point)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}
