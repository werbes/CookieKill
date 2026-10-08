package game

import (
	"math"
	"reflect"
	"testing"
)

func practiceWorld() (*World, *player) {
	w, p := testWorld()
	p.island = CookiePracticeIsland
	p.x, p.z, p.y = 0, 20, 0
	p.profile.Coins = 100
	p.cookieArena = &cookieArenaSession{}
	w.cookiePracticeEnter(p)
	return w, p
}
func mountPractice(t *testing.T, w *World, p *player, id string) {
	t.Helper()
	station, _ := practiceStation(id)
	p.x, p.z, p.y = station.X, station.Z+station.Depth/2+.6, 0
	p.practice.lastX, p.practice.lastZ = p.x, p.z
	mustAct(t, w, Action{Action: "practice_exercise", Target: id})
}
func practiceInputs(w *World, p *player, input Input, seconds float64) {
	for elapsed := 0.; elapsed < seconds-.00001; elapsed += .05 {
		w.Input(p.profile.ID, input)
		w.Tick(.05)
	}
}

func TestPracticeTreadmillRequiresFreshForwardInputAndChargesCompletedSet(t *testing.T) {
	w, p := practiceWorld()
	mountPractice(t, w, p, "practice_treadmill")
	advance(w, 25)
	if p.practice.progress != 0 || p.profile.Coins != 100 || p.profile.Levels["stamina"] != 0 {
		t.Fatal("idle treadmill granted progress")
	}
	practiceInputs(w, p, Input{X: 1}, 2)
	practiceInputs(w, p, Input{Z: 1}, 2)
	if p.practice.progress != 0 {
		t.Fatal("sideways/backward intent counted as running")
	}
	w.Input("one", Input{Z: -1})
	advance(w, 2)
	if p.practice.progress <= 0 || p.practice.progress > .5001 {
		t.Fatal("stale movement kept treadmill running")
	}
	before := p.practice.progress
	practiceInputs(w, p, Input{X: -1, Yaw: math.Pi / 2}, 1)
	if math.Abs(p.practice.progress-before-1) > .001 {
		t.Fatal("looking sideways stopped real forward-key treadmill work")
	}
	practiceInputs(w, p, Input{Z: -1}, 20)
	if p.profile.Levels["stamina"] != 1 || p.profile.Coins != 98 || p.x != -16 || p.z != 6 || p.posture != "treadmill" {
		t.Fatalf("treadmill completion/position incorrect: %+v", p.practice)
	}
	if !cookiePracticeMovementLocked(p) {
		t.Fatal("mounted treadmill did not lock world movement")
	}
	mustAct(t, w, Action{Action: "practice_stop"})
	if cookiePracticeMovementLocked(p) || cookiePracticeBlocked(p, p.x, p.z) || p.practice.progress != 0 {
		t.Fatal("stepping off left player trapped or retained partial set")
	}
}

func TestPracticePressesRequireKeyEdgesCadenceProteinRecoveryAndCap(t *testing.T) {
	w, p := practiceWorld()
	mountPractice(t, w, p, "practice_bench_press")
	p.profile.Levels["arms"] = 9
	w.Input("one", Input{Jump: true})
	practiceInputs(w, p, Input{Jump: true}, 4)
	if p.practice.progress != 1 {
		t.Fatal("holding Space repeated a press")
	}
	for i := 1; i < 10; i++ {
		w.Input("one", Input{})
		w.Input("one", Input{Jump: true})
		w.Tick(.8)
	}
	if p.profile.Levels["arms"] != 10 || p.profile.Coins != 98 {
		t.Fatal("controlled bench set did not grant exactly one paid level")
	}
	for i := 0; i < 15; i++ {
		w.Input("one", Input{})
		w.Input("one", Input{Jump: true})
		w.Tick(1)
	}
	if p.profile.Levels["arms"] != 10 || p.profile.Coins != 98 {
		t.Fatal("maximum level spent coins or advanced beyond10")
	}
	mustAct(t, w, Action{Action: "practice_stop"})
	mountPractice(t, w, p, "practice_leg_press")
	p.buffs["protein"] = w.time + 60
	for i := 0; i < 10; i++ {
		w.Input("one", Input{})
		w.Input("one", Input{Jump: true})
		w.Tick(.45)
	}
	if p.profile.Levels["legs"] != 1 || p.profile.Coins != 96 || p.cooldowns["train"]-w.time > 1.5001 {
		t.Fatal("protein did not support faster controlled reps and recovery")
	}
}

func TestPracticeOccupancyDistanceAndInsufficientFundsAreAuthoritative(t *testing.T) {
	w, p := practiceWorld()
	if err := w.Act("one", Action{Action: "practice_exercise", Target: "practice_bench_press"}); err == nil {
		t.Fatal("remote equipment mount accepted")
	}
	mountPractice(t, w, p, "practice_bench_press")
	w.Join("two", "Visitor")
	other := w.players["two"]
	other.island = CookiePracticeIsland
	other.cookieArena = &cookieArenaSession{}
	other.x, other.z = -8, 8.1
	w.cookiePracticeEnter(other)
	if err := w.Act("two", Action{Action: "practice_exercise", Target: "practice_bench_press"}); err == nil {
		t.Fatal("two players occupied one press")
	}
	p.profile.Coins = 1
	for i := 0; i < 10; i++ {
		mustAct(t, w, Action{Action: "practice_rep"})
		w.Tick(.8)
	}
	if p.profile.Coins != 1 || p.profile.Levels["arms"] != 0 || p.practice.progress != 10 {
		t.Fatal("unaffordable set charged or granted a level")
	}
	p.profile.Coins = 2
	mustAct(t, w, Action{Action: "practice_rep"})
	if p.profile.Coins != 0 || p.profile.Levels["arms"] != 1 {
		t.Fatal("completed paid set did not redeem on the next real repetition")
	}
	w.cookiePracticeLeave(p)
	if p.practice != nil {
		t.Fatal("leaving practice retained mounted equipment")
	}
}

func practiceSwimTo(t *testing.T, w *World, p *player, x, z float64) {
	t.Helper()
	for i := 0; i < 700 && distance(p.x, p.z, x, z) > .2; i++ {
		dx, dz := x-p.x, z-p.z
		d := max(1, math.Hypot(dx, dz))
		w.Input(p.profile.ID, Input{X: dx / d, Z: dz / d})
		w.Tick(.05)
	}
	if distance(p.x, p.z, x, z) > .3 {
		t.Fatalf("pool route blocked at %.2f,%.2f toward %.2f,%.2f", p.x, p.z, x, z)
	}
}
func TestPracticePoolRequiresActualOrderedLengthsInOneLane(t *testing.T) {
	w, p := practiceWorld()
	p.x, p.z = 10, 15.5
	p.practice.lastX, p.practice.lastZ = p.x, p.z
	advance(w, 30)
	if p.profile.Levels["swim"] != 0 || p.practice.station != "" {
		t.Fatal("idling in pool trained swimming")
	}
	practiceSwimTo(t, w, p, 10, 13)
	if p.practice.station != "practice_pool" || p.practice.poolStage != 0 || !inWater(p) || playerY(p) != -.6 {
		t.Fatal("physical pool lane did not start swimming")
	}
	practiceSwimTo(t, w, p, 10, -5)
	if p.profile.Levels["swim"] != 0 || p.practice.poolStage != 1 {
		t.Fatal("single length incorrectly completed round trip")
	}
	practiceSwimTo(t, w, p, 10, 13)
	if p.profile.Levels["swim"] != 1 || p.profile.Coins != 98 {
		t.Fatal("ordered swimming lengths did not earn one paid level")
	}
	practiceSwimTo(t, w, p, 14, 13)
	if p.practice.poolStage != 0 || p.practice.progress > 0 {
		t.Fatal("switching lanes retained unordered progress")
	}
}

func TestPracticeTargetsUseSweptServerGeometryAndNoEconomyReward(t *testing.T) {
	w, p := practiceWorld()
	before := cloneProfile(p.profile)
	shot := &Projectile{Owner: "one", Item: "sugar", Island: CookiePracticeIsland, X: -16, Y: 1.8, Z: -20}
	targetT, target, hit := w.cookiePracticeShotHit(shot, -16, 1.8, -22)
	if !hit || target != "practice_target_0" || targetT != .5 {
		t.Fatal("real target crossing was not detected")
	}
	if _, _, hit := w.cookiePracticeShotHit(shot, -14, 1.8, -22); hit {
		t.Fatal("near miss counted as a target hit")
	}
	w.hitCookiePracticeTarget(p, target, shot)
	if p.practice.hits != 1 || p.profile.Coins != before.Coins || !reflect.DeepEqual(p.profile.Inventory, before.Inventory) {
		t.Fatal("practice hit changed personal economy or failed to count")
	}
	x := w.practiceTargets()[3].X
	w.time += 2
	if w.practiceTargets()[3].X == x {
		t.Fatal("moving target stayed still")
	}
	view := Snapshot{}
	w.cookiePracticeSnapshot(p, &view)
	if view.CookiePractice == nil || len(view.CookiePractice.Targets) != 4 || len(view.CookiePractice.Stations) != 4 || view.CookiePractice.Hits != 1 || view.CookiePractice.Fee != 2 {
		t.Fatal("practice snapshot incomplete")
	}
	view.CookiePractice.Targets[0].X = 999
	if w.practiceTargets()[0].X != -16 {
		t.Fatal("target snapshot aliases geometry")
	}
}
