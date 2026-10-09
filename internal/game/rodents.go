package game

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"strings"
)

type RodentSpecies struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Rarity        string  `json:"rarity"`
	Description   string  `json:"description"`
	CaptureRadius float64 `json:"captureRadius"`
}

var rodentCatalog = []RodentSpecies{
	{"wood_mouse", "Wood mouse", "common", "Sticks and fallen trees disturb this quick little forest mouse.", 5},
	{"house_mouse", "House mouse", "common", "Likes flower beds and ponds on home islands.", 5},
	{"rat", "Rat", "common", "Appears only when a nut or berry cookie is dropped in a bakery kitchen; matches the chaser's current speed.", 5},
	{"squirrel", "Squirrel", "uncommon", "Dashes away from trees in fast zigzags.", 5},
	{"cactus_mouse", "Cactus mouse", "uncommon", "Can appear when a cactus is felled. Slow, but must be caught within three metres.", 3},
	{"blind_albino_rat", "Blind albino rat", "rare", "A night forest visitor. Hears sprinting and flees until the runner is fifty metres away.", 5},
	{"flying_squirrel", "Flying squirrel", "rare", "Perches in trees, glides twenty degrees below horizontal, then returns to the closest tree.", 5},
	{"beaver", "Beaver", "legendary", "At night, visits the eastern ocean edge. Nearby jumps make it dive; catch within four metres.", 4},
	{"hamster", "Hamster", "legendary", "One visits Betty during the last five minutes of each day, then burrows at night.", 5},
	{"otter", "Otter", "mythic", "Visits at night every three to five days. A nut cookie first, then cosmic cake, makes this fast swimmer catchable. Cosmic cake first sends it skyward in a rainbow.", 5},
	{"chipmunk", "Chipmunk", "uncommon", "Scampers around fallen wood, pausing to sniff nut cookies.", 5},
	{"mole_rat", "Mole rat", "uncommon", "Briefly emerges from forest soil. Cannot be caught while burrowed.", 5},
}

func rodentSpeciesFor(id string) (RodentSpecies, bool) {
	for _, species := range rodentCatalog {
		if species.ID == id {
			return species, true
		}
	}
	return RodentSpecies{}, false
}

func capturedRodentItem(item string) (string, bool) {
	if !strings.HasPrefix(item, "pet_") {
		return "", false
	}
	id := strings.TrimPrefix(item, "pet_")
	_, ok := rodentSpeciesFor(id)
	return id, ok
}

func rodentPetItem(item string) bool { _, ok := capturedRodentItem(item); return ok }

type RodentView struct {
	ID            string  `json:"id"`
	Species       string  `json:"species"`
	Name          string  `json:"name"`
	Rarity        string  `json:"rarity"`
	X             float64 `json:"x"`
	Y             float64 `json:"y"`
	Z             float64 `json:"z"`
	Yaw           float64 `json:"yaw"`
	Island        string  `json:"island"`
	State         string  `json:"state"`
	CaptureRadius float64 `json:"captureRadius"`
	NutFed        bool    `json:"nutFed,omitempty"`
}

type RodentWorldView struct {
	RainbowUntil float64 `json:"rainbowUntil"`
	OtterNextDay int     `json:"otterNextDay"`
}

type rodent struct {
	RodentView
	homeX, homeZ, born, expires, phase, stateUntil float64
	goalX, goalZ, speed                            float64
	target, treeID                                 string
	nutFed                                         bool
}

type rodentSimulation struct {
	entities                 map[string]*rodent
	rng                      *rand.Rand
	nextSpawn, rainbowUntil  float64
	hamsterDay, otterNextDay int
}

const maxWildRodents = 64

func (w *World) seedRodents() {
	if w.rodents != nil {
		return
	}
	w.rodents = &rodentSimulation{entities: map[string]*rodent{}, rng: rand.New(rand.NewPCG(53171, 80623)), hamsterDay: -1}
	w.rodents.otterNextDay = dayCycleAt(w.time).DayNumber + 3 + w.rodents.rng.IntN(3)
	w.rodents.nextSpawn = w.time + 20
	// A small initial population makes exploration useful before the first chop.
	trees := w.rodentTrees()
	for i := 0; i < min(6, len(trees)); i++ {
		n := trees[w.rodents.rng.IntN(len(trees))]
		w.spawnRodentEncounter("wood_mouse", n.X+1.5, n.Z+1.5)
	}
}

func (w *World) rodentTrees() []*Node {
	trees := []*Node{}
	for _, n := range w.nodes {
		if n.Kind == "wood" && n.Available && n.Island == "" && !n.InCave && area(n.X, n.Z) == "forest" {
			trees = append(trees, n)
		}
	}
	return trees
}

func (w *World) rodentCount(species, island string) int {
	n := 0
	for _, r := range w.rodents.entities {
		if r.Species == species && r.Island == island {
			n++
		}
	}
	return n
}

func (w *World) rodentPositionFree(island string, x, y, z float64, aquatic bool) bool {
	if island == "" {
		if !finite(x) || !finite(y) || !finite(z) || x < MinX+.25 || x > MaxX-.25 || z < MinZ+.25 || z > MaxZ-.25 {
			return false
		}
		return (aquatic || !waterAt(x, z)) && !caveMouth(x, z) && !w.solidBlockedAt(x, y, z, .22)
	}
	p := &player{island: island, x: x, y: y, z: z}
	h, ok := w.currentIsland(p)
	if !ok || math.Hypot(x, z) > islandWalkRadius(h)-.5 {
		return false
	}
	for _, o := range h.Objects {
		r, _ := buildObjectRecipe(o)
		if o.Kind != "pond" && objectBlocks(o, r, x, y, z, .2) {
			return false
		}
	}
	return true
}

func (w *World) spawnRodent(species, island string, x, z float64) *rodent {
	w.seedRodents()
	spec, ok := rodentSpeciesFor(species)
	if !ok || len(w.rodents.entities) >= maxWildRodents {
		return nil
	}
	limit := 8
	if spec.Rarity == "rare" {
		limit = 2
	} else if spec.Rarity == "legendary" || spec.Rarity == "mythic" {
		limit = 1
	} else if island != "" {
		limit = 2
	}
	if w.rodentCount(species, island) >= limit {
		return nil
	}
	aquatic := species == "beaver" || species == "otter"
	found := false
	for i := 0; i < 24; i++ {
		a, radius := float64(i)*2.4, float64(i/6)*.8
		nx, nz := x+math.Cos(a)*radius, z+math.Sin(a)*radius
		if w.rodentPositionFree(island, nx, 0, nz, aquatic) {
			x, z, found = nx, nz, true
			break
		}
	}
	if !found {
		return nil
	}
	w.sequence++
	r := &rodent{RodentView: RodentView{ID: fmt.Sprintf("rodent_%d", w.sequence), Species: species, Name: spec.Name, Rarity: spec.Rarity, X: x, Z: z, Island: island, State: "wandering", CaptureRadius: spec.CaptureRadius}, homeX: x, homeZ: z, born: w.time, expires: w.time + 240, phase: w.rodents.rng.Float64() * 2 * math.Pi}
	if spec.Rarity == "rare" || aquatic {
		r.expires = w.time + 360
	}
	if species == "flying_squirrel" {
		if tree := w.closestRodentTree(x, z, ""); tree != nil {
			r.treeID, r.X, r.Z, r.Y, r.State = tree.ID, tree.X+1, tree.Z, max(6, 7*tree.Scale), "perched"
		}
	}
	w.rodents.entities[r.ID] = r
	return r
}

func (w *World) rodentGather(p *player, n *Node) {
	if p == nil || p.island != "" || p.inCave {
		return
	}
	if (n.Kind == "wood" || n.Kind == "stick") && area(n.X, n.Z) != "forest" {
		return
	}
	w.seedRodents()
	roll := w.rodents.rng.Float64()
	species := ""
	switch n.Kind {
	case "stick":
		if roll < .15 {
			species = "wood_mouse"
		}
	case "wood":
		switch {
		case roll < .12:
			species = "wood_mouse"
		case roll < .16:
			species = "squirrel"
		case roll < .18:
			species = "chipmunk"
		case roll < .195:
			species = "mole_rat"
		case roll < .20:
			species = "flying_squirrel"
		}
	case "cactus":
		if roll < .10 {
			species = "cactus_mouse"
		}
	}
	if species != "" {
		w.spawnRodentEncounter(species, n.X, n.Z)
	}
}

// Gathering and ambient forest encounters share space, so a passive spawn cannot
// crowd an animal disturbed by gathering. Keep its origin clear even after it
// flees; nearby trees should not continually refill the same encounter.
func (w *World) spawnRodentEncounter(species string, x, z float64) *rodent {
	for _, r := range w.rodents.entities {
		if r.Island == "" && r.expires > w.time && (distance(x, z, r.X, r.Z) < 10 || distance(x, z, r.homeX, r.homeZ) < 10) {
			return nil
		}
	}
	return w.spawnRodent(species, "", x, z)
}

func (w *World) rodentDropped(p *player, item string) {
	if p == nil || p.island != "" || p.inCave || item != "nut_cookie" && item != "berry_cookie" {
		return
	}
	w.seedRodents()
	if p.cooldowns["rodent_rat_bait"] > w.time || math.Abs(playerY(p)) > 1 {
		return
	}
	for _, plot := range w.layout.Plots {
		kitchen := func(x, z float64) bool { return math.Abs(x-plot.X) < 9.4 && z > plot.Z-7.4 && z < plot.Z-1.5 }
		if w.plotOwner(plot.ID) == "" || !kitchen(p.x, p.z) {
			continue
		}
		var bait *groundDrop
		for _, drop := range w.drops {
			n := drop.node
			if n.Item == item && n.Island == "" && !n.InCave && math.Abs(n.Y) < .5 && kitchen(n.X, n.Z) && distance(p.x, p.z, n.X, n.Z) < 3 && w.now().Before(drop.expires) {
				if bait == nil || n.ID < bait.node.ID {
					cp := drop
					bait = &cp
				}
			}
		}
		if bait != nil && w.spawnRodent("rat", "", bait.node.X, bait.node.Z) != nil {
			p.cooldowns["rodent_rat_bait"] = w.time + 30
		}
		return
	}
}

func (w *World) spawnAmbientRodents() {
	s, cycle := w.rodents, dayCycleAt(w.time)
	if cycle.Phase == "day" && cycle.Remaining <= 300 && s.hamsterDay != cycle.DayNumber {
		if r := w.spawnRodent("hamster", "", 172, 155.5); r != nil {
			s.hamsterDay, r.expires = cycle.DayNumber, w.time+cycle.Remaining+2
		}
	}
	if cycle.Phase == "night" && cycle.DayNumber >= s.otterNextDay {
		for i := 0; i < 64; i++ {
			x, z := MinX+4+s.rng.Float64()*(MaxX-MinX-8), MinZ+4+s.rng.Float64()*(MaxZ-MinZ-8)
			if r := w.spawnRodent("otter", "", x, z); r != nil {
				s.otterNextDay, r.expires = cycle.DayNumber+3+s.rng.IntN(3), w.time+cycle.Remaining
				break
			}
		}
	}
	if w.time < s.nextSpawn {
		return
	}
	s.nextSpawn = w.time + 20
	trees := w.rodentTrees()
	if len(trees) > 0 {
		n, roll := trees[s.rng.IntN(len(trees))], s.rng.Float64()
		species := ""
		switch {
		case roll < .12:
			species = "wood_mouse"
		case roll < .16:
			species = "squirrel"
		case roll < .18:
			species = "chipmunk"
		case roll < .195:
			species = "mole_rat"
		case roll < .20:
			species = "flying_squirrel"
		case cycle.Phase == "night" && roll < .205:
			species = "blind_albino_rat"
		}
		if species != "" {
			w.spawnRodentEncounter(species, n.X+1.5, n.Z+1.5)
		}
	}
	if cycle.Phase == "night" && s.rng.Float64() < .12 {
		x := -.6 - s.rng.Float64()*4
		w.spawnRodent("beaver", "", x, coastZ(x)+2+s.rng.Float64()*(MaxZ-coastZ(x)-5))
	}
	ids := make([]string, 0, len(w.players))
	for id := range w.players {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := w.players[id]
		if p.disconnected {
			continue
		}
		if p.island != "" {
			if h, ok := w.currentIsland(p); ok && s.rng.Float64() < .35 {
				for _, o := range h.Objects {
					if o.Kind == "flowers" || o.Kind == "pond" {
						w.spawnRodent("house_mouse", p.island, o.X+2, o.Z+1)
						break
					}
				}
			}
		}
	}
}

func rodentPlayer(p *player, island string) bool {
	return p != nil && !p.disconnected && p.health > 0 && p.island == island && !p.inCave && p.coaster == nil
}

func (w *World) nearestRodentPlayer(r *rodent, radius float64, sprintOnly bool) *player {
	var closest *player
	best := radius
	for id, p := range w.players {
		if !rodentPlayer(p, r.Island) || sprintOnly && !p.sprinting {
			continue
		}
		d := distance(r.X, r.Z, p.x, p.z)
		if d < best || d == best && closest != nil && id < closest.profile.ID {
			best, closest = d, p
		}
	}
	return closest
}

func (w *World) closestRodentTree(x, z float64, except string) *Node {
	var best *Node
	dist := math.Inf(1)
	for _, n := range w.nodes {
		if n.Kind != "wood" || !n.Available || n.ID == except || n.Island != "" || n.InCave {
			continue
		}
		if d := distance(x, z, n.X, n.Z); d < dist {
			best, dist = n, d
		}
	}
	return best
}

func (w *World) moveRodent(r *rodent, dx, dz float64) {
	steps := max(1, int(math.Ceil(math.Hypot(dx, dz)/.18)))
	aquatic := r.Species == "beaver" || r.Species == "otter"
	allowed := func(x, z float64) bool {
		return (r.Species != "beaver" || x >= -8 && x <= -.5 && waterAt(x, z)) && w.rodentPositionFree(r.Island, x, r.Y, z, aquatic)
	}
	for i := 0; i < steps; i++ {
		nx, nz := r.X+dx/float64(steps), r.Z+dz/float64(steps)
		if allowed(nx, r.Z) {
			r.X = nx
		}
		if allowed(r.X, nz) {
			r.Z = nz
		}
	}
	if math.Hypot(dx, dz) > 1e-8 {
		r.Yaw = math.Atan2(-dx, -dz)
	}
}

func (w *World) tickRodents(dt float64) {
	w.seedRodents()
	if w.rodents.rainbowUntil <= w.time {
		w.rodents.rainbowUntil = 0
	}
	w.spawnAmbientRodents()
	cycle := dayCycleAt(w.time)
	for id, r := range w.rodents.entities {
		if w.time >= r.expires || cycle.Phase == "day" && (r.Species == "beaver" || r.Species == "blind_albino_rat" || r.Species == "otter" && r.State != "rainbow") {
			delete(w.rodents.entities, id)
			continue
		}
		if r.State == "rainbow" {
			r.Y += 4 * dt
			continue
		}
		if r.Species == "hamster" && cycle.Phase == "night" {
			if r.State != "burrowing" {
				r.State, r.stateUntil = "burrowing", w.time+1.5
			}
			r.Y = max(-.5, r.Y-dt*.4)
			if w.time >= r.stateUntil {
				delete(w.rodents.entities, id)
			}
			continue
		}
		if r.Species == "beaver" && w.diveBeaver(r) {
			continue
		}
		if r.Species == "flying_squirrel" && w.tickFlyingSquirrel(r, dt) {
			continue
		}
		if r.State == "hidden" || r.State == "diving" {
			if w.time < r.stateUntil {
				continue
			}
			r.State, r.Y = "wandering", 0
		}
		if r.Species == "mole_rat" && int((w.time-r.born)/8) > int((w.time-dt-r.born)/8) {
			r.State, r.stateUntil = "hidden", w.time+2.5
			continue
		}
		w.tickGroundRodent(r, dt)
	}
}

func (w *World) diveBeaver(r *rodent) bool {
	if r.State == "diving" && w.time < r.stateUntil {
		return true
	}
	if r.State == "diving" {
		r.State, r.Y = "wandering", 0
	}
	for _, p := range w.players {
		if rodentPlayer(p, r.Island) && !grounded(p) && distance(r.X, r.Z, p.x, p.z) < 9 {
			r.State, r.stateUntil, r.Y = "diving", w.time+8, -.9
			return true
		}
	}
	return false
}

func (w *World) tickFlyingSquirrel(r *rodent, dt float64) bool {
	if r.State == "confused" {
		return false
	}
	if r.State == "perched" {
		p := w.nearestRodentPlayer(r, 12, false)
		treeStanding := false
		for _, n := range w.nodes {
			if n.ID == r.treeID && n.Kind == "wood" && n.Available {
				treeStanding = true
				break
			}
		}
		if treeStanding && (w.time < r.stateUntil || p == nil) {
			return true
		}
		if p != nil {
			r.Yaw = math.Atan2(-(r.X - p.x), -(r.Z - p.z))
		} else {
			r.Yaw = r.phase
		}
		r.State = "gliding"
	}
	if r.State == "gliding" {
		// 110 degrees from upright is 20 degrees below horizontal.
		w.moveRodent(r, -math.Sin(r.Yaw)*8*dt, -math.Cos(r.Yaw)*8*dt)
		r.Y = max(0, r.Y-8*math.Tan(20*math.Pi/180)*dt)
		if r.Y == 0 {
			r.State, r.stateUntil = "wandering", w.time+5
			if tree := w.closestRodentTree(r.X, r.Z, ""); tree != nil {
				r.treeID, r.goalX, r.goalZ = tree.ID, tree.X+1, tree.Z
			}
		}
		return true
	}
	if r.treeID == "" || w.time < r.stateUntil {
		return false
	}
	if distance(r.X, r.Z, r.goalX, r.goalZ) > 1.1 {
		dx, dz := r.goalX-r.X, r.goalZ-r.Z
		d := math.Hypot(dx, dz)
		w.moveRodent(r, dx/d*6*dt, dz/d*6*dt)
		return true
	}
	r.State, r.Y = "climbing", r.Y+3*dt
	if r.Y >= 7 {
		r.State, r.Y, r.stateUntil = "perched", 7, w.time+20
	}
	return true
}

func (w *World) tickGroundRodent(r *rodent, dt float64) {
	radius := 60.0
	if r.Species == "otter" {
		radius = math.Inf(1)
	}
	p := w.nearestRodentPlayer(r, radius, false)
	speed := 6.0
	if p != nil && (r.Species == "rat" || r.Species == "otter") {
		speed = w.playerMovementSpeed(p)
	}
	if r.State == "confused" && w.time < r.stateUntil {
		factor := .6
		if r.Species == "otter" {
			factor = .9
		}
		r.speed = speed * factor
		r.phase += r.speed / 2.7 * dt
		x, z := r.homeX+math.Cos(r.phase)*2.7, r.homeZ+math.Sin(r.phase)*2.7
		w.moveRodent(r, x-r.X, z-r.Z)
		return
	}
	if r.State == "sniffing" && w.time < r.stateUntil {
		r.speed = 0
		return
	}
	if r.State == "confused" || r.State == "sniffing" {
		r.State = "wandering"
	}
	if r.Species == "blind_albino_rat" {
		chaser := w.players[r.target]
		if !rodentPlayer(chaser, r.Island) || distance(r.X, r.Z, chaser.x, chaser.z) >= 50 {
			r.target = ""
		}
		if r.target == "" {
			if runner := w.nearestRodentPlayer(r, 18, true); runner != nil {
				r.target = runner.profile.ID
			}
		}
		p = w.players[r.target]
	} else if p != nil && distance(r.X, r.Z, p.x, p.z) > 15 && r.Species != "otter" {
		p = nil
	}
	if p == nil {
		r.State, r.speed = "wandering", 1.25
		if bait := w.rodentBait(r); bait != nil {
			r.goalX, r.goalZ = bait.node.X, bait.node.Z
		} else {
			r.goalX, r.goalZ = r.homeX+math.Sin(w.time*.18+r.phase)*6, r.homeZ+math.Cos(w.time*.13+r.phase)*6
		}
		dx, dz := r.goalX-r.X, r.goalZ-r.Z
		d := math.Hypot(dx, dz)
		if d > .2 {
			w.moveRodent(r, dx/d*min(d, r.speed*dt), dz/d*min(d, r.speed*dt))
		}
		return
	}
	r.State = "fleeing"
	factor := 1.0
	switch r.Species {
	case "squirrel":
		factor = 1.35
	case "cactus_mouse":
		factor = .55
	case "house_mouse":
		factor = 1
	case "chipmunk":
		factor = 1.1
	case "mole_rat":
		factor = .65
	case "blind_albino_rat":
		factor = 1.15
	case "beaver":
		factor = .8
	case "hamster":
		factor = .75
	case "otter":
		factor = 1.5
		if r.nutFed {
			factor = 1.2
		}
	}
	r.speed = speed * factor
	a := math.Atan2(r.Z-p.z, r.X-p.x)
	if r.Species == "squirrel" || r.Species == "chipmunk" {
		a += .55 * math.Sin(w.time*6+r.phase)
	}
	w.moveRodent(r, math.Cos(a)*r.speed*dt, math.Sin(a)*r.speed*dt)
}

func (w *World) rodentBait(r *rodent) *groundDrop {
	if r.Species != "rat" && r.Species != "chipmunk" {
		return nil
	}
	var best *groundDrop
	distanceTo := 18.0
	for _, drop := range w.drops {
		n := drop.node
		if n.Island != r.Island || n.InCave || n.Item != "nut_cookie" && n.Item != "berry_cookie" || !w.now().Before(drop.expires) {
			continue
		}
		if d := distance(r.X, r.Z, n.X, n.Z); d < distanceTo {
			cp := drop
			best, distanceTo = &cp, d
		}
	}
	return best
}

func rodentHidden(r *rodent) bool {
	return r.State == "hidden" || r.State == "diving" || r.State == "burrowing" || r.State == "rainbow"
}

func (w *World) catchRodent(p *player, id string) error {
	w.seedRodents()
	r := w.rodents.entities[id]
	if r == nil || w.time >= r.expires {
		return errors.New("That animal has moved on.")
	}
	if !rodentPlayer(p, r.Island) || p.fallen {
		return errors.New("Stand nearby in the same place to gently catch this animal.")
	}
	if r.Species == "beaver" {
		w.diveBeaver(r)
	}
	if rodentHidden(r) {
		return errors.New("Wait until the animal comes out of hiding.")
	}
	if r.Species == "otter" {
		if !r.nutFed {
			return errors.New("First hit the otter with a Nut crunch cookie, then a cosmic cake, before catching it.")
		}
		if r.State != "confused" {
			return errors.New("The otter has eaten its nut cookie. Hit it with a cosmic cake, then gently catch it.")
		}
	}
	if p.cooldowns["rodent_catch"] > w.time {
		return errors.New("Give your hands a moment before another catch.")
	}
	if math.Hypot(distance(p.x, p.z, r.X, r.Z), playerY(p)-r.Y) > r.CaptureRadius {
		return errors.New("Move closer to catch this animal.")
	}
	if r.Island == "" {
		if _, hit := w.solidShotHit(p.x, playerY(p)+.65, p.z, r.X, r.Y+.25, r.Z); hit {
			return errors.New("There is an obstacle between you and the animal.")
		}
	} else {
		steps := max(1, int(math.Ceil(distance(p.x, p.z, r.X, r.Z)/.2)))
		for i := 1; i <= steps; i++ {
			f := float64(i) / float64(steps)
			if !w.rodentPositionFree(r.Island, p.x+(r.X-p.x)*f, r.Y, p.z+(r.Z-p.z)*f, false) {
				return errors.New("There is an obstacle between you and the animal.")
			}
		}
	}
	if !gain(p, "pet_"+r.Species, 1) {
		return errors.New("Make one inventory space before catching an animal.")
	}
	delete(w.rodents.entities, id)
	p.cooldowns["rodent_catch"] = w.time + .35
	w.eventActor("Gently caught a "+r.Name+"! It is safe in your inventory; release it on your nut island.", "notification", p.x, p.z, p.profile.ID)
	return nil
}

func throwableRecipe(item string) (Recipe, bool) {
	if r, ok := recipeFor(item); ok {
		return r, true
	}
	if cake, ok := clubCakeFor(item); ok {
		return Recipe{ID: cake.ID, Name: cake.Name}, true
	}
	return Recipe{}, false
}

func (w *World) hitClubCakePlayer(p *player, item string) bool {
	cake, ok := clubCakeFor(item)
	if !ok {
		return false
	}
	if p.buffs == nil {
		p.buffs = map[string]float64{}
	}
	p.buffs["funny_cake"] = w.time + cake.Duration
	return true
}

func (w *World) rodentShotHit(shot *Projectile, nx, ny, nz float64) (float64, string, bool) {
	if w.rodents == nil || shot.Island != "" {
		return 0, "", false
	}
	best, id := 2.0, ""
	for _, r := range w.rodents.entities {
		if r.Island != "" || rodentHidden(r) || w.time >= r.expires {
			continue
		}
		radius := .45
		if r.Species == "otter" || r.Species == "beaver" {
			radius = .7
		}
		if at, hit := segmentHit(shot.X, shot.Y, shot.Z, nx, ny, nz, r.X, r.Y+.3, r.Z, radius); hit && (at < best || at == best && r.ID < id) {
			best, id = at, r.ID
		}
	}
	return best, id, id != ""
}

func (w *World) hitRodent(shot *Projectile, id string) {
	if w.rodents == nil {
		return
	}
	r := w.rodents.entities[id]
	if r == nil || rodentHidden(r) || r.Island != shot.Island {
		return
	}
	if r.Species == "otter" {
		if shot.Item == "nut_cookie" {
			r.nutFed = true
			if r.State != "confused" {
				r.State = "fleeing"
			}
			return
		}
		if shot.Item != "cosmic_cake" {
			return
		}
		if !r.nutFed {
			r.State, r.expires = "rainbow", w.time+60
			w.rodents.rainbowUntil = w.time + 60
			w.rodents.otterNextDay = dayCycleAt(w.time).DayNumber + 3 + w.rodents.rng.IntN(3)
			w.event("The otter floats skyward! A rainbow shimmers across every island for a minute.", "rainbow", r.X, r.Z)
			return
		}
		r.State, r.stateUntil, r.homeX, r.homeZ = "confused", r.expires, r.X-2.7, r.Z
		r.phase = 0
		return
	}
	if _, ok := clubCakeFor(shot.Item); ok {
		r.State, r.stateUntil, r.homeX, r.homeZ = "confused", w.time+20, r.X-2.7, r.Z
		r.phase = 0
	} else if shot.Item == "nut_cookie" || shot.Item == "berry_cookie" {
		r.State, r.stateUntil = "sniffing", w.time+2.5
	}
}

func (w *World) rodentSnapshot(viewer *player, out *Snapshot) {
	w.seedRodents()
	out.Rodents = []RodentView{}
	out.RodentCatalog = append([]RodentSpecies{}, rodentCatalog...)
	out.RodentWorld = RodentWorldView{RainbowUntil: w.rodents.rainbowUntil, OtterNextDay: w.rodents.otterNextDay}
	if viewer == nil || viewer.inCave {
		return
	}
	for _, r := range w.rodents.entities {
		if r.Island == viewer.island && w.time < r.expires {
			view := r.RodentView
			view.NutFed = r.nutFed
			out.Rodents = append(out.Rodents, view)
		}
	}
	sort.Slice(out.Rodents, func(i, j int) bool { return out.Rodents[i].ID < out.Rodents[j].ID })
}
