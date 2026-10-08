package game

import (
	"math"
	"testing"
)

func johnTestWorld() (*World, *player) {
	w, p := movementWorld()
	w.time = DayDuration + 10
	p.profile.Coins = 100
	return w, p
}
func placeTestJohn(w *World, p *player, x, z float64) *johnPork {
	j := &johnPork{actor: player{profile: Profile{ID: "john_pork"}, x: x, z: z, y: groundY(p), island: p.island, inCave: p.inCave}, target: p.profile.ID, lastTarget: Point{p.x, p.z}, lastIsland: p.island}
	w.johnPork = j
	return j
}
func tickTestJohn(w *World, seconds float64) {
	for seconds > 1e-9 {
		dt := min(seconds, .05)
		w.time += dt
		w.tickJohnPork(dt)
		seconds -= dt
	}
}

func TestDayNightExactDurationsBoundariesAndRepeats(t *testing.T) {
	for _, tc := range []struct {
		time               float64
		phase              string
		elapsed, remaining float64
		day                int
	}{
		{0, "day", 0, 720, 0}, {719.999, "day", 719.999, .001, 0}, {720, "night", 0, 360, 0}, {1079.999, "night", 359.999, .001, 0}, {1080, "day", 0, 720, 1}, {1800, "night", 0, 360, 1}, {2160, "day", 0, 720, 2},
	} {
		got := dayCycleAt(tc.time)
		if got.Phase != tc.phase || got.DayNumber != tc.day || math.Abs(got.Elapsed-tc.elapsed) > 1e-6 || math.Abs(got.Remaining-tc.remaining) > 1e-6 || got.DayDuration != 720 || got.NightDuration != 360 {
			t.Fatalf("at %.3f: %+v", tc.time, got)
		}
	}
	w, p := johnTestWorld()
	placeTestJohn(w, p, p.x, p.z+.5)
	w.johnIgnored = map[string]bool{"other": true}
	w.time = 1080
	w.tickJohnPork(.05)
	if w.johnPork != nil || len(w.johnIgnored) != 0 {
		t.Fatal("dawn kept John or last night's depletion exclusions")
	}
}

func TestJohnSelectsRichestConnectedAndRetainsTarget(t *testing.T) {
	w, p := johnTestWorld()
	w.Join("two", "Two")
	q := w.players["two"]
	q.buffs = map[string]float64{}
	q.profile.Coins = 200
	q.x, q.z = 40, -20
	w.Join("three", "Three")
	r := w.players["three"]
	r.buffs = map[string]float64{}
	r.profile.Coins = 300
	r.x, r.z = 70, -20
	w.SetConnected("three", false)
	w.tickJohnPork(.05)
	if w.johnPork == nil || w.johnPork.target != "two" {
		t.Fatal("disconnected grace player stole richest connected target")
	}
	p.profile.Coins = 1000
	w.tickJohnPork(.05)
	if w.johnPork.target != "two" {
		t.Fatal("wealth change switched the active pursuit")
	}
	w.SetConnected("two", false)
	w.tickJohnPork(.05)
	if w.johnPork == nil || w.johnPork.target != "one" {
		t.Fatal("departed target was not replaced")
	}
	w.Join("three", "Three")
	if r.disconnected {
		t.Fatal("reconnect did not restore eligibility")
	}
	w.johnPork = nil
	p.profile.Coins = 0
	setItem(r, "sugar", 0)
	w.tickJohnPork(.05)
	if w.johnPork == nil || w.johnPork.target != "one" {
		t.Fatal("richer player with no baked food was selected")
	}
}

func TestJohnEatsEightPerSecondOnlyBakedCarriedFoodsAndStops(t *testing.T) {
	w, p := johnTestWorld()
	setItem(p, "sugar", 10)
	setItem(p, "cake", 2)
	setItem(p, "dough", 9)
	setItem(p, "prepared_cake", 3)
	setItem(p, "berry", 5)
	p.profile.BakeryDisplay = map[string]int{"sugar": 9}
	p.profile.BakeryPantry = map[string]int{"dough": 7}
	placeTestJohn(w, p, p.x, p.z+.5)
	tickTestJohn(w, .1)
	if bakedFoodCount(p) != 12 {
		t.Fatal("ate before the first eighth-second")
	}
	tickTestJohn(w, .9)
	if bakedFoodCount(p) != 4 {
		t.Fatalf("ate %d in a second, want8", 12-bakedFoodCount(p))
	}
	tickTestJohn(w, .5)
	if bakedFoodCount(p) != 0 || w.johnPork != nil || !w.johnIgnored["one"] {
		t.Fatal("did not stop immediately when baked food was depleted")
	}
	if p.profile.Inventory["dough"] != 9 || p.profile.Inventory["prepared_cake"] != 3 || p.profile.Inventory["berry"] != 5 || p.profile.BakeryDisplay["sugar"] != 9 || p.profile.BakeryPantry["dough"] != 7 || p.profile.Coins != 100 {
		t.Fatal("John ate raw ingredients, stored food, or coins")
	}
	setItem(p, "sugar", 5)
	tickTestJohn(w, 1)
	if w.johnPork != nil || p.profile.Inventory["sugar"] != 5 {
		t.Fatal("depleted player was pursued again the same night")
	}
	w.time = 1080
	w.tickJohnPork(.05)
	w.time = 1801
	w.tickJohnPork(.05)
	if w.johnPork == nil {
		t.Fatal("new night did not reset depletion exclusions")
	}
}

func TestJohnEatingChecksRangeHeightAndWalls(t *testing.T) {
	w, p := johnTestWorld()
	j := placeTestJohn(w, p, p.x+.99, p.z)
	if !w.johnCanEat(&j.actor, p) {
		t.Fatal("clear reachable food was not in range")
	}
	j.actor.x = p.x + 1.01
	if w.johnCanEat(&j.actor, p) {
		t.Fatal("ate outside one metre")
	}
	j.actor.x = p.x + .6
	p.jumpOffset = 1
	if w.johnCanEat(&j.actor, p) {
		t.Fatal("ate across excess vertical distance")
	}
	p.jumpOffset = 0
	w.colliders = []Collider{{X: p.x + .3, Z: p.z, Width: .1, Depth: 4, Height: 3}}
	if w.johnCanEat(&j.actor, p) {
		t.Fatal("ate through a solid wall")
	}
	w.colliders = nil
	j.actor.inCave = true
	if w.johnCanEat(&j.actor, p) {
		t.Fatal("ate between surface and underground layers")
	}
}

func TestJohnSpeedIsNinetyPercentAcrossMovementModes(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		legs, swim           int
		camel, sprint, water bool
		buffs                map[string]float64
		speed                float64
	}{
		{"walk", 0, 0, false, false, false, nil, 6}, {"trained", 5, 0, false, false, false, nil, 7.2}, {"sprint", 0, 0, false, true, false, nil, 9.9}, {"camel", 0, 0, true, false, false, nil, 10.8}, {"swim", 0, 5, false, false, true, nil, 5.2}, {"swim buff", 0, 0, false, false, true, map[string]float64{"swim": 9999}, 6.4}, {"speed buff", 0, 0, false, false, false, map[string]float64{"speed": 9999}, 8.4}, {"slowed", 0, 0, false, false, false, map[string]float64{"slowed": 9999}, 3.3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, p := johnTestWorld()
			p.profile.Levels["legs"], p.profile.Levels["swim"] = tc.legs, tc.swim
			p.ridingCamel, p.sprinting = tc.camel, tc.sprint
			if tc.water {
				p.x, p.z = -100, 100
			}
			if tc.buffs != nil {
				p.buffs = tc.buffs
			}
			j := placeTestJohn(w, p, p.x-15, p.z)
			x := j.actor.x
			if math.Abs(w.playerMovementSpeed(p)-tc.speed) > 1e-8 {
				t.Fatal("speed fixture disagrees with authoritative player formula")
			}
			tickTestJohn(w, 1)
			if math.Abs(j.actor.x-x-tc.speed*.9) > 1e-6 {
				t.Fatalf("moved %.8f, want %.8f", j.actor.x-x, tc.speed*.9)
			}
		})
	}
}

func TestJohnRoutesAroundWallsAndCannotPassClosedIslandDoor(t *testing.T) {
	w, p := johnTestWorld()
	p.x, p.z = 14, -20
	j := placeTestJohn(w, p, 6, -20)
	w.colliders = []Collider{{X: 10, Z: -20, Width: .2, Depth: 8, Height: 4}}
	for i := 0; i < 160 && !w.johnCanEat(&j.actor, p); i++ {
		tickTestJohn(w, .05)
		if w.solidBlockedAt(j.actor.x, groundY(&j.actor), j.actor.z, .42) {
			t.Fatal("John crossed a wall")
		}
	}
	if !w.johnCanEat(&j.actor, p) {
		t.Fatalf("failed to route around wall: %+v", j.actor)
	}
	w, p = johnTestWorld()
	p.profile.HomeIsland = &Island{Owner: "one", Objects: []BuildObject{{ID: "door", Kind: "door", X: 0, Z: 0}, {ID: "wall_left", Kind: "wall", X: -2.7, Z: 0}, {ID: "wall_right", Kind: "wall", X: 2.7, Z: 0}}}
	p.island = "one"
	p.x, p.z = 0, -2
	j = placeTestJohn(w, p, 0, 2)
	if _, ok := w.johnSegment(j.actor, Point{0, -2}); ok {
		t.Fatal("John walked through the closed island door")
	}
	p.profile.HomeIsland.Objects[0].Open = true
	if _, ok := w.johnSegment(j.actor, Point{0, -2}); !ok {
		t.Fatal("John could not walk through an open island door")
	}
}

func TestJohnUsesCaveStairsAndFollowsRealTeleportWithDelay(t *testing.T) {
	w, p := testWorld()
	w.time = DayDuration + 10
	p.x, p.z = 120, -94
	p.inCave = true
	j := placeTestJohn(w, p, 120, -73)
	j.actor.inCave = false
	for i := 0; i < 120 && !w.johnCanEat(&j.actor, p); i++ {
		tickTestJohn(w, .05)
		if w.solidBlockedAt(j.actor.x, groundY(&j.actor), j.actor.z, .42) {
			t.Fatal("John clipped the rocky entrance or tunnel")
		}
	}
	if !j.actor.inCave || j.actor.z > -92.9 || groundY(&j.actor) >= -8 {
		t.Fatalf("did not descend physical stairs: x%.2f z%.2f y%.2f", j.actor.x, j.actor.z, groundY(&j.actor))
	}
	p.profile.HomeIsland = &Island{Owner: "one", Objects: []BuildObject{}}
	p.island = "one"
	p.inCave = false
	p.x, p.z, p.y = 0, 0, 0
	tickTestJohn(w, .05)
	if w.johnPorkSnapshot(p) != nil {
		t.Fatal("world transition moved John instantly onto the target")
	}
	tickTestJohn(w, 1.9)
	if w.johnPorkSnapshot(p) != nil {
		t.Fatal("teleport pursuit delay ended early")
	}
	tickTestJohn(w, .15)
	view := w.johnPorkSnapshot(p)
	if view == nil || view.Island != "one" || view.InCave || distance(view.X, view.Z, p.x, p.z) < 3 {
		t.Fatal("John did not follow to a clear island arrival at a fair distance")
	}
}

func TestJohnSnapshotPrivacyAndDetachment(t *testing.T) {
	w, p := johnTestWorld()
	j := placeTestJohn(w, p, p.x-4, p.z)
	snap := w.Snapshot("one")
	if snap.JohnPork == nil || snap.DayCycle.Phase != "night" {
		t.Fatal("missing authoritative night snapshot")
	}
	snap.JohnPork.X = 999
	if j.actor.x == 999 {
		t.Fatal("snapshot aliases NPC runtime")
	}
	p.inCave = true
	if w.Snapshot("one").JohnPork != nil {
		t.Fatal("surface NPC leaked to cave view")
	}
	p.inCave = false
	p.island = "other"
	if w.Snapshot("one").JohnPork != nil {
		t.Fatal("NPC leaked to another private island")
	}
}

func TestJohnClubSanctuaryCancelsPursuitWithoutIgnoringPlayer(t *testing.T) {
	w, p := testWorld()
	w.time = DayDuration + 10
	p.x, p.z = ClubX, ClubMouthZ+.3
	setItem(p, "sugar", 20)
	j := placeTestJohn(w, p, ClubX, ClubMouthZ+.8)
	j.eatProgress, j.eating = .7, true
	p.inCave = true
	p.z = ClubMouthZ - .1
	if w.johnEligible(p) || w.johnCanEat(&j.actor, p) || w.spawnJohnNear(&johnPork{}, p) {
		t.Fatal("the underground club was not a sanctuary at its mouth")
	}
	w.tickJohnPork(.25)
	if w.johnPork != nil || w.johnIgnored[p.profile.ID] || p.profile.Inventory["sugar"] != 20 {
		t.Fatal("entering the club kept the pursuit, ate food, or permanently ignored the player")
	}
	for _, at := range []Point{{ClubX, -214}, {-191, -245}} {
		p.x, p.z = at.X, at.Z
		if w.johnEligible(p) || w.spawnJohnNear(&johnPork{}, p) {
			t.Fatal("stairs or dance floor allowed a pursuit")
		}
	}
	p.x, p.z, p.inCave = ClubX, ClubMouthZ+1, false
	w.tickJohnPork(.05)
	if w.johnPork == nil || w.johnPork.target != p.profile.ID {
		t.Fatal("leaving the sanctuary did not restore eligibility")
	}
}

func TestJohnRetargetsOutsideClubAndNavigationCannotEnter(t *testing.T) {
	w, p := testWorld()
	w.time = DayDuration + 10
	p.profile.Coins = 1000
	p.x, p.z, p.inCave = -191, -245, true
	w.Join("outside", "Outside")
	q := w.players["outside"]
	q.buffs = map[string]float64{}
	q.x, q.z = 40, -20
	placeTestJohn(w, p, ClubX, ClubMouthZ+1)
	w.johnPork.actor.inCave = false
	w.tickJohnPork(.05)
	if w.johnPork == nil || w.johnPork.target != q.profile.ID {
		t.Fatal("safe wealthy player prevented retargeting to an eligible player")
	}
	actor := player{profile: Profile{ID: "john_pork"}, x: ClubX, z: ClubMouthZ + .1}
	if _, ok := w.johnSegment(actor, Point{ClubX, ClubStairTopZ - 1}); ok {
		t.Fatal("navigation followed the player into the underground disco")
	}
	if route := w.johnRoute(actor, Point{ClubX, -214}); len(route) > 0 {
		for _, step := range route {
			var ok bool
			actor, ok = w.johnSegment(actor, step)
			if !ok || johnSanctuary(&actor) {
				t.Fatal("path search found a forbidden club entry")
			}
		}
	}
	// Defense in depth also handles an obsolete actor already inside the club.
	j := placeTestJohn(w, p, p.x, p.z+.4)
	if w.johnCanEat(&j.actor, p) {
		t.Fatal("an obsolete underground actor ate in the sanctuary")
	}
	w.moveJohnPork(j, p, 10)
	if j.actor.x != p.x || j.actor.z != p.z+.4 {
		t.Fatal("obsolete actor kept moving inside the sanctuary")
	}
}

func TestJohnNeverEntersCamelRace(t *testing.T) {
	w, p := johnTestWorld()
	p.island = CamelRaceIsland
	j := placeTestJohn(w, p, p.x, p.z+.4)
	if w.johnEligible(p) || w.johnCanEat(&j.actor, p) || w.spawnJohnNear(j, p) {
		t.Fatal("John interfered with a race participant")
	}
	if _, ok := w.johnStep(j.actor, j.actor.x+.1, j.actor.z); ok {
		t.Fatal("John navigation accepted the race island")
	}
	w.tickJohnPork(.05)
	if w.johnPork != nil || w.johnIgnored[p.profile.ID] {
		t.Fatal("race entry failed to release pursuit temporarily")
	}
}

func TestJohnSleepsAtAccessibleNortheastHutDuringDay(t *testing.T) {
	w, p := johnTestWorld()
	w.seedJohnHut()
	placeTestJohn(w, p, p.x, p.z+.5)
	w.time = DayDuration + NightDuration
	w.tickJohnPork(.05)
	view := w.johnPorkSnapshot(p)
	if view == nil || !view.Sleeping || view.X != JohnBedX || view.Y != JohnBedY || view.Z != JohnBedZ || view.Yaw != JohnBedYaw || view.Target != "" || view.Eating || view.Island != "" || view.InCave {
		t.Fatalf("dawn did not put John in his bed: %+v", view)
	}
	view.X = 0
	if w.johnPorkSnapshot(p).X != JohnBedX {
		t.Fatal("sleeping snapshot aliases state")
	}
	p.island = "private"
	if w.johnPorkSnapshot(p) != nil {
		t.Fatal("sleeper leaked onto private island")
	}
	p.island, p.inCave = "", true
	if w.johnPorkSnapshot(p) != nil {
		t.Fatal("sleeper leaked underground")
	}
	p.inCave, p.x, p.z = false, JohnHutX, JohnHutZ+6
	for z := JohnHutZ + 5.9; z >= JohnHutZ; z -= .1 {
		if !w.moveMainland(p, JohnHutX, z, 0) {
			t.Fatalf("hut doorway blocked at z %.2f", z)
		}
		p.x, p.z = JohnHutX, z
	}
	if !w.solidBlockedAt(JohnHutX+4, 0, JohnHutZ, .42) || !w.solidBlockedAt(JohnHutX-1.8, 0, JohnHutZ-.9, .42) {
		t.Fatal("hut walls or bed have no physical collision")
	}
	if JohnHutX < 200 || JohnHutZ > -220 {
		t.Fatal("hut is not in the far northeast")
	}
	w.time = DayDuration + 1
	w.tickJohnPork(.05)
	if john := w.johnPorkSnapshot(p); john != nil && john.Sleeping {
		t.Fatal("John stayed asleep after nightfall")
	}
}

func BenchmarkJohnBlockedRoute(b *testing.B) {
	w, p := movementWorld()
	p.x, p.z = 14, -20
	w.colliders = []Collider{{X: 10, Z: -20, Width: .2, Depth: 8, Height: 4}}
	j := placeTestJohn(w, p, 6, -20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.johnRoute(j.actor, Point{p.x, p.z})
	}
}
