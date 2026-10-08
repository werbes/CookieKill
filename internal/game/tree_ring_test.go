package game

import (
	"math"
	"reflect"
	"testing"
)

func walkTreeTo(t *testing.T, w *World, p *player, x, z float64) {
	t.Helper()
	start := w.time
	for distance(p.x, p.z, x, z) > .08 {
		dx, dz := x-p.x, z-p.z
		length := math.Hypot(dx, dz)
		w.Input(p.profile.ID, Input{X: dx / length, Z: dz / length})
		w.Tick(min(.03, length/6))
		if w.time-start > 6 {
			t.Fatalf("tree climb stuck at %.3f,%.3f,%.3f toward %.2f,%.2f", p.x, playerY(p), p.z, x, z)
		}
	}
	w.Input(p.profile.ID, Input{})
}

func TestTreeHutActualStairClimbAndThreeDimensionalEntry(t *testing.T) {
	w, p := testWorld()
	w.fallRandom = func() int { return 999 }
	p.x, p.z = 213, 177.5
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "cookie_ring_enter", Target: "tree_golden_cookie"}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("golden cookie reachable from ground below hut")
	}
	startAngle := 13*math.Pi/16 - .4
	p.x, p.z = TreeHutX+3.4*math.Cos(startAngle), TreeHutZ+3.4*math.Sin(startAngle)
	startX, startZ := p.x, p.z
	for i := 0; i < 64; i++ {
		angle := 13*math.Pi/16 + (float64(i)+.5)*2*math.Pi/32
		walkTreeTo(t, w, p, TreeHutX+3.4*math.Cos(angle), TreeHutZ+3.4*math.Sin(angle))
		expected := float64(i+1) * .1875
		if math.Abs(playerY(p)-expected) > .4 || !grounded(p) {
			t.Fatalf("spiral tread %d has wrong elevation: got %.3f, expected near %.3f", i, playerY(p), expected)
		}
	}
	for _, point := range []Point{{207.6, 184.8}, {207.6, 181}, {213, 176.5}} {
		walkTreeTo(t, w, p, point.X, point.Z)
		if math.Abs(playerY(p)-TreeHutY) > .01 || !grounded(p) {
			t.Fatal("bridge or original hut room lost its level walkable floor")
		}
	}
	mustAct(t, w, Action{Action: "interact", Target: "tree_golden_cookie"})
	if p.island != CookieRingIsland || p.cookieArena == nil {
		t.Fatal("climbed golden cookie did not enter ring")
	}
	mustAct(t, w, Action{Action: "cookie_arena_leave"})
	if p.island != "" || math.Abs(playerY(p)-12) > .01 || distance(p.x, p.z, 213, 176.5) > .1 {
		t.Fatal("ring leave did not restore hut floor")
	}
	// The same stairs remain traversable downward without menu teleports.
	for _, point := range []Point{{207.6, 181}, {207.6, 184.8}} {
		walkTreeTo(t, w, p, point.X, point.Z)
	}
	for i := 63; i >= 0; i-- {
		angle := 13*math.Pi/16 + (float64(i)+.5)*2*math.Pi/32
		walkTreeTo(t, w, p, TreeHutX+3.4*math.Cos(angle), TreeHutZ+3.4*math.Sin(angle))
		if !grounded(p) {
			t.Fatal("spiral descent required a jump or fall")
		}
	}
	walkTreeTo(t, w, p, startX, startZ)
	if playerY(p) != 0 {
		t.Fatal("tree stairs did not return to the ground")
	}
}

func TestTreeHutSlimTrunkAndExteriorClearance(t *testing.T) {
	w, p := testWorld()
	for _, y := range []float64{0, 4, 8} {
		if !w.solidBlockedAt(TreeHutX, y, TreeHutZ, .42) {
			t.Fatal("narrow tree trunk is not solid")
		}
	}
	// Former rectangular trunk walls no longer enclose the ground approach.
	for _, point := range []Point{{202.7, 184}, {217.3, 184}, {210, 173.7}, {213.8, 194.3}} {
		if w.solidBlockedAt(point.X, 0, point.Z, .42) {
			t.Fatal("old hollow-trunk wall still blocks the exterior")
		}
	}
	p.y = TreeHutY
	for _, point := range []Point{{204.5, 179}, {207, 181}, {215.5, 181}} {
		if treeHutGround(p, point.X, point.Z) != TreeHutY || w.solidBlockedAt(point.X, TreeHutY, point.Z, .42) {
			t.Fatal("the original hut interior did not retain its full walkable floor")
		}
	}
}

func TestTreeHutProjectileHitsRotatedExteriorTread(t *testing.T) {
	c := treeHutSurfaces()[7]
	w := &World{colliders: []Collider{c}}
	angle := -c.Rotation * math.Pi / 180
	x, z := c.X+.9*math.Cos(angle), c.Z+.9*math.Sin(angle)
	if _, hit := w.staticShotHit(x, c.Y-.2, z, x, c.Y+c.Height+.2, z); !hit {
		t.Fatal("projectile passed through the radial end of a rotated stair tread")
	}
	x, z = c.X-.9*math.Sin(angle), c.Z+.9*math.Cos(angle)
	if _, hit := w.staticShotHit(x, c.Y-.2, z, x, c.Y+c.Height+.2, z); hit {
		t.Fatal("projectile hit empty space beside the narrow tangent edge")
	}
}

func ringPlayers(t *testing.T) (*World, *player, *player) {
	t.Helper()
	w, p := testWorld()
	w.Join("two", "Second fighter")
	q := w.players["two"]
	for _, r := range []*player{p, q} {
		r.x, r.y, r.z = 213, 12, 177.5
		if err := w.Act(r.profile.ID, Action{Action: "cookie_ring_enter", Target: "tree_golden_cookie"}); err != nil {
			t.Fatal(err)
		}
	}
	w.time = 5
	return w, p, q
}

func TestCookieRingRealProjectileKillsOnceAndKeepsInventoryMoneySafe(t *testing.T) {
	w, p, q := ringPlayers(t)
	p.x, p.z, p.yaw, p.pitch = 0, -5, 0, 0
	q.x, q.z = 0, -11
	p.profile.Coins, q.profile.Coins = 10, 100
	beforeP, beforeQ := cloneProfile(p.profile), cloneProfile(q.profile)
	for i := 0; i < 6; i++ {
		mustAct(t, w, Action{Action: "throw", Item: "cake"}) // Arena always supplies sugar.
		if shot := w.projectiles[len(w.projectiles)-1]; shot.Item != "sugar" || shot.Island != CookieRingIsland {
			t.Fatal("arena ammo selection escaped supplies")
		}
		w.Tick(.65)
	}
	if p.profile.Kills != beforeP.Kills+1 || p.profile.CookieRingKills != 1 || q.profile.Deaths != beforeQ.Deaths+1 || q.island != CookieRingIsland || q.health != 100 || !w.active(q, "spawn_shield") {
		t.Fatalf("wrong ring result attacker=%+v victimhealth=%v kills%d", p.cookieArena, q.health, p.profile.Kills)
	}
	if p.profile.Coins != 10 || q.profile.Coins != 100 || !reflect.DeepEqual(p.profile.Inventory, beforeP.Inventory) || !reflect.DeepEqual(q.profile.Hotbar, beforeQ.Hotbar) {
		t.Fatal("supplied fight ammo moved inventory or wallet coins")
	}
	w.damageCookieRing(p, q, 1000, 5)
	if p.profile.Kills != 1 || p.profile.CookieRingKills != 1 || q.profile.Deaths != 1 {
		t.Fatal("spawn protection allowed a duplicate ring death")
	}
	mustAct(t, w, Action{Action: "cookie_arena_leave"})
	saved := New(w.Profiles()).Profiles()["one"]
	if saved.Kills != 1 || saved.CookieRingKills != 1 {
		t.Fatal("ring leaderboard kills did not persist")
	}
}

func TestCookieRingWorldIsolationCoverAndRespawnShotInvalidation(t *testing.T) {
	w, p, q := ringPlayers(t)
	w.Join("mainland", "Mainland observer")
	m := w.players["mainland"]
	m.x, m.z = 0, -11
	m.buffs = map[string]float64{}
	p.x, p.z, p.yaw = 0, -5, 0
	q.x, q.z = 0, -11
	mustAct(t, w, Action{Action: "throw"})
	s := w.Snapshot("one")
	if len(s.Projectiles) != 1 || s.Projectiles[0].Island != CookieRingIsland || len(s.Players) != 1 || s.Players[0].HeldItem != "sugar" || !s.CookieArena.UnlimitedAmmo || s.Island != nil {
		t.Fatal("arena snapshot missing isolated gameplay state")
	}
	if len(w.Snapshot("mainland").Projectiles) != 0 {
		t.Fatal("mainland saw arena projectile")
	}
	w.Tick(.5)
	if m.health != 100 || q.health >= 100 {
		t.Fatal("ring shot hit another world or missed same-world target")
	}
	for _, e := range w.Snapshot("mainland").Events {
		if e.Kind == "hit" && e.Actor == p.profile.ID {
			t.Fatal("arena hit event leaked into mainland snapshot")
		}
	}
	// A crate between the fighters stops the actual swept shot.
	p.x, p.z, p.yaw = -8, 0, 0
	q.x, q.z = -8, -10
	q.health = 100
	w.time += 1
	mustAct(t, w, Action{Action: "throw"})
	w.Tick(.6)
	if q.health != 100 {
		t.Fatal("ring shot tunneled through cover")
	}
	p.x, p.z, p.yaw = 0, -5, 0
	q.x, q.z = 0, -11
	w.time += 1
	mustAct(t, w, Action{Action: "throw"})
	old := w.projectiles[len(w.projectiles)-1]
	w.spawnCookieArena(p)
	if w.moveCookieArenaProjectile(old, .2) {
		t.Fatal("projectile from a previous life survived respawn")
	}
	for i := 0; i < 100; i++ {
		w.Input("one", Input{X: 1, Z: 1, Sprint: true})
		w.Tick(.05)
	}
	if math.Abs(p.x) > 23.58 || math.Abs(p.z) > 19.58 {
		t.Fatal("ring movement escaped boundary")
	}
	mustAct(t, w, Action{Action: "cookie_arena_leave"})
	for _, id := range []string{"one", "mainland", ""} {
		for _, e := range w.Snapshot(id).Events {
			if e.Island == CookieRingIsland {
				t.Fatal("retained ring event escaped its world after the actor left")
			}
		}
	}
}

func TestCookieArenaReturnDisconnectAndRaceGatherCleanup(t *testing.T) {
	w, p := testWorld()
	p.x, p.y, p.z, p.health, p.stamina = 213, 12, 177.5, 41, 37
	p.buffs = map[string]float64{"speed": 100}
	w.projectiles = append(w.projectiles, &Projectile{ID: "mainland_shot", Owner: "one", X: 210, Y: 13, Z: 177, born: w.time})
	mustAct(t, w, Action{Action: "cookie_ring_enter", Target: "tree_golden_cookie"})
	if len(w.projectiles) != 0 {
		t.Fatal("entering ring kept the player's mainland shot active")
	}
	w.time = 5
	mustAct(t, w, Action{Action: "throw"})
	w.SetConnected("one", false)
	w.Tick(.05)
	if p.cookieArena != nil || p.island != "" || p.y != 12 || p.health != 41 || p.stamina > 38 || p.buffs["speed"] != 100 || len(w.projectiles) != 0 {
		t.Fatal("disconnect did not restore pre-arena state and remove shots")
	}
	w.Leave("one")
	w.Join("one", "Returning fighter")
	if p.island != "" || playerY(p) != 12 {
		t.Fatal("reconnect lost tree hut return")
	}
	p.x, p.z = -227, 20
	p.y = 0
	mustAct(t, w, Action{Action: "race_enter", Target: "camel_race_lollipop"})
	p.x, p.y, p.z = 213, 12, 177.5
	mustAct(t, w, Action{Action: "cookie_ring_enter", Target: "tree_golden_cookie"})
	w.time = 100
	w.tickCamelRace(0)
	if p.cookieArena != nil || p.island != CamelRaceIsland || w.camelRace.entries["one"].returnTo.island != "" || w.camelRace.entries["one"].returnTo.y != 12 {
		t.Fatal("race gather did not cleanly exit cookie arena")
	}
	mustAct(t, w, Action{Action: "race_leave"})
	if p.island != "" || playerY(p) != 12 {
		t.Fatal("race returned to an orphan arena")
	}
}

func TestCookiePracticeVisitPreservesRealProteinEffects(t *testing.T) {
	w, p := testWorld()
	p.x, p.y, p.z = 215, 12, 181.5
	p.buffs = map[string]float64{"protein": 10, "speed": 20}
	mustAct(t, w, Action{Action: "cookie_practice_enter", Target: "tree_silver_cookie"})
	if p.buffs["protein"] != 10 || p.buffs["speed"] != 20 {
		t.Fatal("practice portal erased active training buffs")
	}
	setItem(p, "protein_drink", 1)
	w.time = 11
	mustAct(t, w, Action{Action: "consume_protein", Item: "protein_drink"})
	until := p.buffs["protein"]
	mustAct(t, w, Action{Action: "cookie_arena_leave"})
	if p.buffs["protein"] != until || until <= 11 || p.profile.Inventory["protein_drink"] != 0 {
		t.Fatal("practice return lost consumed protein or duplicated the item")
	}
	if p.buffs["spawn_shield"] > 0 {
		t.Fatal("peaceful practice visit exported a free spawn shield")
	}
}
