package game

import (
	"math"
	"reflect"
	"testing"
	"time"
)

func coasterTestWorld() (*World, *player) {
	w, p := testWorld()
	w.time = 500
	p.x, p.y, p.z, p.yaw, p.pitch = rollerCoasterX, 0, rollerCoasterZ+3, .3, -.1
	p.stamina, p.health = 71, 83
	return w, p
}

func TestRollerCoasterGeometryAndStationAreClear(t *testing.T) {
	w, _ := coasterTestWorld()
	first, last := coasterTrack[0], coasterTrack[len(coasterTrack)-1]
	if distance(first.X, first.Z, last.X, last.Z) > 1e-8 || math.Abs(first.Y-last.Y) > 1e-8 || first.Y != 1.1 || first.T != 0 || last.T != 48 {
		t.Fatal("coaster is not a closed, timed loop with a station-level floor")
	}
	maxY, minPitch, maxPitch, maxBank := 0., 0., 0., 0.
	for _, point := range coasterTrack {
		maxY, minPitch, maxPitch, maxBank = max(maxY, point.Y), min(minPitch, point.Pitch), max(maxPitch, point.Pitch), max(maxBank, point.Bank)
		if !finite(point.Yaw) || !finite(point.Pitch) || !coasterReserved(point.X, point.Z, 0) || point.Y < 1 {
			t.Fatal("invalid track point or unreserved footprint")
		}
	}
	if maxY < 23 || minPitch > -.3 || maxPitch < .3 || maxBank < .2 {
		t.Fatal("track lacks a lift, steep drop, hill, or banked turn")
	}
	for z := -151.; z >= -163; z -= .5 {
		if w.solidBlockedAt(165, 0, z, .65) {
			t.Fatalf("station approach obstructed at z%v", z)
		}
	}
	for _, n := range w.nodes {
		if validChoice(n.Kind, "cactus", "wood", "chest", "stone") && coasterReserved(n.X, n.Z, .5) {
			t.Fatalf("procedural resource obstructs coaster: %+v", n)
		}
	}
}

func TestRollerCoasterBoardValidationAndConcurrentRiders(t *testing.T) {
	for _, setup := range []func(*player){
		func(p *player) { p.x += 20 }, func(p *player) { p.island = "one" },
		func(p *player) { p.inCave = true }, func(p *player) { p.jumpOffset = 1 },
		func(p *player) { p.y = 10 }, func(p *player) { p.fallen = true },
		func(p *player) { p.disconnected = true },
	} {
		w, p := coasterTestWorld()
		setup(p)
		if err := w.Act("one", Action{Action: "coaster_board", Target: "desert_coaster"}); err == nil || p.coaster != nil {
			t.Fatal("invalid layer, range, height, or state allowed boarding")
		}
	}
	w, p := coasterTestWorld()
	w.Join("two", "Second rider")
	other := w.players["two"]
	other.x, other.z = rollerCoasterX+2, rollerCoasterZ
	mustAct(t, w, Action{Action: "interact", Target: "desert_coaster"})
	if err := w.Act("two", Action{Action: "coaster_board", Target: "desert_coaster"}); err == nil {
		t.Fatal("second car overlapped the departing car")
	}
	w.time += 3
	if err := w.Act("two", Action{Action: "coaster_board", Target: "desert_coaster"}); err != nil {
		t.Fatal(err)
	}
	w.time += 12
	w.tickRollerCoaster()
	s := w.SnapshotUpdate("one")
	if s.RollerCoaster.Ride == nil || len(s.RollerCoaster.Cars) != 2 || len(s.RollerCoaster.Track) != 201 || s.Me.Posture != "coaster" || s.Me.Y != s.RollerCoaster.Ride.Y || p.y <= 1.1 || p.x == other.x {
		t.Fatal("multiple riders did not occupy independent authoritative cars")
	}
	before := w.SnapshotUpdate("one")
	s.RollerCoaster.Track[0].Y = 999
	s.RollerCoaster.Cars[0].X = 999
	s.RollerCoaster.Ride.PlayerName = "changed"
	if !reflect.DeepEqual(before.RollerCoaster, w.SnapshotUpdate("one").RollerCoaster) {
		t.Fatal("coaster snapshot aliases track, cars, or the personal ride")
	}
	other.island = "private_home"
	if view := w.Snapshot("two").RollerCoaster; len(view.Cars) != 0 || view.Ride != nil {
		t.Fatal("mainland coaster riders leaked into a different world")
	}
}

func TestRollerCoasterFullRidePreservesFootAndCamelState(t *testing.T) {
	for _, mounted := range []bool{false, true} {
		t.Run(map[bool]string{false: "foot", true: "camel"}[mounted], func(t *testing.T) {
			w, p := coasterTestWorld()
			if mounted {
				p.profile.Camels = []Camel{{ID: "one_camel_1", Name: "Daisy", X: p.x, Y: 0, Z: p.z, Yaw: p.yaw, Speed: 14, Jump: 8, Stamina: 180, Happiness: 92, TraitsVersion: 1}}
				p.ridingCamel, p.mountedCamelID = true, "one_camel_1"
			}
			before := cloneProfile(p.profile)
			x, z, yaw, stamina := p.x, p.z, p.yaw, p.stamina
			mustAct(t, w, Action{Action: "coaster_board", Target: "desert_coaster"})
			start := p.coaster.startedAt
			maxY := 0.
			for i := 0; i < 95; i++ {
				yaw := p.yaw
				w.Input("one", Input{X: -1, Z: 1, Jump: true, Sprint: true, Yaw: 2})
				if p.yaw != yaw {
					t.Fatal("input overrode the authoritative car heading between ticks")
				}
				w.Tick(.5)
				pose := coasterPose(w.time - start)
				if p.coaster == nil || math.Abs(p.x-pose.X) > 1e-8 || math.Abs(p.y-pose.Y) > 1e-8 || p.fallen || !grounded(p) || p.stamina != stamina {
					t.Fatal("normal movement, jump, stamina, or falling overrode the rail ride")
				}
				maxY = max(maxY, p.y)
			}
			w.time = start + 48
			w.tickRollerCoaster()
			if p.coaster != nil || p.x != x || p.y != 0 || p.z != z || p.yaw != yaw || p.stamina != stamina || p.health != 83 || p.ridingCamel != mounted || maxY < 23 || p.posture != "" {
				t.Fatal("full ride did not safely return the rider and camel to the station")
			}
			if !reflect.DeepEqual(p.profile.Camels, before.Camels) || !reflect.DeepEqual(p.profile.Inventory, before.Inventory) || p.profile.Coins != before.Coins || p.profile.CamelCoins != before.CamelCoins {
				t.Fatal("ride changed camel ownership, traits, parking, goods, or wallet")
			}
		})
	}
}

func TestRollerCoasterActionsProjectilesAndJohnCannotExploitRide(t *testing.T) {
	w, p := coasterTestWorld()
	w.time = DayDuration + 10
	w.Join("attacker", "Attacker")
	attacker := w.players["attacker"]
	coins := attacker.profile.Coins
	placeTestJohn(w, p, p.x+1, p.z)
	w.projectiles = []*Projectile{{ID: "old", Owner: "one"}}
	mustAct(t, w, Action{Action: "coaster_board", Target: "desert_coaster"})
	if w.johnPork != nil || w.johnEligible(p) || len(w.projectiles) != 0 {
		t.Fatal("boarding retained John or the rider's old projectiles")
	}
	for _, action := range []string{"throw", "camel_dismount", "camel_call", "teleport_main", "teleport_home", "race_casual_host", "eat", "drop_item", "build", "coaster_board"} {
		if err := w.Act("one", Action{Action: action, Item: "sugar", Target: "desert_coaster"}); err == nil {
			t.Fatalf("ride allowed unsafe action %s", action)
		}
	}
	p.buffs = map[string]float64{}
	if w.damagePlayer(attacker, p, 1000, "test") || p.health != 83 {
		t.Fatal("coaster rider took damage")
	}
	w.projectiles = []*Projectile{{ID: "incoming", Owner: "attacker", X: p.x - 1, Y: p.y + 1, Z: p.z, VX: 30, born: w.time, damage: 100, Item: "berry_cookie"}}
	w.moveProjectiles(.1)
	if p.health != 83 || attacker.profile.Coins != coins || p.buffs["slowed"] != 0 {
		t.Fatal("shooting a coaster rider earned rewards, damage, or debuffs")
	}
	mustAct(t, w, Action{Action: "coaster_leave"})
	if p.coaster != nil || p.y != 0 || distance(p.x, p.z, rollerCoasterX, rollerCoasterZ+3) != 0 {
		t.Fatal("early exit did not return safely to the boarding position")
	}
}

func TestRollerCoasterDisconnectWorldChangeAndRaceGatherClearRide(t *testing.T) {
	for _, exit := range []string{"disconnect", "logout", "home", "race", "respawn"} {
		t.Run(exit, func(t *testing.T) {
			w, p := coasterTestWorld()
			mustAct(t, w, Action{Action: "coaster_board", Target: "desert_coaster"})
			w.time += 20
			w.tickRollerCoaster()
			switch exit {
			case "disconnect":
				w.SetConnected("one", false)
				w.tickRollerCoaster()
			case "logout":
				w.Leave("one")
				w.Join("one", "Returned")
			case "home":
				p.profile.HomeIsland = &Island{Owner: "one", Visitors: true}
				w.setIsland(p, "one")
			case "race":
				w.raceRegistrations = map[string]float64{"one": 1200}
				w.time = 1180
				w.tickCamelRace(0)
				if p.island != CamelRaceIsland || p.coaster != nil {
					t.Fatal("scheduled race did not safely transfer the coaster rider")
				}
				mustAct(t, w, Action{Action: "race_leave"})
			case "respawn":
				w.spawn(p)
			}
			if p.coaster != nil || p.y != 0 || p.fallen {
				t.Fatal("session transition left a floating or active coaster rider")
			}
			if exit != "home" && exit != "respawn" && (p.x != rollerCoasterX || p.z != rollerCoasterZ+3) {
				t.Fatal("transition lost the safe station return location")
			}
		})
	}
}

func TestRollerCoasterRentalExpiresOnlyAfterSafeReturn(t *testing.T) {
	w, p := coasterTestWorld()
	now := time.Unix(1000, 0)
	w.now = func() time.Time { return now }
	p.profile.Camels = []Camel{{ID: "one_rental_1", Name: "Rental", X: p.x, Z: p.z, RentalUntil: 1002, Speed: 10, Jump: 6, Stamina: 125, Happiness: 80, TraitsVersion: 1}}
	p.profile.CamelRentalUntil = 1002
	p.ridingCamel, p.mountedCamelID = true, "one_rental_1"
	mustAct(t, w, Action{Action: "coaster_board", Target: "desert_coaster"})
	now = time.Unix(1003, 0)
	w.time += 20
	w.tickRollerCoaster()
	if p.coaster == nil || !p.ridingCamel || w.Snapshot("one").RollerCoaster.Ride.CamelID != "one_rental_1" {
		t.Fatal("rental vanished under its passenger during the ride")
	}
	mustAct(t, w, Action{Action: "coaster_leave"})
	if p.ridingCamel || len(p.profile.Camels) != 0 || p.y != 0 || p.island != "" {
		t.Fatal("expired rental survived the safe station return")
	}
}
