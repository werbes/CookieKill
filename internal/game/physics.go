package game

import (
	"fmt"
	"math"
	"math/rand/v2"
)

const (
	jumpSpeed   = 6.6
	jumpGravity = 18.0
)

func grounded(p *player) bool   { return p.jumpOffset == 0 && p.jumpVelocity == 0 }
func playerY(p *player) float64 { return groundY(p) + p.jumpOffset }

// Teleports, respawns and sitting discard pending movement from the old position.
func resetMovement(p *player) {
	p.input = Input{}
	p.jumpOffset, p.jumpVelocity = 0, 0
	p.jumpQueued, p.sprinting = false, false
	// Preserve physical key holds: arriving at a destination is not a new Space
	// press or a Shift release, and teleporting must not bypass exhaustion.
	delete(p.cooldowns, "jump")
}

// Tick accepts elapsed seconds; catch-up is bounded to one second and split
// into small steps so a delayed network loop cannot tunnel through targets.
func (w *World) Tick(dt float64) {
	if !finite(dt) || dt <= 0 {
		return
	}
	dt = min(dt, 1)
	for dt > 0 {
		step := min(dt, .05)
		w.step(step)
		dt -= step
	}
}

func (w *World) step(dt float64) {
	w.time += dt
	for _, p := range w.players {
		if w.time-p.cooldowns["last_input"] > .5 {
			p.input.X = 0
			p.input.Z = 0
			p.input.Sprint = false
			p.input.Jump = false
			p.jumpQueued = false
		}
		previousGround := groundY(p)
		previousY := playerY(p)
		if p.jumpQueued && grounded(p) && !inWater(p) && !p.ridingCamel && p.cooldowns["jump"] <= w.time {
			p.jumpVelocity = jumpSpeed
			p.posture = ""
		}
		p.jumpQueued = false
		moving := math.Hypot(p.input.X, p.input.Z) > .01
		if moving {
			p.posture = ""
		}
		if p.ridingCamel && (!p.profile.CamelOwned && p.profile.CamelRentalUntil <= w.now().Unix() || inWater(p)) {
			p.ridingCamel = false
		}
		speed := 6 * (1 + float64(p.profile.Levels["legs"])*.04)
		if p.ridingCamel {
			speed *= 1.8
		}
		if inWater(p) {
			speed = 4 * (1 + float64(p.profile.Levels["swim"])*.06)
			if w.active(p, "swim") {
				speed *= 1.6
			}
		}
		maxStamina := 100 + float64(p.profile.Levels["stamina"])*10
		if p.stamina <= 0 {
			p.sprintExhausted = true
		}
		if p.sprintExhausted && !p.sprintHeld && p.stamina >= maxStamina*.2 {
			p.sprintExhausted = false
		}
		p.sprinting = moving && !inWater(p) && p.input.Sprint && !p.sprintExhausted
		if p.sprinting {
			p.stamina = max(0, p.stamina-dt*18)
			if p.stamina == 0 {
				p.sprinting = false
				p.sprintExhausted = true
			} else {
				speed *= 1.65
			}
		} else {
			p.stamina = min(maxStamina, p.stamina+dt*14)
		}
		if w.active(p, "speed") {
			speed *= 1.4
		}
		if w.active(p, "slowed") {
			speed *= .55
		}
		nx, nz := max(MinX+1, min(MaxX-1, p.x+p.input.X*speed*dt)), max(MinZ+1, min(MaxZ-1, p.z+p.input.Z*speed*dt))
		// Fast sprint buffs must still collide with thin cabin walls.
		steps := max(1, int(math.Ceil(math.Max(math.Abs(nx-p.x), math.Abs(nz-p.z))/.25)))
		dx, dz := (nx-p.x)/float64(steps), (nz-p.z)/float64(steps)
		for i := 0; i < steps; i++ {
			if p.island != "" && !w.islandBlocked(p, p.x+dx, p.z) || p.island == "" && !w.solidBlocked(p.x+dx, p.z, .42) && !w.homeBlocked(p, p.x+dx, p.z) {
				p.x += dx
			}
			if p.island != "" && !w.islandBlocked(p, p.x, p.z+dz) || p.island == "" && !w.solidBlocked(p.x, p.z+dz, .42) && !w.homeBlocked(p, p.x, p.z+dz) {
				p.z += dz
			}
		}
		if p.island != "" {
			p.y = w.islandGround(p)
		}
		w.moveJump(p, dt, previousGround, previousY)
		for key, until := range p.buffs {
			if until <= w.time {
				delete(p.buffs, key)
			}
		}
	}
	nodes := w.nodes[:0]
	for _, n := range w.nodes {
		if n.ExpiresAt > 0 && n.ExpiresAt <= w.time {
			continue
		}
		nodes = append(nodes, n)
		if !n.Available && n.readyAt <= w.time {
			if n.Kind == "wood" && w.treeRegrowthBlocked(n) {
				n.readyAt = w.time + 1
				continue
			}
			n.Available = true
			if n.Kind == "wood" {
				n.ChopRemaining = n.ChopTotal
			}
		}
	}
	w.nodes = nodes
	w.moveAnimals(dt)
	w.moveProjectiles(dt)
}

func (w *World) moveJump(p *player, dt, previousGround, previousY float64) {
	if inWater(p) || p.ridingCamel {
		p.jumpOffset, p.jumpVelocity = 0, 0
		return
	}
	if grounded(p) {
		return
	}
	// Subtract changes in the supporting surface so the jump follows a world-
	// space arc while walking up or down the cave ramp and island stairs.
	p.jumpOffset += p.jumpVelocity*dt - jumpGravity*dt*dt/2 - (groundY(p) - previousGround)
	p.jumpVelocity -= jumpGravity * dt
	if p.jumpOffset > 0 && playerY(p) > previousY {
		if ceiling, hit := w.jumpCeiling(p, previousY, playerY(p)); hit {
			p.jumpOffset = max(0, ceiling-1.7-groundY(p))
			p.jumpVelocity = min(0, p.jumpVelocity)
		}
	}
	if p.jumpOffset <= 0 {
		p.jumpOffset, p.jumpVelocity = 0, 0
		p.cooldowns["jump"] = w.time + .12
	}
}

// Walking collision still blocks thin walls during a jump. This additional
// vertical sweep keeps the player's head beneath roofs and upper floors.
func (w *World) jumpCeiling(p *player, fromY, toY float64) (float64, bool) {
	ceiling, hit := math.Inf(1), false
	consider := func(y float64, overlaps bool) {
		if overlaps && fromY+1.7 <= y+.001 && toY+1.7 >= y && y < ceiling {
			ceiling, hit = y, true
		}
	}
	if p.island != "" {
		if island, ok := w.currentIsland(p); ok {
			for _, o := range island.Objects {
				if o.Kind == "door" && o.Open {
					continue
				}
				r, _ := buildRecipeFor(o.Kind)
				lx, lz := objectLocal(o, p.x, p.z)
				consider(o.Y, circleRect(lx, lz, .42, 0, 0, r.Width, r.Depth))
			}
		}
		return ceiling, hit
	}
	for _, colliders := range [][]Collider{w.colliders, w.propertyColliders} {
		for _, c := range colliders {
			if c.resource != nil && !c.resource.Available {
				continue
			}
			overlaps := circleRect(p.x, p.z, .42, c.X, c.Z, c.Width, c.Depth)
			if c.Radius > 0 {
				overlaps = distance(p.x, p.z, c.X, c.Z) <= c.Radius+.42
			}
			consider(c.Y, overlaps)
		}
	}
	return ceiling, hit
}

func (w *World) treeRegrowthBlocked(n *Node) bool {
	radius := .45 * max(.65, n.Scale)
	for _, p := range w.players {
		if p.island == "" && distance(p.x, p.z, n.X, n.Z) < radius+.5 {
			return true
		}
	}
	for _, fire := range w.nodes {
		if fire.Kind == "fire" && fire.Island == "" && distance(fire.X, fire.Z, n.X, n.Z) < radius+.7 {
			return true
		}
	}
	return false
}

// segmentHit finds the closest point along a projectile's swept segment. The
// server uses these distances rather than trusting a client hit notification.
func segmentHit(x, y, z, nx, ny, nz, cx, cy, cz, radius float64) (float64, bool) {
	dx, dy, dz := nx-x, ny-y, nz-z
	denom := dx*dx + dy*dy + dz*dz
	t := 0.0
	if denom > 0 {
		t = ((cx-x)*dx + (cy-y)*dy + (cz-z)*dz) / denom
	}
	t = max(0, min(1, t))
	ax, ay, az := x+dx*t-cx, y+dy*t-cy, z+dz*t-cz
	return t, ax*ax+ay*ay+az*az <= radius*radius
}

// Separate volumes classify hits on the server. Rotating hand and foot offsets
// with the avatar makes side-on shots obey the same visible anatomy.
func playerHit(s *Projectile, nx, ny, nz float64, p *player) (float64, int, bool) {
	parts := []struct {
		x, y, z, r float64
		coins      int
	}{{0, 1.68, 0, .24, 5}, {0, 1.04, 0, .38, 1}, {0, .55, 0, .29, 1}, {-.49, 1.05, 0, .18, 3}, {.49, 1.05, 0, .18, 3}, {-.19, .15, -.08, .19, 3}, {.19, .15, -.08, .19, 3}}
	nearest, reward, hit := 2.0, 0, false
	for _, v := range parts {
		cx := p.x + v.x*math.Cos(p.yaw) + v.z*math.Sin(p.yaw)
		cz := p.z - v.x*math.Sin(p.yaw) + v.z*math.Cos(p.yaw)
		t, ok := segmentHit(s.X, s.Y, s.Z, nx, ny, nz, cx, playerY(p)+v.y, cz, v.r)
		if ok && t < nearest {
			nearest, reward, hit = t, v.coins, true
		}
	}
	return nearest, reward, hit
}

func (w *World) moveProjectiles(dt float64) {
	surviving := w.projectiles[:0]
	for _, shot := range w.projectiles {
		if w.time-shot.born > 5 {
			continue
		}
		nx, ny, nz := shot.X+shot.VX*dt, shot.Y+shot.VY*dt, shot.Z+shot.VZ*dt
		nearest := 2.0
		var hitPlayer *player
		var hitAnimal *Animal
		hitDummy := false
		hitCoins := 0
		hitHome := ""
		blocked := false
		if t, hit := w.solidShotHit(shot.X, shot.Y, shot.Z, nx, ny, nz); hit {
			nearest = t
			blocked = true
		}
		w.eachHome(func(h Home) bool {
			radius := 2.8
			if h.Safe {
				radius = 8
			}
			if t, hit := segmentHit(shot.X, shot.Y, shot.Z, nx, ny, nz, h.X, 1.2, h.Z, radius); hit && t < nearest {
				nearest = t
				hitHome = h.Owner
				blocked = true
			}
			return false
		})
		for id, p := range w.players {
			if id == shot.Owner || p.island != "" {
				continue
			}
			t, coins, hit := playerHit(shot, nx, ny, nz, p)
			if hit && t < nearest {
				nearest = t
				hitPlayer = p
				hitAnimal = nil
				hitDummy = false
				hitCoins = coins
				blocked = false
				hitHome = ""
			}
		}
		for _, a := range w.animals {
			if a.Health <= 0 {
				continue
			}
			radius := 1.15
			if a.Species == "whale" {
				radius = 2.3
			}
			if a.Species == "fish" {
				radius = .65
			}
			if a.Scale > 0 {
				radius *= a.Scale
			}
			t, hit := segmentHit(shot.X, shot.Y, shot.Z, nx, ny, nz, a.X, a.Y+.1, a.Z, radius)
			if hit && t < nearest {
				nearest = t
				hitAnimal = a
				hitPlayer = nil
				hitDummy = false
				blocked = false
				hitHome = ""
			}
		}
		t, hit := segmentHit(shot.X, shot.Y, shot.Z, nx, ny, nz, -35, 1, -47, .9)
		if hit && t < nearest {
			nearest = t
			blocked = false
			hitHome = ""
			hitPlayer = nil
			hitAnimal = nil
			hitDummy = true
		}
		owner := w.players[shot.Owner]
		if blocked {
			if hitHome != "" && hitHome != shot.Owner {
				w.damageHome(hitHome, shot.damage)
			}
			continue
		}
		if hitPlayer != nil {
			if !w.active(hitPlayer, "spawn_shield") && !w.insideSafe(hitPlayer.x, hitPlayer.z) {
				damage := shot.damage
				if shot.super && owner != nil && w.active(owner, "superstrength") {
					damage = 1000
				}
				if !w.damagePlayer(owner, hitPlayer, damage, "a cookie") {
					if shot.Item == "berry_cookie" {
						hitPlayer.buffs["slowed"] = w.time + 5
					}
					if shot.Item == "nut_cookie" {
						hitPlayer.buffs["weakened"] = w.time + 6
					}
				}
				// Award after damage/death: the wealth comparison uses balances
				// immediately before this hit, including on a killing headshot.
				if owner != nil {
					addCoins(owner, hitCoins)
					if len(w.events) > 0 {
						e := &w.events[len(w.events)-1]
						e.HitZone = "body"
						if hitCoins == 5 {
							e.HitZone = "head"
						}
						if hitCoins == 3 {
							e.HitZone = "hand/foot"
						}
						e.Reward = hitCoins
					}
				}
			}
			continue
		}
		if hitAnimal != nil {
			w.damageAnimal(owner, hitAnimal, shot.damage)
			continue
		}
		if hitDummy {
			w.eventActor(fmt.Sprintf("Practice hit! %.0f damage.", shot.damage), "hit", -35, -47, shot.Owner)
			continue
		}
		floor := caveGroundY(nx, nz)
		if waterAt(nx, nz) {
			floor = -1
		}
		if ny < floor || nx < MinX || nx > MaxX || nz < MinZ || nz > MaxZ {
			continue
		}
		shot.X = nx
		shot.Y = ny
		shot.Z = nz
		shot.VY -= dt * 5
		surviving = append(surviving, shot)
	}
	w.projectiles = surviving
}

func (w *World) damagePlayer(killer, victim *player, damage float64, cause string) bool {
	if victim.island != "" || w.active(victim, "spawn_shield") || w.insideSafe(victim.x, victim.z) {
		return false
	}
	victim.health -= damage
	if victim.health > 0 {
		actor := ""
		if killer != nil {
			actor = killer.profile.ID
		}
		w.eventActor(fmt.Sprintf("%s took %.0f damage.", victim.profile.Name, damage), "hit", victim.x, victim.z, actor)
		return false
	}
	victim.profile.Deaths++
	if killer != nil && killer.profile.ID != victim.profile.ID {
		coins := victim.profile.Coins * 5 / 100
		if killer.profile.Coins < victim.profile.Coins {
			coins = (victim.profile.Coins + 9) / 10
		}
		victim.profile.Coins -= coins
		addCoins(killer, coins)
		occupied := []int{}
		for i, s := range victim.profile.BagSlots {
			if s.Count > 0 {
				occupied = append(occupied, i)
			}
		}
		rand.Shuffle(len(occupied), func(i, j int) { occupied[i], occupied[j] = occupied[j], occupied[i] })
		stacks := 0
		for _, slot := range occupied[:min(5, len(occupied))] {
			s := victim.profile.BagSlots[slot]
			if gain(killer, s.Item, s.Count) {
				victim.profile.BagSlots[slot] = Stack{}
				stacks++
			}
		}
		syncInventory(&victim.profile)
		killer.profile.Kills++
		awardHats(&killer.profile)
		w.eventActor(fmt.Sprintf("%s defeated %s and gained %d coins and %d inventory stacks. Safe slots and hotbar stayed safe.", killer.profile.Name, victim.profile.Name, coins, stacks), "defeat", victim.x, victim.z, killer.profile.ID)
	} else {
		w.event(victim.profile.Name+" was defeated by "+cause+" and returned to the forest.", "defeat", victim.x, victim.z)
	}
	w.spawn(victim)
	return true
}

func (w *World) damageAnimal(attacker *player, a *Animal, damage float64) {
	if a.Egg {
		a.eggSince = w.time
		a.HatchIn = 120
		w.event("The disturbed egg needs another quiet two minutes.", "animal", a.X, a.Z)
		return
	}
	actor := ""
	if attacker != nil {
		actor = attacker.profile.ID
	}
	a.Health -= damage
	if attacker != nil {
		attacker.profile.Reputation[a.ID] = max(-10, attacker.profile.Reputation[a.ID]-2)
	}
	if a.Health > 0 {
		w.eventActor("The "+a.Species+" remembers being hurt.", "hit", a.X, a.Z, actor)
		return
	}
	a.Health = 0
	a.respawnAt = w.time + 60
	if attacker != nil {
		for _, witness := range w.animals {
			if witness.Health > 0 && distance(a.X, a.Z, witness.X, witness.Z) <= 35 {
				attacker.profile.Reputation[witness.ID] = max(-10, attacker.profile.Reputation[witness.ID]-3)
			}
		}
		gain(attacker, "sea_salt", 2)
	}
	w.eventActor("A "+a.Species+" was killed. Witnesses will remember; predators may take revenge.", "defeat", a.X, a.Z, actor)
}
