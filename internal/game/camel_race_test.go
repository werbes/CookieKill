package game

import (
	"math"
	"reflect"
	"testing"
)

func raceWorld(t *testing.T, count int) (*World, []*player) {
	t.Helper()
	w, p := testWorld()
	players := []*player{p}
	for i := 1; i < count; i++ {
		id := string(rune('a' + i))
		w.Join(id, "Racer "+id)
		players = append(players, w.players[id])
	}
	for _, p := range players {
		p.x, p.z, p.inCave = -227, 20, false
		if err := w.Act(p.profile.ID, Action{Action: "race_enter", Target: "camel_race_lollipop"}); err != nil {
			t.Fatal(err)
		}
	}
	w.time = CamelRaceFirstStart - CamelRaceGather
	w.tickCamelRace(0)
	return w, players
}

func TestCamelRaceScheduleCutoffAndRegistrationValidation(t *testing.T) {
	for _, tc := range []struct{ now, start float64 }{
		{0, 120}, {99.999, 120}, {100, 1200}, {119, 1200}, {120, 1200},
		{1179.999, 1200}, {1180, 2280}, {2280, 3360},
	} {
		if got := nextCamelRaceStart(tc.now); got != tc.start {
			t.Fatalf("time %v: next start=%v, want %v", tc.now, got, tc.start)
		}
	}
	for _, setup := range []func(*player){
		func(p *player) { p.x = -220 },
		func(p *player) { p.inCave = true },
		func(p *player) { p.island = "one" },
		func(p *player) { p.jumpOffset = 4 },
	} {
		w, p := testWorld()
		p.x, p.z = -227, 20
		setup(p)
		if err := w.Act("one", Action{Action: "race_enter", Target: "camel_race_lollipop"}); err == nil || w.raceRegistrations["one"] != 0 {
			t.Fatal("invalid remote or wrong-layer signup succeeded")
		}
	}
	w, p := testWorld()
	p.x, p.z = -227, 20
	mustAct(t, w, Action{Action: "interact", Target: "camel_race_lollipop"})
	if err := w.Act("one", Action{Action: "race_enter", Target: "camel_race_lollipop"}); err == nil || w.raceRegistrations["one"] != 120 {
		t.Fatal("repeat signup was accepted")
	}
	mustAct(t, w, Action{Action: "race_leave"})
	w.time = 100 // No simulation tick yet: cutoff still applies.
	mustAct(t, w, Action{Action: "race_enter", Target: "camel_race_lollipop"})
	w.tickCamelRace(0)
	if w.raceRegistrations["one"] != 1200 || p.island == CamelRaceIsland {
		t.Fatal("late signup entered the frozen field")
	}
}

func TestCamelRaceGatherCountdownIsolationAndDetachedSnapshot(t *testing.T) {
	w, players := raceWorld(t, 2)
	p := players[0]
	if w.camelRace.entrantCount != 2 || !p.ridingCamel || p.island != CamelRaceIsland || p.raceCamel == nil || len(p.profile.Camels) != 0 {
		t.Fatal("gather did not provide isolated loan mounts")
	}
	x, z := p.x, p.z
	for i := 0; i < 19; i++ {
		w.Input("one", Input{X: -1, Z: 1, Sprint: true, Jump: true})
		w.Tick(1)
	}
	if p.x != x || p.z != z || !grounded(p) {
		t.Fatal("countdown did not freeze the starting grid")
	}
	for _, a := range []Action{{Action: "teleport_main"}, {Action: "camel_dismount"}, {Action: "eat", Item: "sugar"}, {Action: "drop_item", From: "hotbar", FromSlot: 0}} {
		if err := w.Act("one", a); err == nil {
			t.Fatalf("race allowed escaping its authoritative rules: %+v", a)
		}
	}
	w.Join("observer", "Mainland visitor")
	before := w.Snapshot("one")
	if len(before.Nodes) != 0 || before.Island != nil || len(before.Players) != 1 || before.Players[0].Island != CamelRaceIsland || before.Me.Zone != "Camel Cove ring" || before.CamelRace.OwnCamel == nil {
		t.Fatal("arena snapshot leaked mainland or home-island state")
	}
	for _, other := range w.Snapshot("observer").Players {
		if other.ID == "one" {
			t.Fatal("mainland saw racing player")
		}
	}
	changed := w.SnapshotUpdate("one")
	changed.CamelRace.Entrants[0].Name = "Tampered"
	changed.CamelRace.OwnCamel.Speed = 999
	changed.CamelRace.Track.OuterX = 999
	if !reflect.DeepEqual(before, w.Snapshot("one")) {
		t.Fatal("race snapshot aliases authoritative state")
	}
	w.time = 120
	w.Input("one", Input{Z: 1})
	w.Tick(.2)
	if p.z <= z {
		t.Fatal("race did not release the grid at its start")
	}
}

func raceSteer(w *World, p *player, forward bool, sprint bool) {
	angle := camelRaceAngle(p.x, p.z)
	step := .07
	if !forward {
		step = -step
	}
	x, z := 51.5*math.Cos(angle+step), 33.5*math.Sin(angle+step)
	dx, dz := x-p.x, z-p.z
	d := math.Hypot(dx, dz)
	w.Input(p.profile.ID, Input{X: dx / d, Z: dz / d, Sprint: sprint})
}

func TestCamelRaceOrderedThreeLapsShortcutsWrongWayAndSingleEntrantPrize(t *testing.T) {
	w, players := raceWorld(t, 1)
	p := players[0]
	entry := w.camelRace.entries["one"]
	w.time = 120
	// Driving at the infield cannot cut through its collision boundary.
	for i := 0; i < 100; i++ {
		w.Input("one", Input{X: -1000})
		w.Tick(.05)
	}
	if !camelRaceTrackContains(p.x, p.z) || p.x < 43 || entry.checkpoints != 0 {
		t.Fatal("infield shortcut advanced the race or crossed its barrier")
	}
	// A complete lap backwards cannot grant checkpoint credit.
	for entry.progress > -2*math.Pi-.3 {
		raceSteer(w, p, false, false)
		w.Tick(.05)
		if w.time > 165 {
			t.Fatal("test driver could not make a backward lap")
		}
	}
	if entry.checkpoints != 0 || entry.Rank != 0 || p.profile.CamelCoins != 0 {
		t.Fatal("backwards lap granted race progress or prize")
	}
	beforeCoins := p.profile.Coins
	for entry.Status == "racing" && w.time < 350 {
		raceSteer(w, p, true, true)
		w.Tick(.05)
		if entry.checkpoints < CamelRaceCheckpoints*CamelRaceLaps && entry.Rank != 0 {
			t.Fatal("race finished before all ordered gates")
		}
	}
	if entry.Status != "finished" || entry.Lap != 3 || entry.checkpoints != 48 || entry.Rank != 1 || entry.Coins != 0 || entry.CamelCoins != 50 || p.profile.Coins != beforeCoins || p.profile.CamelCoins != 50 {
		t.Fatalf("incorrect solo result: %+v at time%v", entry.CamelRaceEntryView, w.time)
	}
	w.finishCamelRacer(p, entry, w.time+1)
	if p.profile.CamelCoins != 50 {
		t.Fatal("duplicate finish paid twice")
	}
	mustAct(t, w, Action{Action: "race_leave"})
	if p.x != -227 || p.z != 20 || p.island != "" || p.ridingCamel || p.raceCamel != nil {
		t.Fatal("leaving finished race did not restore origin and remove loan")
	}
}

func TestCamelRacePrizeFieldFrozenAndOnlyTopThreePaid(t *testing.T) {
	w, players := raceWorld(t, 5)
	w.Join("late", "Late racer")
	late := w.players["late"]
	late.x, late.z = -227, 20
	if err := w.Act("late", Action{Action: "race_enter", Target: "camel_race_lollipop"}); err != nil {
		t.Fatal(err)
	}
	w.time = 125
	// Quitting does not shrink the purse for the already frozen field.
	if err := w.Act(players[4].profile.ID, Action{Action: "race_leave"}); err != nil {
		t.Fatal(err)
	}
	for i, p := range players[:4] {
		before := p.profile.Coins
		w.finishCamelRacer(p, w.camelRace.entries[p.profile.ID], 130+float64(i))
		wantCoins := []int{800, 400, 200, 0}[i]
		wantCamel := []int{50, 30, 10, 0}[i]
		if p.profile.Coins != before+wantCoins || p.profile.CoinsEarned != wantCoins || p.profile.CamelCoins != wantCamel {
			t.Fatalf("rank%d wrong purse: coins%d earned%d camel%d", i+1, p.profile.Coins-before, p.profile.CoinsEarned, p.profile.CamelCoins)
		}
	}
	if w.camelRace.entrantCount != 5 || w.raceRegistrations["late"] != 1200 || late.island != "" {
		t.Fatal("late signup or quit altered frozen race field")
	}
	w.tickCamelRace(0)
	w.time += 21
	w.tickCamelRace(0)
	for _, p := range players {
		if p.island == CamelRaceIsland {
			t.Fatal("completed race did not automatically return racers")
		}
	}
	for _, p := range players[:3] {
		saved := New(w.Profiles()).Profiles()[p.profile.ID]
		if saved.CamelCoins != p.profile.CamelCoins || saved.CoinsEarned != p.profile.CoinsEarned || saved.Coins != p.profile.Coins {
			t.Fatal("race winnings did not survive restart")
		}
	}
}

func TestCamelRaceOwnedTraitsSprintJumpAndReturnPreserveCamel(t *testing.T) {
	w, p := testWorld()
	p.x, p.z, p.yaw, p.pitch = -227, 20, .5, .3
	p.profile.Camels = []Camel{{ID: "one_camel_1", X: p.x, Z: p.z, Speed: 15, Jump: 9, Stamina: 180, Happiness: 100, TraitsVersion: 1}}
	p.ridingCamel, p.mountedCamelID = true, "one_camel_1"
	p.stamina = 72
	syncMountedCamel(p)
	beforeCamel := p.profile.Camels[0]
	mustAct(t, w, Action{Action: "race_enter", Target: "camel_race_lollipop"})
	w.time = 100
	w.tickCamelRace(0)
	if p.raceCamel.ID != beforeCamel.ID || p.raceCamel.Speed != 15 || p.raceCamel.Jump != 9 || staminaCapacity(p) != 180 || p.stamina != 180 {
		t.Fatal("race did not copy the owned camel's traits")
	}
	w.time = 120
	beforeZ := p.z
	w.Input("one", Input{Z: 1, Sprint: true, Jump: true})
	w.Tick(.2)
	if p.z-beforeZ < 4.8 || p.stamina >= 180 || playerY(p) < 1 || p.profile.Camels[0] != beforeCamel {
		t.Fatalf("race traits/movement mutated owned parking or failed sprint/jump: delta%v stamina%v height%v", p.z-beforeZ, p.stamina, playerY(p))
	}
	mustAct(t, w, Action{Action: "race_leave"})
	if p.x != -227 || p.z != 20 || p.yaw != .5 || p.pitch != .3 || !p.ridingCamel || p.mountedCamelID != beforeCamel.ID || p.stamina != 72 || p.profile.Camels[0] != beforeCamel {
		t.Fatal("return failed to restore rider and owned camel")
	}
}

func TestCamelRaceDisconnectTimeoutAndOfflineRegistrations(t *testing.T) {
	w, players := raceWorld(t, 3)
	p := players[0]
	w.SetConnected("one", false)
	w.Tick(.05)
	if w.camelRace.entries["one"].Status != "dnf" || p.island != "" || p.profile.CamelCoins != 0 {
		t.Fatal("disconnected racer was not returned without a prize")
	}
	w.Leave("one")
	w.Join("one", "Returning racer")
	p = w.players["one"]
	if p.island == CamelRaceIsland || p.profile.CamelCoins != 0 {
		t.Fatal("reconnect resumed or paid an abandoned race")
	}
	w.Leave(players[1].profile.ID)
	if w.camelRace.entries[players[1].profile.ID].Status != "dnf" || players[1].island != "" {
		t.Fatal("logout did not restore original location")
	}
	w.time = 120 + CamelRaceTimeout
	w.tickCamelRace(0)
	if players[2].island != "" || w.camelRace.entries[players[2].profile.ID].Status != "dnf" {
		t.Fatal("race timeout left a racer stranded")
	}
	p.x, p.z = -227, 20
	mustAct(t, w, Action{Action: "race_enter", Target: "camel_race_lollipop"})
	w.Leave("one")
	w.time = 1180
	w.tickCamelRace(0)
	if w.camelRace != nil || w.raceRegistrations["one"] != 0 {
		t.Fatal("offline registration manufactured a prize-paying entrant")
	}
}

func TestCamelRaceCoveDryEntranceWallsRoofAndReservedScenery(t *testing.T) {
	w, p := testWorld()
	if waterAt(-227, 20) || groundY(p) != 0 {
		t.Fatal("race lollipop is not on dry ground")
	}
	p.x, p.z = -210, 21
	for _, target := range []Point{{-218, 21}, {-227, 20}} {
		for i := 0; i < 100 && distance(p.x, p.z, target.X, target.Z) > .2; i++ {
			dx, dz := target.X-p.x, target.Z-p.z
			d := math.Hypot(dx, dz)
			w.Input("one", Input{X: dx / d, Z: dz / d})
			w.Tick(.05)
		}
		if distance(p.x, p.z, target.X, target.Z) > .2 || p.inCave {
			t.Fatal("above-ground cove entrance obstructed or entered underground layer")
		}
	}
	mustAct(t, w, Action{Action: "race_enter", Target: "camel_race_lollipop"})
	for _, point := range []Point{{-238.5, 20}, {-227, 8.5}, {-227, 27.5}, {-216.5, 13}} {
		if !w.solidBlockedAt(point.X, 0, point.Z, .42) {
			t.Fatal("cove wall missing collision")
		}
	}
	if w.solidBlockedAt(-227, 0, 20, .42) || !w.solidBlockedAt(-227, 4.5, 20, .42) {
		t.Fatal("cove roof or walkable interior incorrect")
	}
	for _, prop := range w.layout.Props {
		if specialPlaceReserved(prop.X, prop.Z, 0) && (prop.Kind == "tree" || prop.Kind == "pine" || prop.Kind == "palm" || prop.Kind == "cactus" || prop.Kind == "rock") {
			t.Fatalf("procedural scenery obstructs new place: %+v", prop)
		}
	}
}
