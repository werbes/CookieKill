package game

import (
	"math"
	"testing"
)

func movementWorld() (*World, *player) {
	w, p := testWorld()
	w.colliders, w.propertyColliders, w.animals = nil, nil, nil
	p.x, p.z = 10, -20
	return w, p
}

func TestExhaustedSprintStaysAtWalkingSpeedUntilReleaseAndRecovery(t *testing.T) {
	w, p := movementWorld()
	p.stamina = .1
	for i := 0; i < 60; i++ {
		before := p.x
		w.Input("one", Input{X: 1, Sprint: true})
		w.Tick(.05)
		if math.Abs(p.x-before-.3) > 1e-8 || p.sprinting || !p.sprintExhausted {
			t.Fatalf("exhausted sprint accelerated while Shift remained held: step=%d distance=%v stamina=%v", i, p.x-before, p.stamina)
		}
	}
	if p.stamina < 20 {
		t.Fatal("walking should recover stamina even while Shift is held")
	}
	w.Input("one", Input{})
	w.Tick(.05)
	before := p.x
	w.Input("one", Input{X: 1, Sprint: true})
	w.Tick(.05)
	if !p.sprinting || p.sprintExhausted || math.Abs(p.x-before-.495) > 1e-8 {
		t.Fatal("recovered stamina and a new Shift press did not restore sprint")
	}
	if got := w.Snapshot("one").Me; !got.Sprinting || got.SprintExhausted {
		t.Fatal("snapshot does not report actual sprint state")
	}
}

func TestExhaustionRequiresMeaningfulStaminaRecovery(t *testing.T) {
	w, p := movementWorld()
	p.profile.Levels["stamina"] = 5
	p.stamina = 0
	w.Input("one", Input{Sprint: true})
	w.Tick(.05)
	w.Input("one", Input{})
	w.Tick(.05)
	w.Input("one", Input{X: 1, Sprint: true})
	w.Tick(.05)
	if p.sprinting || !p.sprintExhausted {
		t.Fatal("a brief Shift release bypassed exhausted stamina recovery")
	}
	p.stamina = 29
	w.Input("one", Input{})
	w.Tick(.05)
	if !p.sprintExhausted {
		t.Fatal("stamina upgrade did not scale the recovery threshold")
	}
	w.Tick(.15)
	if p.sprintExhausted {
		t.Fatal("holding Shift released after 20% recovery did not clear exhaustion")
	}
}

func TestJumpTapIsQueuedAndHoldingSpaceDoesNotRepeat(t *testing.T) {
	w, p := movementWorld()
	w.Input("one", Input{Jump: true})
	w.Input("one", Input{})
	w.Tick(.05)
	if playerY(p) <= 0 || grounded(p) {
		t.Fatal("Space press/release between simulation ticks lost the jump")
	}
	peak := playerY(p)
	for i := 0; i < 25; i++ {
		w.Input("one", Input{Jump: true})
		w.Tick(.05)
		peak = max(peak, playerY(p))
	}
	if peak < 1.15 || peak > 1.25 || !grounded(p) || playerY(p) != 0 {
		t.Fatalf("jump did not make one modest arc and land: peak=%v offset=%v", peak, p.jumpOffset)
	}
	w.Input("one", Input{})
	w.Tick(.15)
	w.Input("one", Input{Jump: true})
	w.Tick(.05)
	if grounded(p) {
		t.Fatal("a fresh Space press after landing did not jump")
	}
}

func TestJumpCannotRestartInMidairOrDuringLandingCooldown(t *testing.T) {
	w, p := movementWorld()
	w.Input("one", Input{Jump: true})
	w.Tick(.25)
	w.Input("one", Input{})
	w.Input("one", Input{Jump: true})
	w.Tick(.1)
	want := jumpSpeed*.35 - jumpGravity*.35*.35/2
	if math.Abs(playerY(p)-want) > 1e-8 {
		t.Fatal("pressing Space in midair changed the jump arc")
	}
	w.Tick(.4)
	if !grounded(p) {
		t.Fatal("jump did not land")
	}
	w.Input("one", Input{})
	w.Input("one", Input{Jump: true})
	w.Tick(.05)
	if !grounded(p) {
		t.Fatal("landing cooldown was bypassed")
	}
}

func TestJumpFollowsWorldHeightOnCaveRampAndLands(t *testing.T) {
	w, p := movementWorld()
	p.x, p.z = 120, -105
	start := playerY(p)
	w.Input("one", Input{Z: -1, Jump: true})
	w.Tick(.3)
	want := start + jumpSpeed*.3 - jumpGravity*.3*.3/2
	if math.Abs(playerY(p)-want) > 1e-8 || groundY(p) >= start {
		t.Fatalf("ramp dragged jump along its surface: got=%v want=%v ground=%v", playerY(p), want, groundY(p))
	}
	w.Input("one", Input{})
	advance(w, 1.5)
	if !grounded(p) || playerY(p) != caveGroundY(p.x, p.z) {
		t.Fatal("jump did not land on the lowered cave ramp")
	}
	if got := w.Snapshot("one").Me; !got.Grounded || got.Y != groundY(p) {
		t.Fatal("snapshot did not report cave landing")
	}
}

func TestJumpOnHomeIslandUpperFloor(t *testing.T) {
	w, p := movementWorld()
	p.island = "one"
	p.x, p.y, p.z = 0, 3.2, 0
	p.profile.HomeIsland = &Island{Owner: "one", Objects: []BuildObject{{ID: "upper", Kind: "floor", Y: 3}}}
	w.Input("one", Input{Jump: true})
	w.Tick(.3)
	if playerY(p) < 4.3 || p.y != 3.2 {
		t.Fatalf("jump lost the raised island floor: surface=%v player=%v", p.y, playerY(p))
	}
	advance(w, 1)
	if !grounded(p) || playerY(p) != 3.2 {
		t.Fatal("jump did not land on the island floor")
	}
}

func TestJumpCannotPassThroughUpperFloorOrThinWall(t *testing.T) {
	t.Run("ceiling", func(t *testing.T) {
		w, p := movementWorld()
		p.island = "one"
		p.x, p.z = 0, 0
		p.profile.HomeIsland = &Island{Owner: "one", Objects: []BuildObject{{ID: "ceiling", Kind: "floor", Y: 2.1}}}
		for i := 0; i < 20; i++ {
			w.Input("one", Input{Jump: true})
			w.Tick(.05)
			if playerY(p)+1.7 > 2.100001 {
				t.Fatal("jump passed through the underside of an upper floor")
			}
		}
		if !grounded(p) || p.y != 0 {
			t.Fatal("ceiling impact did not return to the original floor")
		}
	})
	t.Run("wall", func(t *testing.T) {
		w, p := movementWorld()
		w.colliders = []Collider{{ID: "wall", X: 10, Z: -21, Width: 8, Depth: .2, Height: 3}}
		p.buffs["speed"] = w.time + 5
		for i := 0; i < 20; i++ {
			w.Input("one", Input{Z: -1, Sprint: true, Jump: true})
			w.Tick(.05)
		}
		if p.z < -20.49 {
			t.Fatal("sprinting jump tunneled through a thin wall")
		}
	})
}

func TestJumpIsUnavailableWhileSwimmingOrRiding(t *testing.T) {
	for _, swimming := range []bool{true, false} {
		w, p := movementWorld()
		if swimming {
			p.x, p.z = -100, 100
		} else {
			p.ridingCamel, p.profile.CamelOwned = true, true
		}
		w.Input("one", Input{Jump: true})
		w.Tick(.2)
		if !grounded(p) {
			t.Fatalf("unsupported jump while swimming=%v riding=%v", swimming, p.ridingCamel)
		}
	}
}

func TestJumpHeightIsUsedForViewsAndCombat(t *testing.T) {
	w, p := movementWorld()
	w.Join("two", "Observer")
	w.Input("one", Input{Jump: true})
	w.Tick(.3)
	y := playerY(p)
	if w.Snapshot("one").Me.Y != y || w.Snapshot("two").Players[0].Y != y {
		t.Fatal("jump height missing from self or remote avatar view")
	}
	if err := w.throw(p, "sugar"); err != nil {
		t.Fatal(err)
	}
	if math.Abs(w.projectiles[0].Y-(y+1.45)) > 1e-8 {
		t.Fatal("airborne throw spawned at ground level")
	}
	shot := &Projectile{X: p.x - 2, Y: y + 1.68, Z: p.z}
	if _, coins, hit := playerHit(shot, p.x+2, shot.Y, p.z, p); !hit || coins != 5 {
		t.Fatal("hit volumes did not follow the airborne player's head")
	}
}

func TestTeleportClearsJumpMotionWithoutReusingHeldKeys(t *testing.T) {
	w, p := movementWorld()
	p.profile.HomeIsland = &Island{Owner: "one"}
	p.stamina = .1
	w.Input("one", Input{X: 1, Sprint: true, Jump: true})
	w.Tick(.1)
	if grounded(p) || !p.sprintExhausted {
		t.Fatal("test must start jumping with exhausted stamina")
	}
	mustAct(t, w, Action{Action: "teleport_home"})
	if got := w.Snapshot("one").Me; !got.Grounded || got.Y != 0 || got.Sprinting || !got.SprintExhausted {
		t.Fatal("teleport did not clear airborne motion or preserve exhaustion")
	}
	// The browser can send another held-key packet before the teleport snapshot.
	for i := 0; i < 3; i++ {
		before := p.x
		w.Input("one", Input{X: 1, Sprint: true, Jump: true})
		w.Tick(.05)
		if !grounded(p) || p.sprinting || math.Abs(p.x-before-.3) > 1e-8 {
			t.Fatal("teleport reused held Space or bypassed exhausted walking")
		}
	}
	w.Input("one", Input{})
	w.Input("one", Input{Jump: true})
	w.Tick(.05)
	if grounded(p) {
		t.Fatal("Space did not work after a real release at the destination")
	}
}

func TestStaleInputAndReconnectDoNotReplayHeldJump(t *testing.T) {
	w, p := movementWorld()
	w.Input("one", Input{Jump: true})
	advance(w, 1)
	if !grounded(p) {
		t.Fatal("jump did not finish after input stopped")
	}
	w.Input("one", Input{Jump: true})
	w.Tick(.05)
	if !grounded(p) {
		t.Fatal("a delayed held-key packet was mistaken for a new Space press")
	}
	w.Leave("one")
	w.Join("one", "Baker")
	w.Input("one", Input{Jump: true})
	w.Tick(.05)
	if !grounded(p) {
		t.Fatal("reconnecting replayed a held Space press")
	}
	w.Input("one", Input{})
	w.Input("one", Input{Jump: true})
	w.Tick(.05)
	if grounded(p) {
		t.Fatal("a new Space press was lost after reconnecting")
	}
}
