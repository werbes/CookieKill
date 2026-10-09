package game

import "math"

const DayDuration = 12 * 60.0
const NightDuration = 6 * 60.0

type DayCycle struct {
	DayNumber     int     `json:"dayNumber"`
	Phase         string  `json:"phase"`
	Elapsed       float64 `json:"elapsed"`
	Remaining     float64 `json:"remaining"`
	DayDuration   float64 `json:"dayDuration"`
	NightDuration float64 `json:"nightDuration"`
}

func dayCycleAt(seconds float64) DayCycle {
	elapsed := math.Mod(max(0, seconds)+1e-8, DayDuration+NightDuration)
	phase, duration := "day", DayDuration
	if elapsed >= DayDuration {
		phase, elapsed, duration = "night", elapsed-DayDuration, NightDuration
	}
	elapsed = max(0, elapsed-1e-8)
	return DayCycle{DayNumber: int(math.Floor((max(0, seconds) + 1e-8) / (DayDuration + NightDuration))), Phase: phase, Elapsed: elapsed, Remaining: duration - elapsed, DayDuration: DayDuration, NightDuration: NightDuration}
}

type JohnPorkView struct {
	ID       string  `json:"id"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Z        float64 `json:"z"`
	Yaw      float64 `json:"yaw"`
	Target   string  `json:"target"`
	Island   string  `json:"island"`
	InCave   bool    `json:"inCave"`
	Eating   bool    `json:"eating"`
	Sleeping bool    `json:"sleeping"`
}

type johnPork struct {
	actor                             player
	target                            string
	lastTarget                        Point
	lastIsland                        string
	travelUntil, nextRoute, nextSpawn float64
	path                              []Point
	goal                              Point
	eatProgress                       float64
	eating                            bool
}

// SetConnected excludes disconnected sessions immediately, including the
// server's reconnect grace period before their player is moved offline.
func (w *World) SetConnected(id string, connected bool) {
	if p := w.players[id]; p != nil {
		p.disconnected = !connected
	}
}

// Both the player and pursuer use this exact speed, including current movement
// mode and buffs. An idle target still has its normal walking speed.
func (w *World) playerMovementSpeed(p *player) float64 {
	speed := 6 * (1 + float64(p.profile.Levels["legs"])*.04)
	if p.ridingCamel {
		if camelSpeed := camelBaseSpeed(p); camelSpeed > 0 {
			speed = camelSpeed
		} else {
			speed *= 1.8
		}
	}
	if inWater(p) {
		speed = 4 * (1 + float64(p.profile.Levels["swim"])*.06)
		if w.active(p, "swim") {
			speed *= 1.6
		}
	}
	if p.sprinting {
		speed *= 1.65
	}
	if w.active(p, "speed") {
		speed *= 1.4
	}
	if w.active(p, "slowed") {
		speed *= .55
	}
	return speed
}

func bakedFoodCount(p *player) int {
	count := 0
	for _, r := range recipes {
		count += p.profile.Inventory[r.ID]
	}
	return count
}

func johnSanctuary(p *player) bool {
	return p != nil && p.island == "" && p.inCave && clubFootprint(p.x, p.z)
}

func (w *World) johnEligible(p *player) bool {
	return p != nil && p.island == "" && p.coaster == nil && !p.disconnected && p.health > 0 && !johnSanctuary(p) && !w.active(p, "spawn_shield") && !w.johnIgnored[p.profile.ID] && bakedFoodCount(p) > 0
}

// Home and activity portals, and the coaster, leave John behind. Escaping does
// not count as having lost the player's cookies for the night.
func (w *World) leaveJohnPorkBehind(p *player) {
	if p != nil && (p.island != "" || p.coaster != nil) && w.johnPork != nil && (w.johnPork.target == p.profile.ID || w.johnPork.actor.island != "") {
		w.johnPork = nil
	}
}

func (w *World) richestJohnTarget() *player {
	var best *player
	for _, p := range w.players {
		if w.johnEligible(p) && (best == nil || p.profile.Coins > best.profile.Coins || p.profile.Coins == best.profile.Coins && p.profile.ID < best.profile.ID) {
			best = p
		}
	}
	return best
}

func (w *World) tickJohnPork(dt float64) {
	cycle := dayCycleAt(w.time)
	if cycle.Phase != "night" {
		w.johnPork = nil
		w.johnIgnored = nil
		return
	}
	if w.johnIgnored == nil {
		w.johnIgnored = map[string]bool{}
	}
	j := w.johnPork
	if j != nil && (!w.johnEligible(w.players[j.target]) || johnSanctuary(&j.actor) || j.actor.island != "") {
		if p := w.players[j.target]; p != nil && p.island == "" && p.coaster == nil && !johnSanctuary(p) && bakedFoodCount(p) == 0 {
			w.johnIgnored[j.target] = true
		}
		w.johnPork = nil
		j = nil
	}
	if j == nil {
		p := w.richestJohnTarget()
		if p == nil {
			return
		}
		j = &johnPork{target: p.profile.ID, lastTarget: Point{p.x, p.z}, lastIsland: p.island}
		w.johnPork = j
		w.eventActor("John Pork is following "+p.profile.CallName+" tonight.", "john_pork", p.x, p.z, p.profile.ID)
	}
	target := w.players[j.target]
	// A mainland portal/respawn is the only time John relocates.
	// Walking into a cave remains a physical journey through its mouth/stairs.
	if target.island != j.lastIsland || distance(target.x, target.z, j.lastTarget.X, j.lastTarget.Z) > 20 {
		j.travelUntil = w.time + 2
		j.actor.profile.ID = ""
		j.path = nil
		j.eatProgress = 0
		j.eating = false
	}
	j.lastTarget = Point{target.x, target.z}
	j.lastIsland = target.island
	if w.time < j.travelUntil {
		return
	}
	if j.actor.profile.ID == "" {
		if w.time < j.nextSpawn {
			return
		}
		j.nextSpawn = w.time + 1
		if !w.spawnJohnNear(j, target) {
			return
		}
	}
	j.eating = false
	if w.johnCanEat(&j.actor, target) {
		j.eating = true
		j.eatProgress += min(dt, cycle.Elapsed) * 8
		amount := int(j.eatProgress + 1e-8)
		if amount > 0 {
			j.eatProgress -= float64(amount)
			for _, r := range recipes {
				n := min(amount, target.profile.Inventory[r.ID])
				if n > 0 {
					take(target, r.ID, n)
					amount -= n
				}
				if amount == 0 {
					break
				}
			}
			if bakedFoodCount(target) == 0 {
				w.johnIgnored[target.profile.ID] = true
				w.johnPork = nil
			}
		}
		return
	}
	j.eatProgress = 0
	w.moveJohnPork(j, target, w.playerMovementSpeed(target)*.9*dt)
}

func (w *World) johnCanEat(j, target *player) bool {
	if !w.johnEligible(target) || j.island != "" || johnSanctuary(j) || j.inCave != target.inCave || math.Hypot(distance(j.x, j.z, target.x, target.z), groundY(j)-playerY(target)) > 1+1e-8 {
		return false
	}
	_, hit := w.solidShotHit(j.x, groundY(j)+1, j.z, target.x, playerY(target)+1, target.z)
	return !hit
}

func (w *World) spawnJohnNear(j *johnPork, target *player) bool {
	if !w.johnEligible(target) {
		return false
	}
	radii := []float64{70, 50, 30, 18, 12, 9, 6, 4, 2.5}
	if target.inCave {
		radii = []float64{12, 9, 6, 4, 2.5}
	}
	for _, radius := range radii {
		angles := 16
		if radius > 18 {
			angles = 8
		}
		for i := 0; i < angles; i++ {
			a := float64(i) * 2 * math.Pi / float64(angles)
			candidate := player{profile: Profile{ID: "john_pork"}, island: target.island, inCave: target.inCave, y: groundY(target), x: target.x + math.Sin(a)*radius, z: target.z + math.Cos(a)*radius}
			stepped, ok := w.johnStep(candidate, candidate.x, candidate.z)
			if !ok || stepped.inCave != target.inCave {
				continue
			}
			candidate = stepped
			if end, ok := w.johnSegment(candidate, Point{target.x, target.z}); !ok || end.inCave != target.inCave {
				continue
			}
			j.actor = candidate
			j.path = nil
			j.nextRoute = 0
			return true
		}
	}
	return false
}

func (w *World) johnPorkSnapshot(viewer *player) *JohnPorkView {
	if viewer != nil && viewer.island != "" {
		return nil
	}
	if dayCycleAt(w.time).Phase != "night" {
		if viewer != nil && (viewer.island != "" || viewer.inCave) {
			return nil
		}
		return &JohnPorkView{ID: "john_pork", X: JohnBedX, Y: JohnBedY, Z: JohnBedZ, Yaw: JohnBedYaw, Sleeping: true}
	}
	j := w.johnPork
	if j == nil || j.actor.island != "" || j.actor.profile.ID == "" || !w.johnEligible(w.players[j.target]) || w.time < j.travelUntil || viewer != nil && (viewer.island != j.actor.island || viewer.inCave != j.actor.inCave) {
		return nil
	}
	p := &j.actor
	return &JohnPorkView{ID: "john_pork", X: p.x, Y: groundY(p), Z: p.z, Yaw: p.yaw, Target: j.target, Island: p.island, InCave: p.inCave, Eating: j.eating}
}
