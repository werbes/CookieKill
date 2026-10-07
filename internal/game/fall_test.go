package game

import (
	"math"
	"reflect"
	"testing"
)

func TestFallRollsAtEveryFiftyMetresWithExactOdds(t *testing.T) {
	w, p := movementWorld()
	rolls := 0
	w.fallRandom = func() int { rolls++; return 999 }
	w.recordFootTravel(p, 49.999)
	if rolls != 0 {
		t.Fatal("rolled before fifty metres")
	}
	w.recordFootTravel(p, .001)
	if rolls != 1 || math.Abs(p.fallDistance) > 1e-8 {
		t.Fatalf("boundary rolls=%d distance=%v", rolls, p.fallDistance)
	}
	w.recordFootTravel(p, 102.5)
	if rolls != 3 || math.Abs(p.fallDistance-2.5) > 1e-8 {
		t.Fatalf("lost a completed interval or its remainder: rolls=%d distance=%v", rolls, p.fallDistance)
	}
	for _, tc := range []struct {
		drinks, draw int
		falls        bool
	}{{0, 0, true}, {0, 1, false}, {1, 10, true}, {1, 11, false}, {2, 20, true}, {2, 21, false}, {99, 990, true}, {99, 991, false}, {100, 999, true}} {
		resetFall(p)
		p.alcoholDrinks = tc.drinks
		w.fallRandom = func() int { return tc.draw }
		w.recordFootTravel(p, 50)
		if p.fallen != tc.falls {
			t.Fatalf("drinks=%d draw=%d fallen=%v", tc.drinks, tc.draw, p.fallen)
		}
	}
}

func TestFallDistanceUsesCollisionResolvedFootTravel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input Input
		wall  bool
	}{{"walking", Input{X: 1}, false}, {"diagonal", Input{X: 1, Z: 1}, false}, {"sprinting", Input{X: 1, Sprint: true}, false}, {"blocked", Input{X: 1}, true}, {"sliding", Input{X: 1, Z: 1}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			w, p := movementWorld()
			if tc.wall {
				w.colliders = []Collider{{ID: "wall", X: 11, Z: -20, Width: 1, Depth: 20, Height: 3}}
			}
			x, z := p.x, p.z
			w.Input("one", tc.input)
			w.Tick(.05)
			travel := distance(x, z, p.x, p.z)
			if math.Abs(p.fallDistance-travel) > 1e-8 {
				t.Fatalf("counted input instead of resolved travel: distance=%v actual=%v", p.fallDistance, travel)
			}
			if tc.name == "blocked" && travel != 0 || tc.name != "blocked" && travel <= 0 {
				t.Fatalf("invalid movement fixture: %v", travel)
			}
		})
	}
	w, p := movementWorld()
	w.fallRandom = func() int { return 0 }
	p.fallDistance = 49.8
	w.Input("one", Input{X: 1, Sprint: true})
	w.Tick(.05)
	if !p.fallen || p.x > 10.5 || p.posture != "lying" || p.sprinting {
		t.Fatalf("crossing fifty metres failed to stop movement immediately: x=%v posture=%q", p.x, p.posture)
	}
}

func TestFallDistanceExcludesSwimmingRidingJumpingAndTeleports(t *testing.T) {
	for _, mode := range []string{"swim", "camel", "jump"} {
		t.Run(mode, func(t *testing.T) {
			w, p := movementWorld()
			switch mode {
			case "swim":
				p.x, p.z = -100, 100
			case "camel":
				p.ridingCamel, p.profile.CamelOwned = true, true
			case "jump":
				p.jumpOffset, p.jumpVelocity = 1, 1
			}
			p.fallDistance = 49.9
			w.fallRandom = func() int { t.Fatal("ineligible travel rolled"); return 0 }
			x := p.x
			w.Input("one", Input{X: 1})
			w.Tick(.05)
			if p.x <= x || p.fallDistance != 49.9 || p.fallen {
				t.Fatalf("%s counted as foot travel, or fixture did not move: %+v", mode, p)
			}
		})
	}
	w, p := movementWorld()
	buyTestIsland(t, w, p)
	p.alcoholDrinks, p.fallDistance = 4, 49.9
	w.fallRandom = func() int { t.Fatal("teleport rolled"); return 0 }
	mustAct(t, w, Action{Action: "teleport_home"})
	w.Tick(.05)
	p.x, p.z = islandPortalX, islandPortalZ
	mustAct(t, w, Action{Action: "teleport_cave"})
	w.Tick(.05)
	if p.fallDistance != 49.9 || p.alcoholDrinks != 4 {
		t.Fatal("teleport changed foot travel or alcohol")
	}
}

func TestFallenPlayerCannotBypassRestUsingInputOrActions(t *testing.T) {
	w, p := movementWorld()
	buyTestIsland(t, w, p)
	p.x, p.z, p.inCave = 10, -20, false
	w.fallRandom = func() int { return 0 }
	w.recordFootTravel(p, 50)
	x, z := p.x, p.z
	profile := cloneProfile(p.profile)
	for _, action := range []string{"stand", "teleport_home", "teleport_main", "teleport_cave", "visit_island", "club_sit", "eat", "drink", "interact", "throw", "camel_mount", "build_mode", "select", "move_item", "drop_item", "club_gamble"} {
		if err := w.Act("one", Action{Action: action, Item: "sugar", Target: "club_seat_1"}); err == nil {
			t.Fatalf("%s bypassed fall recovery", action)
		}
	}
	if !reflect.DeepEqual(profile, cloneProfile(p.profile)) {
		t.Fatal("rejected action changed profile")
	}
	for i := 0; i < 39; i++ {
		w.Input("one", Input{X: 1, Jump: true, Sprint: true})
		w.Tick(.05)
		if p.x != x || p.z != z || !grounded(p) || !p.fallen || p.posture != "lying" || p.sprinting {
			t.Fatalf("input bypassed two-second rest at t=%v", w.time)
		}
	}
	w.Input("one", Input{})
	w.Tick(.1)
	if !p.fallen {
		t.Fatal("stood automatically without stand input")
	}
	if err := w.Act("one", Action{Action: "teleport_home"}); err == nil {
		t.Fatal("teleport replaced the required stand action")
	}
	mustAct(t, w, Action{Action: "stand"})
	if p.fallen || p.posture != "" {
		t.Fatal("stand did not restore posture")
	}
	mustAct(t, w, Action{Action: "teleport_home"})
}

func TestMovementAndSpaceStandAfterTwoSeconds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input Input
	}{{"move", Input{X: 1}}, {"space", Input{Jump: true}}} {
		t.Run(tc.name, func(t *testing.T) {
			w, p := movementWorld()
			w.fallRandom = func() int { return 0 }
			w.recordFootTravel(p, 50)
			w.time = p.standAt - .001
			if err := w.Act("one", Action{Action: "stand"}); err == nil {
				t.Fatal("stood before the exact recovery boundary")
			}
			w.Input("one", tc.input)
			w.Tick(.05)
			if p.fallen || p.posture != "" || !grounded(p) {
				t.Fatal("stand input failed or Space also jumped")
			}
			if tc.name == "move" && p.x <= 10 {
				t.Fatal("movement did not resume after standing")
			}
		})
	}
}

func TestOnlySuccessfullyConsumedAlcoholRaisesFallChance(t *testing.T) {
	w, p := movementWorld()
	for _, drink := range clubDrinks {
		setItem(p, drink.ID, 2)
		before := p.alcoholDrinks
		mustAct(t, w, Action{Action: "drink", Item: drink.ID})
		want := before
		if drink.Alcoholic {
			want++
		}
		if p.alcoholDrinks != want {
			t.Fatalf("wrong alcohol effect for %s: %d", drink.ID, p.alcoholDrinks)
		}
		if err := w.Act("one", Action{Action: "drink", Item: drink.ID}); err == nil || p.alcoholDrinks != want || p.profile.Inventory[drink.ID] != 1 {
			t.Fatal("cooldown rejection changed alcohol or consumed a drink")
		}
		w.time += 1
	}
	before := p.alcoholDrinks
	setItem(p, "beer", 0)
	if err := w.Act("one", Action{Action: "drink", Item: "beer"}); err == nil || p.alcoholDrinks != before {
		t.Fatal("missing drink increased odds")
	}
	setItem(p, "beer", 2)
	p.alcoholDrinks = 99
	mustAct(t, w, Action{Action: "eat", Item: "beer"})
	w.time++
	mustAct(t, w, Action{Action: "drink", Item: "beer"})
	if me := w.Snapshot("one").Me; me.FallChance != 100 || me.AlcoholDrinks != 100 {
		t.Fatalf("alcohol cap or self odds incorrect: chance=%v drinks=%d", me.FallChance, me.AlcoholDrinks)
	}
}

func TestFallStateSurvivesReconnectAndClearsOnRespawn(t *testing.T) {
	w, p := movementWorld()
	p.alcoholDrinks = 7
	w.fallRandom = func() int { return 0 }
	w.recordFootTravel(p, 50.5)
	w.Join("two", "Observer")
	me := w.Snapshot("one").Me
	if !me.Fallen || me.StandIn != 2 || me.FallChance != 7.1 || me.Posture != "lying" {
		t.Fatalf("fall snapshot contract missing: %+v", me)
	}
	if w.Snapshot("two").Players[0].Posture != "lying" {
		t.Fatal("other players cannot see the fallen posture")
	}
	w.Leave("one")
	w.Join("one", "Returning")
	p = w.players["one"]
	if !p.fallen || p.fallDistance != .5 || p.alcoholDrinks != 7 || p.posture != "lying" {
		t.Fatal("reconnect cleared runtime fall state")
	}
	if err := w.Act("one", Action{Action: "stand"}); err == nil {
		t.Fatal("reconnect bypassed the rest timer")
	}
	p.buffs = map[string]float64{}
	if !w.damagePlayer(nil, p, 1000, "a cookie") {
		t.Fatal("respawn fixture did not die")
	}
	me = w.Snapshot("one").Me
	if me.Fallen || me.StandIn != 0 || me.FallChance != .1 || me.AlcoholDrinks != 0 || p.fallDistance != 0 || me.Posture != "" {
		t.Fatalf("respawn did not clear alcohol and falls: %+v", me)
	}
}
