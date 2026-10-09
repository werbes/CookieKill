package game

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

const (
	CookieRingIsland     = "cookie_ring"
	CookiePracticeIsland = "cookie_practice"
	TreeHutX             = 210.0
	TreeHutZ             = 184.0
	TreeHutY             = 12.0
	treeStairCount       = 64
	treeStairRadius      = 3.4
	treeStairStart       = 13 * math.Pi / 16 // Keep the upper turn clear of the hut's south floor edge.
	treeStairTurn        = 2 * math.Pi / 32
)

func isCookieArena(island string) bool {
	return island == CookieRingIsland || island == CookiePracticeIsland
}

// Two exterior turns wind around the narrow trunk. Each tread is a thin slab,
// leaving six metres between turns and a clear final approach south of the hut.
func treeHutSurfaces() []Collider {
	surfaces := make([]Collider, 0, treeStairCount+2)
	for i := 0; i < treeStairCount; i++ {
		angle := treeStairStart + (float64(i)+.5)*treeStairTurn
		top := float64(i+1) * TreeHutY / treeStairCount
		surfaces = append(surfaces, Collider{ID: fmt.Sprintf("tree_hut_stair_%d", i), X: TreeHutX + treeStairRadius*math.Cos(angle), Z: TreeHutZ + treeStairRadius*math.Sin(angle), Width: 2.2, Depth: 1.05, Y: top - .16, Height: .16, Rotation: -angle * 180 / math.Pi})
	}
	for _, c := range []Collider{
		{ID: "tree_hut_bridge", X: 207.6, Z: 184.8, Width: 2.8, Depth: 4, Y: 11.7, Height: .3},
		{ID: "tree_hut_deck", X: 210, Z: 179, Width: 14, Depth: 10, Y: 11.7, Height: .3},
	} {
		surfaces = append(surfaces, c)
	}
	return surfaces
}

func treeHutGround(p *player, x, z float64) float64 {
	if math.Abs(x-TreeHutX) > 9 || math.Abs(z-TreeHutZ) > 12 {
		return 0
	}
	height := 0.
	for _, c := range treeHutSurfaces() {
		if colliderOverlaps(c, x, z, .42) {
			top := c.Y + c.Height
			if top <= playerY(p)+.4 {
				height = max(height, top)
			}
		}
	}
	return height
}

func (w *World) seedTreeHut() {
	w.layout.Props = append(w.layout.Props, WorldProp{ID: "south_tree_hut", Kind: "tree_hut", Label: "The Cookie Canopy", X: TreeHutX, Z: TreeHutZ, Width: 14, Depth: 20, Height: 22, Scale: 1})
	w.nodes = append(w.nodes, &Node{ID: "tree_golden_cookie", Kind: "tree_golden_cookie", Label: "Golden cookie · fighting ring", X: 213, Y: TreeHutY, Z: 177.5, Available: true}, &Node{ID: "tree_silver_cookie", Kind: "tree_silver_cookie", Label: "Silver cookie · practice and gym", X: 215, Y: TreeHutY, Z: 181.5, Available: true})
	w.colliders = append(w.colliders, treeHutSurfaces()...)
	for _, c := range []Collider{
		{ID: "tree_hut_trunk", X: TreeHutX, Z: TreeHutZ, Radius: 1.5, Height: 12},
		{ID: "tree_hut_wall_west", X: 202.7, Z: 179, Width: .6, Depth: 10, Y: 12, Height: 3.8},
		{ID: "tree_hut_wall_east", X: 217.3, Z: 179, Width: .6, Depth: 10, Y: 12, Height: 3.8},
		{ID: "tree_hut_wall_north", X: 210, Z: 173.7, Width: 14, Depth: .6, Y: 12, Height: 3.8},
		{ID: "tree_hut_wall_south", X: 213.5, Z: 184.3, Width: 7, Depth: .6, Y: 12, Height: 3.8},
		{ID: "tree_hut_roof", X: 210, Z: 179, Width: 14, Depth: 10, Y: 16, Height: .3},
	} {
		w.colliders = append(w.colliders, c)
	}
}

type cookieArenaSession struct {
	returnTo      camelRaceReturn
	health        float64
	buffs         map[string]float64
	life          uint64
	kills, deaths int
}

type CookieArenaView struct {
	InArena       bool    `json:"inArena"`
	Mode          string  `json:"mode"`
	AmmoItem      string  `json:"ammoItem"`
	UnlimitedAmmo bool    `json:"unlimitedAmmo"`
	ReturnLabel   string  `json:"returnLabel"`
	Kills         int     `json:"kills"`
	Deaths        int     `json:"deaths"`
	ShieldSeconds float64 `json:"shieldSeconds"`
}

func (w *World) cookieArenaAction(p *player, a Action) error {
	if validChoice(a.Action, "cookie_arena_leave", "cookie_ring_leave", "cookie_practice_leave") {
		return w.leaveCookieArena(p)
	}
	mode, x, z := CookieRingIsland, 213., 177.5
	if a.Target == "tree_silver_cookie" {
		mode, x, z = CookiePracticeIsland, 215, 181.5
	} else if a.Target != "tree_golden_cookie" {
		return errors.New("Climb the south tree and taste the golden or silver cookie in its hut.")
	}
	if a.Action == "cookie_ring_enter" && mode != CookieRingIsland || a.Action == "cookie_practice_enter" && mode != CookiePracticeIsland {
		return errors.New("Choose the matching cookie in the tree hut.")
	}
	if p.island != "" || p.inCave || !grounded(p) || p.ridingCamel || math.Hypot(distance(p.x, p.z, x, z), playerY(p)-TreeHutY) > 3.2 {
		return errors.New("Climb the tree stairs on foot and stand beside the cookie inside the hut.")
	}
	w.clearCookieArenaShots(p.profile.ID)
	w.sequence++
	p.cookieArena = &cookieArenaSession{returnTo: camelRaceReturn{x: p.x, y: p.y, z: p.z, yaw: p.yaw, pitch: p.pitch, island: p.island, inCave: p.inCave, ridingCamel: p.ridingCamel, mountedCamelID: p.mountedCamelID, stamina: p.stamina, sprintExhausted: p.sprintExhausted, fallen: p.fallen, standAt: p.standAt, fallDistance: p.fallDistance}, health: p.health, buffs: cloneMap(p.buffs), life: w.sequence}
	p.island, p.inCave, p.ridingCamel, p.mountedCamelID, p.y = mode, false, false, "", 0
	w.leaveJohnPorkBehind(p)
	p.posture, p.buildMode, p.editingBuildID, p.bakeryBuildMode, p.editingBakeryFurnitureID = "", false, "", false, ""
	p.camelCalls = nil
	w.spawnCookieArena(p)
	if mode == CookiePracticeIsland {
		w.cookiePracticeEnter(p)
	}
	return nil
}

func (w *World) leaveCookieArena(p *player) error {
	if p.cookieArena == nil || !isCookieArena(p.island) {
		return errors.New("You are not in a cookie arena.")
	}
	session := p.cookieArena
	if p.island == CookiePracticeIsland {
		// Practice is peaceful: normal food/protein effects continue across
		// the visit, including protein actually consumed during a workout.
		session.buffs = cloneMap(p.buffs)
	}
	w.cookiePracticeLeave(p)
	s := session.returnTo
	p.cookieArena = nil
	p.x, p.y, p.z, p.yaw, p.pitch = s.x, s.y, s.z, s.yaw, s.pitch
	p.island, p.inCave, p.ridingCamel, p.mountedCamelID = s.island, s.inCave, s.ridingCamel, s.mountedCamelID
	p.health, p.stamina, p.sprintExhausted = session.health, s.stamina, s.sprintExhausted
	p.fallen, p.standAt, p.fallDistance = s.fallen, s.standAt, s.fallDistance
	p.posture = ""
	p.buffs = session.buffs
	resetMovement(p)
	w.clearCookieArenaShots(p.profile.ID)
	return nil
}

func (w *World) clearCookieArenaShots(owner string) {
	shots := w.projectiles[:0]
	for _, shot := range w.projectiles {
		if shot.Owner != owner {
			shots = append(shots, shot)
		}
	}
	w.projectiles = shots
}

func cookieRingColliders() []Collider {
	return []Collider{
		{ID: "ring_cover_1", X: -8, Z: -5, Width: 4, Depth: 3, Height: 2.2},
		{ID: "ring_cover_2", X: 8, Z: 5, Width: 4, Depth: 3, Height: 2.2},
		{ID: "ring_cover_3", X: -8, Z: 8, Width: 3, Depth: 5, Height: 1.4},
		{ID: "ring_cover_4", X: 8, Z: -8, Width: 3, Depth: 5, Height: 1.4},
		{ID: "ring_wall_west", X: -24.5, Z: 0, Width: 1, Depth: 42, Height: 5},
		{ID: "ring_wall_east", X: 24.5, Z: 0, Width: 1, Depth: 42, Height: 5},
		{ID: "ring_wall_north", X: 0, Z: -20.5, Width: 50, Depth: 1, Height: 5},
		{ID: "ring_wall_south", X: 0, Z: 20.5, Width: 50, Depth: 1, Height: 5},
	}
}

func cookieArenaColliders(island string) []Collider {
	if island == CookiePracticeIsland {
		return cookiePracticeColliders()
	}
	return cookieRingColliders()
}

func cookieArenaBlocked(p *player, x, z float64) bool {
	if p.island == CookiePracticeIsland {
		return cookiePracticeBlocked(p, x, z)
	}
	if math.Abs(x) > 23.58 || math.Abs(z) > 19.58 {
		return true
	}
	for _, c := range cookieRingColliders() {
		if playerY(p) < c.Y+c.Height && playerY(p)+1.7 > c.Y && colliderOverlaps(c, x, z, .42) {
			return true
		}
	}
	return false
}

func cookieArenaShotHit(island string, x, y, z, nx, ny, nz float64) (float64, bool) {
	nearest, hit := 2., false
	for _, c := range cookieArenaColliders(island) {
		if at, ok := colliderSegmentHit(c, x, y, z, nx, ny, nz); ok && at < nearest {
			nearest, hit = at, true
		}
	}
	return nearest, hit
}

func (w *World) spawnCookieArena(p *player) {
	spots := []Point{{-20, -16}, {20, 16}, {-20, 16}, {20, -16}}
	best, bestClear := spots[0], -1.
	for _, spot := range spots {
		clear := math.Inf(1)
		for id, other := range w.players {
			if id != p.profile.ID && other.island == p.island && !other.disconnected {
				clear = min(clear, distance(spot.X, spot.Z, other.x, other.z))
			}
		}
		if clear > bestClear {
			best, bestClear = spot, clear
		}
	}
	if p.island == CookiePracticeIsland {
		best = Point{0, 20}
	}
	p.x, p.y, p.z, p.yaw, p.pitch = best.X, 0, best.Z, math.Atan2(best.X, best.Z), 0
	p.health, p.stamina, p.sprintExhausted = 100, staminaCapacity(p), false
	p.fallen, p.standAt, p.fallDistance = false, 0, 0
	p.posture = ""
	p.buffs = map[string]float64{"spawn_shield": w.time + 3}
	if p.island == CookiePracticeIsland {
		p.buffs = cloneMap(p.cookieArena.buffs)
	}
	w.sequence++
	p.cookieArena.life = w.sequence
	resetMovement(p)
}

func (w *World) tickCookieArenas() {
	for _, p := range w.players {
		if p.disconnected && p.cookieArena != nil {
			_ = w.leaveCookieArena(p)
		}
	}
}

func (w *World) throwCookieArena(p *player) error {
	if p.cookieArena == nil || p.disconnected {
		return errors.New("Enter the cookie arena first.")
	}
	if cookiePracticeMovementLocked(p) {
		return errors.New("Step off the exercise equipment before throwing cookies.")
	}
	if p.cooldowns["throw"] > w.time {
		return errors.New("Your next throw is not ready yet.")
	}
	arms := float64(p.profile.Levels["arms"])
	speed := 22 + arms*.8
	r, _ := recipeFor("sugar")
	damage := r.Damage * (1 + arms*.045)
	dx, dz, dy := -math.Sin(p.yaw)*math.Cos(p.pitch), -math.Cos(p.yaw)*math.Cos(p.pitch), math.Sin(p.pitch)
	x, y, z := p.x+dx*.8, playerY(p)+1.45+dy*.8, p.z+dz*.8
	if _, hit := cookieArenaShotHit(p.island, p.x, playerY(p)+1.45, p.z, x, y, z); hit {
		return errors.New("Move back from the obstacle before throwing.")
	}
	delete(p.buffs, "spawn_shield")
	w.sequence++
	w.projectiles = append(w.projectiles, &Projectile{ID: strconv.FormatUint(w.sequence, 10), Owner: p.profile.ID, Item: "sugar", Island: p.island, X: x, Y: y, Z: z, VX: dx * speed, VY: dy * speed, VZ: dz * speed, damage: damage, born: w.time, arenaLife: p.cookieArena.life})
	p.cooldowns["throw"] = w.time + .6/(1+arms*.08)
	return nil
}

// Arena shots never enter mainland collision, wildlife, safe-zone, or rewards
// code. A player's life generation invalidates old shots after a respawn.
func (w *World) moveCookieArenaProjectile(shot *Projectile, dt float64) bool {
	owner := w.players[shot.Owner]
	if owner == nil || owner.disconnected || owner.cookieArena == nil || owner.island != shot.Island || owner.cookieArena.life != shot.arenaLife {
		return false
	}
	nx, ny, nz := shot.X+shot.VX*dt, shot.Y+shot.VY*dt, shot.Z+shot.VZ*dt
	nearest, blocked := cookieArenaShotHit(shot.Island, shot.X, shot.Y, shot.Z, nx, ny, nz)
	var victim *player
	var target string
	hitCoins := 0
	if shot.Island == CookieRingIsland {
		for id, p := range w.players {
			if id == shot.Owner || p.island != shot.Island || p.cookieArena == nil || p.disconnected {
				continue
			}
			if at, coins, hit := playerHit(shot, nx, ny, nz, p); hit && at < nearest {
				nearest, blocked, victim, hitCoins = at, false, p, coins
			}
		}
	} else if at, id, hit := w.cookiePracticeShotHit(shot, nx, ny, nz); hit && at < nearest {
		target, blocked = id, false
	}
	if blocked {
		return false
	}
	if victim != nil {
		w.damageCookieRing(owner, victim, shot.damage, hitCoins)
		return false
	}
	if target != "" {
		w.hitCookiePracticeTarget(owner, target, shot)
		return false
	}
	limit := 25.
	if shot.Island == CookieRingIsland {
		limit = 24
	}
	if ny < -1 || math.Abs(nx) > limit+1 || math.Abs(nz) > 26 || w.time-shot.born > 5 {
		return false
	}
	shot.X, shot.Y, shot.Z = nx, ny, nz
	shot.VY -= 5 * dt
	return true
}

func (w *World) damageCookieRing(killer, victim *player, damage float64, zone int) bool {
	if killer == nil || killer == victim || killer.island != CookieRingIsland || victim.island != CookieRingIsland || killer.cookieArena == nil || victim.cookieArena == nil || victim.disconnected || w.active(victim, "spawn_shield") || victim.health <= 0 || !finite(damage) || damage <= 0 {
		return false
	}
	victim.health -= damage
	if victim.health > 0 {
		w.eventActor(fmt.Sprintf("Hit %s for %.0f damage.", victim.profile.CallName, damage), "hit", victim.x, victim.z, killer.profile.ID)
		if len(w.events) > 0 {
			e := &w.events[len(w.events)-1]
			e.HitZone = "body"
			if zone == 5 {
				e.HitZone = "head"
			} else if zone == 3 {
				e.HitZone = "hand/foot"
			}
		}
		return false
	}
	killer.profile.Kills = min(coinLimit, killer.profile.Kills+1)
	killer.profile.CookieRingKills = min(coinLimit, killer.profile.CookieRingKills+1)
	victim.profile.Deaths = min(coinLimit, victim.profile.Deaths+1)
	killer.cookieArena.kills++
	victim.cookieArena.deaths++
	awardAchievements(&killer.profile)
	w.eventActor("Cookie ring victory over "+victim.profile.CallName+"!", "defeat", victim.x, victim.z, killer.profile.ID)
	w.eventActor("You were tagged out. Respawned in the ring with three seconds of protection.", "notification", victim.x, victim.z, victim.profile.ID)
	w.spawnCookieArena(victim)
	return true
}

func (w *World) decorateCookieArenaSnapshot(p *player, out *Snapshot) {
	out.CookieArena = CookieArenaView{}
	if p == nil || p.cookieArena == nil || !isCookieArena(p.island) {
		return
	}
	mode, label, node, z := "ring", "Cookie fighting ring", "cookie_ring_exit", 17.
	if p.island == CookiePracticeIsland {
		mode, label, node, z = "practice", "Cookie practice and gym", "cookie_practice_exit", 22
	}
	out.CookieArena = CookieArenaView{InArena: true, Mode: mode, AmmoItem: "sugar", UnlimitedAmmo: true, ReturnLabel: "South tree hut", Kills: p.cookieArena.kills, Deaths: p.cookieArena.deaths, ShieldSeconds: max(0, p.buffs["spawn_shield"]-w.time)}
	out.Me.Area, out.Me.Zone = mode, label
	out.Me.CanBuildHome, out.Me.BuildMode, out.Me.BakeryBuildMode = false, false, false
	out.Island = nil
	out.Nodes = []Node{{ID: node, Kind: node, Label: "Return to the tree hut", X: 0, Z: z, Island: p.island, Available: true}}
	out.Animals, out.Homes, out.Projectiles = []Animal{}, []Home{}, []Projectile{}
	for _, shot := range w.projectiles {
		if shot.Island == p.island {
			out.Projectiles = append(out.Projectiles, *shot)
		}
	}
	for i := range out.Players {
		out.Players[i].Area = mode
		out.Players[i].HeldItem = "sugar"
	}
}
