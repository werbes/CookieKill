package game

import (
	"math"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"
)

func rodentTestWorld() (*World, *player) {
	w, p := testWorld()
	w.rodents.entities = map[string]*rodent{}
	w.rodents.nextSpawn = math.Inf(1)
	w.colliders, w.propertyColliders = nil, nil
	p.x, p.y, p.z = -120, 0, -100
	return w, p
}

func mustRodent(t *testing.T, w *World, species, island string, x, z float64) *rodent {
	t.Helper()
	r := w.spawnRodent(species, island, x, z)
	if r == nil {
		t.Fatalf("could not spawn %s at %.1f,%.1f", species, x, z)
	}
	return r
}

func TestRodentCatchAtomicRangeWallsLayersAndPersistence(t *testing.T) {
	w, p := rodentTestWorld()
	r := mustRodent(t, w, "wood_mouse", "", p.x+4, p.z)
	w.propertyColliders = []Collider{{ID: "wall", X: p.x + 2, Z: p.z, Width: .3, Depth: 6, Height: 3}}
	if err := w.catchRodent(p, r.ID); err == nil {
		t.Fatal("caught through a wall")
	}
	w.propertyColliders = nil
	for _, change := range []func(){func() { p.inCave = true }, func() { p.island = "elsewhere" }, func() { p.y = 7 }, func() { p.fallen = true }, func() { p.disconnected = true }} {
		change()
		if err := w.catchRodent(p, r.ID); err == nil {
			t.Fatal("invalid layer, posture, or connection allowed capture")
		}
		p.inCave, p.fallen, p.disconnected = false, false, false
		p.island, p.y = "", 0
	}
	for i := range p.profile.BagSlots {
		p.profile.BagSlots[i] = Stack{"dough", inventoryLimit}
	}
	for i := range p.profile.Hotbar {
		p.profile.Hotbar[i] = Stack{"dough", inventoryLimit}
	}
	syncInventory(&p.profile)
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "catch_rodent", Target: r.ID}); err == nil {
		t.Fatal("full bag caught an animal")
	}
	if !reflect.DeepEqual(before, p.profile) || w.rodents.entities[r.ID] != r {
		t.Fatal("failed catch lost inventory or wildlife")
	}
	p.profile.BagSlots[3] = Stack{}
	syncInventory(&p.profile)
	if err := w.Act("one", Action{Action: "catch_rodent", Target: r.ID}); err != nil {
		t.Fatal(err)
	}
	if w.rodents.entities[r.ID] != nil || p.profile.Inventory["pet_wood_mouse"] != 1 {
		t.Fatal("capture did not exchange exactly one animal for one pet")
	}
	if err := w.catchRodent(p, r.ID); err == nil {
		t.Fatal("same animal captured twice")
	}
	w.Leave("one")
	reloaded := New(w.Profiles())
	reloaded.Join("one", "Baker")
	if reloaded.Snapshot("one").Me.Inventory["pet_wood_mouse"] != 1 {
		t.Fatal("captured pet did not survive offline save and load")
	}
}

func TestRodentSpecialCaptureRadiiAndHiddenStates(t *testing.T) {
	for _, spec := range rodentCatalog {
		t.Run(spec.ID, func(t *testing.T) {
			w, p := rodentTestWorld()
			r := mustRodent(t, w, spec.ID, "", p.x+spec.CaptureRadius+.01, p.z)
			r.State, r.Y, r.X, r.Z = "wandering", 0, p.x+spec.CaptureRadius+.01, p.z
			if err := w.catchRodent(p, r.ID); err == nil {
				t.Fatal("capture radius exceeded")
			}
			r.X = p.x + spec.CaptureRadius - .1
			for _, state := range []string{"hidden", "diving", "burrowing", "rainbow"} {
				r.State, r.stateUntil = state, w.time+10
				if err := w.catchRodent(p, r.ID); err == nil {
					t.Fatalf("captured state %s", state)
				}
			}
			r.State = "wandering"
			if spec.ID == "otter" {
				r.State, r.nutFed = "confused", true
			}
			if err := w.catchRodent(p, r.ID); err != nil {
				t.Fatal(err)
			}
			if p.profile.Inventory["pet_"+spec.ID] != 1 {
				t.Fatal("wrong captured species")
			}
		})
	}
}

func TestRodentSpeedsBlindHearingAndZigzags(t *testing.T) {
	for _, spec := range []struct {
		id    string
		speed float64
	}{{"wood_mouse", 6}, {"house_mouse", 6}, {"rat", 9.9}, {"cactus_mouse", 3.3}, {"squirrel", 8.1}} {
		t.Run(spec.id, func(t *testing.T) {
			w, p := rodentTestWorld()
			p.sprinting = true
			r := mustRodent(t, w, spec.id, "", p.x+8, p.z)
			w.tickGroundRodent(r, .1)
			if math.Abs(r.speed-spec.speed) > 1e-8 || r.State != "fleeing" {
				t.Fatalf("speed %.3f, want %.3f", r.speed, spec.speed)
			}
			if spec.id == "squirrel" && r.Z == p.z {
				t.Fatal("squirrel ran straight without zigzag")
			}
		})
	}
	w, p := rodentTestWorld()
	w.time = DayDuration + 1
	r := mustRodent(t, w, "blind_albino_rat", "", p.x+10, p.z)
	w.tickGroundRodent(r, .05)
	if r.target != "" {
		t.Fatal("blind rat heard quiet walking")
	}
	p.sprinting = true
	w.tickGroundRodent(r, .05)
	if r.target != "one" {
		t.Fatal("blind rat did not hear sprinting")
	}
	p.sprinting = false
	p.x = r.X - 40
	w.tickGroundRodent(r, .05)
	if r.target != "one" {
		t.Fatal("blind rat stopped before reaching fifty metres")
	}
	p.x = r.X - 50.1
	w.tickGroundRodent(r, .05)
	if r.target != "" {
		t.Fatal("blind rat kept fleeing beyond fifty metres")
	}
}

func TestFlyingSquirrelGlideAngleAndClosestTreeReturn(t *testing.T) {
	w, p := rodentTestWorld()
	w.nodes = []*Node{{ID: "near", Kind: "wood", X: -90, Z: -100, Available: true, Scale: 1}, {ID: "far", Kind: "wood", X: -180, Z: -100, Available: true, Scale: 1}}
	r := mustRodent(t, w, "flying_squirrel", "", -100, -100)
	r.X, r.Y, r.Z, r.State = -100, 10, -100, "perched"
	p.x, p.z = -105, -100
	w.tickFlyingSquirrel(r, .25)
	if r.State != "gliding" || math.Abs(r.X+98) > 1e-8 || math.Abs(r.Y-(10-2*math.Tan(20*math.Pi/180))) > 1e-8 {
		t.Fatalf("wrong glide: %+v", r.RodentView)
	}
	for i := 0; i < 80 && r.State == "gliding"; i++ {
		w.time += .1
		w.tickFlyingSquirrel(r, .1)
	}
	if r.Y != 0 || r.treeID != "near" || r.State != "wandering" {
		t.Fatal("glider did not land and select nearest tree")
	}
	w.time = r.stateUntil + 1
	x, z := r.X, r.Z
	w.tickFlyingSquirrel(r, .1)
	if math.Abs(distance(x, z, r.X, r.Z)-.6) > 1e-8 {
		t.Fatal("returning glider did not run at untrained walking speed")
	}
}

func TestBeaverStaysAtEasternOceanEdgeAndDivesBeforeCatch(t *testing.T) {
	w, p := rodentTestWorld()
	w.time = DayDuration + 1
	r := mustRodent(t, w, "beaver", "", -3, 100)
	p.x, p.z = -2, 100
	for i := 0; i < 100; i++ {
		w.moveRodent(r, 1, -1)
	}
	if r.X < -8 || r.X > -.5 || !waterAt(r.X, r.Z) {
		t.Fatal("beaver walked out of the eastern ocean strip")
	}
	p.x, p.z, p.jumpOffset = r.X+1, r.Z, .5
	if err := w.catchRodent(p, r.ID); err == nil || r.State != "diving" || r.Y >= 0 {
		t.Fatal("nearby jump did not make the beaver dive before capture")
	}
	p.jumpOffset = 0
	w.time += 8.1
	w.diveBeaver(r)
	if err := w.catchRodent(p, r.ID); err != nil {
		t.Fatal(err)
	}
}

func TestHamsterDailyWindowAndOtterThreeToFiveDaySchedule(t *testing.T) {
	w, _ := rodentTestWorld()
	w.time = DayDuration - 300 - .01
	w.spawnAmbientRodents()
	if w.rodentCount("hamster", "") != 0 {
		t.Fatal("hamster appeared early")
	}
	w.time = DayDuration - 300
	w.spawnAmbientRodents()
	if w.rodentCount("hamster", "") != 1 {
		t.Fatal("daily hamster did not appear beside Betty")
	}
	var hamster *rodent
	for _, r := range w.rodents.entities {
		if r.Species == "hamster" {
			hamster = r
		}
	}
	delete(w.rodents.entities, hamster.ID)
	w.spawnAmbientRodents()
	if w.rodentCount("hamster", "") != 0 {
		t.Fatal("caught hamster respawned the same day")
	}
	w.time = DayDuration + NightDuration + DayDuration - 200
	w.spawnAmbientRodents()
	if w.rodentCount("hamster", "") != 1 {
		t.Fatal("next day's hamster missing")
	}
	w.time = 2*DayDuration + NightDuration
	w.tickRodents(.05)
	for _, r := range w.rodents.entities {
		if r.Species == "hamster" && r.State != "burrowing" {
			t.Fatal("hamster did not burrow at nightfall")
		}
	}
	w.time += 2
	w.tickRodents(.05)
	if w.rodentCount("hamster", "") != 0 {
		t.Fatal("hamster remained above ground at night")
	}
	firstDay := w.rodents.otterNextDay
	if firstDay < 3 || firstDay > 5 {
		t.Fatalf("first otter day %d", firstDay)
	}
	w.time = float64(firstDay)*(DayDuration+NightDuration) + DayDuration - 1
	w.spawnAmbientRodents()
	if w.rodentCount("otter", "") != 0 {
		t.Fatal("otter appeared during daytime")
	}
	w.time++
	w.spawnAmbientRodents()
	if w.rodentCount("otter", "") != 1 {
		t.Fatal("scheduled nighttime otter missing")
	}
	if next := w.rodents.otterNextDay - firstDay; next < 3 || next > 5 {
		t.Fatal("otter schedule outside three to five days")
	}
}

func TestOtterClosestChaserFoodOrderAndGlobalMinuteRainbow(t *testing.T) {
	w, p := rodentTestWorld()
	w.time = DayDuration + 1
	p.x, p.z = -150, -150
	w.Join("near", "Near")
	near := w.players["near"]
	near.x, near.z, near.sprinting = 20, -150, true
	r := mustRodent(t, w, "otter", "", 100, -150)
	w.tickGroundRodent(r, .01)
	if math.Abs(r.speed-9.9*1.5) > 1e-8 {
		t.Fatal("otter did not use closest chaser beyond sixty metres")
	}
	w.hitRodent(&Projectile{Item: "nut_cookie"}, r.ID)
	w.tickGroundRodent(r, .01)
	if !r.nutFed || math.Abs(r.speed-9.9*1.2) > 1e-8 {
		t.Fatal("nut cookie did not reduce otter speed to 1.2 times chaser")
	}
	w.hitRodent(&Projectile{Item: "cosmic_cake"}, r.ID)
	w.tickGroundRodent(r, .01)
	if r.State != "confused" || math.Abs(r.speed-9.9*.9) > 1e-8 || r.stateUntil != r.expires {
		t.Fatal("fed cosmic otter did not circle at .9 until the night ends")
	}
	w.hitRodent(&Projectile{Item: "nut_cookie"}, r.ID)
	if r.State != "confused" {
		t.Fatal("extra nut cookie undid cosmic confusion")
	}
	delete(w.rodents.entities, r.ID)
	r = mustRodent(t, w, "otter", "", p.x+2, p.z)
	w.hitRodent(&Projectile{Item: "cosmic_cake"}, r.ID)
	if r.State != "rainbow" || w.rodents.rainbowUntil != w.time+60 {
		t.Fatal("cosmic-first did not launch the rainbow escape")
	}
	p.profile.HomeIsland = &Island{Owner: "one"}
	p.island = "one"
	s := w.Snapshot("one")
	if s.RodentWorld.RainbowUntil != w.time+60 || len(s.Rodents) != 0 {
		t.Fatal("global rainbow did not reach an isolated home or leaked mainland wildlife")
	}
	w.time += 30
	w.tickRodents(.1)
	if r.Y <= 0 {
		t.Fatal("rainbow otter did not ascend")
	}
	w.time += 30
	w.tickRodents(.1)
	if w.rodents.entities[r.ID] != nil || w.rodents.otterNextDay < 3 || w.rodents.rainbowUntil != 0 {
		t.Fatal("rainbow did not expire and delay the next otter")
	}
}

func TestOtterCatchRequiresBothFoodsAndSpawnCoversWholeMap(t *testing.T) {
	w, p := rodentTestWorld()
	w.time = DayDuration + 1
	r := mustRodent(t, w, "otter", "", p.x+2, p.z)
	if err := w.Act("one", Action{Action: "catch_rodent", Target: r.ID}); err == nil {
		t.Fatal("fresh otter could be caught without either food")
	}
	w.hitRodent(&Projectile{Item: "nut_cookie"}, r.ID)
	if err := w.Act("one", Action{Action: "catch_rodent", Target: r.ID}); err == nil {
		t.Fatal("fed otter could be caught without cosmic cake")
	}
	if s := w.Snapshot("one"); len(s.Rodents) != 1 || !s.Rodents[0].NutFed {
		t.Fatal("snapshot does not report completed nut-cookie stage")
	}
	w.hitRodent(&Projectile{Item: "funny_cake"}, r.ID)
	if err := w.catchRodent(p, r.ID); err == nil {
		t.Fatal("ordinary funny cake substituted for required cosmic cake")
	}
	w.hitRodent(&Projectile{Item: "cosmic_cake"}, r.ID)
	if err := w.Act("one", Action{Action: "catch_rodent", Target: r.ID}); err != nil {
		t.Fatal(err)
	}
	if p.profile.Inventory["pet_otter"] != 1 {
		t.Fatal("correct two-food sequence did not permit capture")
	}
	regions := map[string]bool{}
	for i := 0; i < 60; i++ {
		w.rodents.entities = map[string]*rodent{}
		w.rodents.otterNextDay = 0
		w.spawnAmbientRodents()
		for _, a := range w.rodents.entities {
			if a.Species == "otter" {
				region := area(a.X, a.Z)
				if waterAt(a.X, a.Z) {
					region = "ocean"
				}
				regions[region] = true
			}
		}
	}
	for _, region := range []string{"forest", "desert", "city", "ocean"} {
		if !regions[region] {
			t.Fatalf("whole-map otter spawning never visited %s", region)
		}
	}
	for _, point := range []Point{{MinX - 1, 0}, {MaxX + 1, 0}, {0, MinZ - 1}, {0, MaxZ + 1}} {
		if w.rodentPositionFree("", point.X, 0, point.Z, true) {
			t.Fatal("wild animal could leave playable world bounds")
		}
	}
}

func TestKitchenRatRequiresRealDroppedCookieAndNeverSpawnsPassively(t *testing.T) {
	w, p := rodentTestWorld()
	plot := w.layout.Plots[0]
	p.profile.Bakery, p.profile.BakeryPlot = true, plot.ID
	w.drops = map[string]groundDrop{}
	p.x, p.z = plot.X, plot.Z-3
	w.rodentDropped(p, "nut_cookie")
	if w.rodentCount("rat", "") != 0 {
		t.Fatal("rat appeared without dropped food")
	}
	addBait := func(x, z float64) {
		w.drops["bait"] = groundDrop{node: Node{ID: "bait", Item: "nut_cookie", Amount: 1, X: x, Z: z}, expires: w.now().Add(time.Minute)}
	}
	p.x, p.z = -100, -100
	addBait(p.x, p.z)
	w.rodentDropped(p, "nut_cookie")
	if w.rodentCount("rat", "") != 0 {
		t.Fatal("rat appeared from food outside a kitchen")
	}
	p.x, p.z = plot.X, plot.Z+3
	addBait(p.x, p.z)
	w.rodentDropped(p, "nut_cookie")
	if w.rodentCount("rat", "") != 0 {
		t.Fatal("cafe bait spawned a kitchen rat")
	}
	p.x, p.z = plot.X, plot.Z-3
	addBait(p.x, p.z)
	w.rodentDropped(p, "nut_cookie")
	if w.rodentCount("rat", "") != 1 {
		t.Fatal("valid dropped nut cookie did not attract a common kitchen rat")
	}
	w.rodentDropped(p, "nut_cookie")
	if w.rodentCount("rat", "") != 1 {
		t.Fatal("drop spam bypassed the rat attraction cooldown")
	}
	delete(w.drops, "bait")
	w.rodents.entities = map[string]*rodent{}
	for i := 0; i < 30; i++ {
		w.time += 20
		w.rodents.nextSpawn = 0
		w.tickRodents(0)
	}
	if w.rodentCount("rat", "") != 0 {
		t.Fatal("bakery ownership passively spawned unbaited rats")
	}
}

func TestRodentHomeHabitatPopulationExpiryAndSnapshotOwnership(t *testing.T) {
	w, p := rodentTestWorld()
	p.profile.HomeIsland = &Island{Owner: "one", Objects: []BuildObject{{ID: "flowers", Kind: "flowers", X: 10, Z: 5}}}
	p.island = "one"
	for i := 0; i < 20; i++ {
		w.time += 20
		w.rodents.nextSpawn = 0
		w.tickRodents(0)
	}
	if w.rodentCount("house_mouse", "one") == 0 || w.rodentCount("house_mouse", "one") > 2 {
		t.Fatal("home flowers did not produce a bounded mouse population")
	}
	s := w.Snapshot("one")
	if len(s.Rodents) == 0 {
		t.Fatal("home mouse omitted from snapshot")
	}
	for _, r := range s.Rodents {
		if r.Island != "one" {
			t.Fatal("snapshot leaked another world")
		}
	}
	s.Rodents[0].Species = "tampered"
	s.RodentCatalog[0].Name = "tampered"
	if fresh := w.Snapshot("one"); fresh.Rodents[0].Species == "tampered" || fresh.RodentCatalog[0].Name == "tampered" {
		t.Fatal("snapshot aliases world or catalog")
	}
	for i := 0; i < 100; i++ {
		w.spawnRodent("wood_mouse", "", -120, -100)
	}
	if w.rodentCount("wood_mouse", "") > 8 || len(w.rodents.entities) > maxWildRodents {
		t.Fatal("population cap exceeded")
	}
	for _, r := range w.rodents.entities {
		r.expires = w.time
	}
	w.rodents.nextSpawn = math.Inf(1)
	w.tickRodents(.05)
	if len(w.rodents.entities) != 0 {
		t.Fatal("expired wildlife did not despawn")
	}
}

func TestClubCakesAreRealProjectilesWithoutDamageOrCoinRewards(t *testing.T) {
	for _, item := range []string{"funny_cake", "cosmic_cake"} {
		t.Run(item, func(t *testing.T) {
			w, p := rodentTestWorld()
			w.Join("target", "Target")
			victim := w.players["target"]
			victim.x, victim.z = p.x, p.z-4
			victim.buffs = map[string]float64{}
			p.profile.Hotbar[0] = Stack{item, 2}
			syncInventory(&p.profile)
			p.buffs["superstrength"] = w.time + 20
			p.yaw, p.pitch = 0, 0
			coins, health := p.profile.Coins, victim.health
			if err := w.Act("one", Action{Action: "throw", Item: item}); err != nil {
				t.Fatal(err)
			}
			w.moveProjectiles(.2)
			cake, _ := clubCakeFor(item)
			if len(w.projectiles) != 0 || victim.buffs["funny_cake"] != w.time+cake.Duration || victim.health != health || p.profile.Coins != coins || p.profile.Inventory[item] != 1 {
				t.Fatal("cake projectile failed to consume exactly one, apply effect, or suppress combat rewards")
			}
			if _, ok := recipeFor(item); ok {
				t.Fatal("throwable cake accidentally became craftable")
			}
		})
	}
}

func TestRodentRealSweptFoodImpactsRespectWallsAndFeedOrder(t *testing.T) {
	w, p := rodentTestWorld()
	w.time = DayDuration + 1
	r := mustRodent(t, w, "otter", "", p.x, p.z-4)
	p.profile.Hotbar[0] = Stack{"nut_cookie", 2}
	p.profile.Hotbar[1] = Stack{"cosmic_cake", 1}
	syncInventory(&p.profile)
	p.yaw, p.pitch = 0, math.Atan2(-1.15, 4)
	w.propertyColliders = []Collider{{ID: "wall", X: p.x, Z: p.z - 2, Width: 5, Depth: .2, Height: 3}}
	if err := w.Act("one", Action{Action: "throw", Item: "nut_cookie"}); err != nil {
		t.Fatal(err)
	}
	w.moveProjectiles(.3)
	if r.nutFed {
		t.Fatal("food projectile passed through a wall")
	}
	w.propertyColliders = nil
	w.time++
	if err := w.Act("one", Action{Action: "throw", Item: "nut_cookie"}); err != nil {
		t.Fatal(err)
	}
	w.moveProjectiles(.3)
	if !r.nutFed {
		t.Fatal("swept nut projectile missed the otter")
	}
	w.time++
	if err := w.Act("one", Action{Action: "throw", Item: "cosmic_cake"}); err != nil {
		t.Fatal(err)
	}
	w.moveProjectiles(.3)
	if r.State != "confused" || p.profile.Inventory["cosmic_cake"] != 0 || w.rodents.rainbowUntil != 0 {
		t.Fatal("correct projectile food order did not produce catchable circles")
	}
}

func TestGatherRodentTriggersRequireCompletedResourceGain(t *testing.T) {
	for _, kind := range []string{"wood", "stick", "cactus"} {
		t.Run(kind, func(t *testing.T) {
			w, p := rodentTestWorld()
			n := &Node{ID: "resource", Kind: kind, X: p.x + 1, Z: p.z, Available: true, Amount: 3, ChopTotal: 3, ChopRemaining: 1, respawn: 30}
			w.nodes = append(w.nodes, n)
			// Select a reproducible first roll below every source's spawn chance.
			for seed := uint64(0); ; seed++ {
				rng := rand.New(rand.NewPCG(seed, 29))
				if rng.Float64() < .05 {
					w.rodents.rng = rand.New(rand.NewPCG(seed, 29))
					break
				}
			}
			mustAct(t, w, Action{Action: "interact", Target: n.ID})
			species := "wood_mouse"
			if kind == "cactus" {
				species = "cactus_mouse"
			}
			if w.rodentCount(species, "") != 1 {
				t.Fatal("successful resource collection did not invoke wildlife trigger")
			}
		})
	}
}

func TestGatherRodentsUsuallyLeaveSuccessfulResourcesUndisturbed(t *testing.T) {
	for _, kind := range []string{"wood", "stick", "cactus"} {
		t.Run(kind, func(t *testing.T) {
			w, p := rodentTestWorld()
			n := &Node{ID: "quiet_resource", Kind: kind, X: p.x + 1, Z: p.z, Available: true, Amount: 3, ChopTotal: 3, ChopRemaining: 1, respawn: 30}
			w.nodes = append(w.nodes, n)
			for seed := uint64(0); ; seed++ {
				rng := rand.New(rand.NewPCG(seed, 29))
				if rng.Float64() > .8 {
					w.rodents.rng = rand.New(rand.NewPCG(seed, 29))
					break
				}
			}
			before := 0
			for _, count := range p.profile.Inventory {
				before += count
			}
			mustAct(t, w, Action{Action: "interact", Target: n.ID})
			after := 0
			for _, count := range p.profile.Inventory {
				after += count
			}
			if n.Available || after <= before {
				t.Fatal("quiet encounter roll prevented successful resource gathering")
			}
			if len(w.rodents.entities) != 0 {
				t.Fatal("successful gathering must support a no-animal outcome")
			}
		})
	}
}

func TestGatherRodentEncountersAreOccasionalAndRarityOrdered(t *testing.T) {
	const attempts = 20000
	for _, tc := range []struct {
		kind string
		rate float64
	}{{"wood", .20}, {"stick", .15}, {"cactus", .10}} {
		t.Run(tc.kind, func(t *testing.T) {
			w, p := rodentTestWorld()
			w.rodents.rng = rand.New(rand.NewPCG(301, 509))
			n := &Node{Kind: tc.kind, X: p.x, Z: p.z}
			counts := map[string]int{}
			encounters := 0
			for i := 0; i < attempts; i++ {
				clear(w.rodents.entities)
				w.rodentGather(p, n)
				if len(w.rodents.entities) > 1 {
					t.Fatal("one gathering action produced multiple animals")
				}
				for _, r := range w.rodents.entities {
					counts[r.Species]++
					encounters++
				}
			}
			if rate := float64(encounters) / attempts; math.Abs(rate-tc.rate) > .015 {
				t.Fatalf("encounter rate %.3f, want approximately %.3f", rate, tc.rate)
			}
			if tc.kind == "wood" {
				ordered := []string{"wood_mouse", "squirrel", "chipmunk", "mole_rat", "flying_squirrel"}
				for i := 1; i < len(ordered); i++ {
					if counts[ordered[i]] == 0 || counts[ordered[i-1]] <= counts[ordered[i]] {
						t.Fatalf("rarer animals were not less frequent: %v", counts)
					}
				}
			}
		})
	}
}

func TestAmbientRodentsHaveQuietOutcomesAndShareGatheringSpace(t *testing.T) {
	w, p := rodentTestWorld()
	w.nodes = []*Node{{ID: "forest_tree", Kind: "wood", Available: true, X: p.x, Z: p.z}}
	w.rodents.rng = rand.New(rand.NewPCG(719, 127))
	const attempts = 4000
	encounters := 0
	for i := 0; i < attempts; i++ {
		clear(w.rodents.entities)
		w.rodents.nextSpawn = 0
		w.spawnAmbientRodents()
		encounters += len(w.rodents.entities)
	}
	if rate := float64(encounters) / attempts; rate < .17 || rate > .23 {
		t.Fatalf("ambient forest encounters should also be occasional, rate %.3f", rate)
	}
	clear(w.rodents.entities)
	r := w.spawnRodentEncounter("wood_mouse", p.x, p.z)
	if r == nil {
		t.Fatal("first encounter failed")
	}
	for i := 0; i < 100; i++ {
		w.rodents.nextSpawn = 0
		w.spawnAmbientRodents()
		w.rodentGather(p, w.nodes[0])
	}
	if len(w.rodents.entities) != 1 {
		t.Fatal("ambient spawning and gathering crowded the same tree")
	}
	r.X += 20
	if w.spawnRodentEncounter("chipmunk", p.x, p.z) != nil {
		t.Fatal("fleeing animal caused its spawn location to refill immediately")
	}
	if w.spawnRodentEncounter("chipmunk", r.X+1, r.Z) != nil {
		t.Fatal("new encounter crowded an existing animal's current location")
	}
	r.expires = w.time
	if w.spawnRodentEncounter("chipmunk", p.x, p.z) == nil {
		t.Fatal("expired encounter permanently blocked its old location")
	}
}
