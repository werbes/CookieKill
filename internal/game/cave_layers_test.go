package game

import (
	"math"
	"strings"
	"testing"
)

func TestCaveStairsHaveNinetySixQuarterMetreTreads(t *testing.T) {
	for step := 0; step < CaveStairCount; step++ {
		z := CaveStairTopZ - (float64(step)+.5)*CaveStairRun
		want := -float64(step+1) * CaveStairRise
		for _, offset := range []float64{-.15, 0, .15} {
			if got := caveGroundY(120, z+offset); math.Abs(got-want) > 1e-8 {
				t.Fatalf("tread %d is not flat: got %v, want %v", step, got, want)
			}
		}
	}
}

func TestCoveredCaveRoofIsFreelyWalkableSurface(t *testing.T) {
	w, p := testWorld()
	// Keep the complete cave; remove unrelated cacti to inspect its actual roof.
	caveColliders := []Collider{}
	for _, c := range w.colliders {
		if strings.HasPrefix(c.ID, "cave_") && c.resource == nil && c.Y+c.Height <= 0 {
			caveColliders = append(caveColliders, c)
		}
	}
	w.colliders = caveColliders
	for _, route := range []struct{ x, z, dx, dz float64 }{{103, -119, 1, 0}, {113, -133, 0, 1}, {120, -119, 1, 0}, {120, -106, 1, 0}} {
		p.x, p.z, p.inCave = route.x, route.z, false
		for step := 0; step < 110; step++ {
			beforeX, beforeZ := p.x, p.z
			w.Input("one", Input{X: route.dx, Z: route.dz})
			w.Tick(.05)
			if math.Abs(distance(p.x, p.z, beforeX, beforeZ)-.3) > 1e-7 || p.inCave || playerY(p) != 0 {
				t.Fatalf("surface route %+v hit buried geometry at (%v,%v), y=%v, cave=%v", route, p.x, p.z, playerY(p), p.inCave)
			}
		}
	}
	p.x, p.z = 111, -124
	snapshot := w.Snapshot("one")
	if snapshot.Me.InCave || snapshot.Me.Y != 0 || snapshot.Me.Zone == "Sunbaked clay caves" {
		t.Fatal("standing above the chamber was reported as underground")
	}
	if err := w.Act("one", Action{Action: "eat_lollipop", Item: "pink"}); err == nil {
		t.Fatal("surface player ate the lollipop through the desert roof")
	}
}

func TestUndergroundWallsBlockBelowButNotAboveTheDesert(t *testing.T) {
	w, p := testWorld()
	for _, point := range []Point{{106.6, -121}, {133.4, -121}, {120, -124}, {120, -129.4}} {
		if w.solidBlockedAt(point.X, 0, point.Z, .42) {
			t.Fatalf("buried cave wall blocks the surface at %+v", point)
		}
		if !w.solidBlockedAt(point.X, CaveFloorY, point.Z, .42) {
			t.Fatalf("underground wall is missing at %+v", point)
		}
	}
	p.x, p.z, p.inCave = 108, -121, true
	for step := 0; step < 20; step++ {
		w.Input("one", Input{X: -1, Sprint: true, Jump: true})
		w.Tick(.05)
	}
	if p.x < 107.4 || !p.inCave || playerY(p) >= 0 {
		t.Fatal("sprint-jumping escaped an underground wall into surface space")
	}
	if got := w.Snapshot("one").Me; !got.InCave || got.Zone != "Sunbaked clay caves" {
		t.Fatal("underground layer missing from player snapshot")
	}
}

func TestCaveLayerIsClearedBySurfaceTeleportsAndRespawn(t *testing.T) {
	w, p := testWorld()
	p.inCave = true
	w.setIsland(p, "one")
	if p.inCave {
		t.Fatal("home teleport retained underground layer")
	}
	p.inCave = true
	w.setIsland(p, "")
	if p.inCave || groundY(p) != 0 {
		t.Fatal("surface teleport retained underground layer")
	}
	p.inCave = true
	w.spawn(p)
	if p.inCave || groundY(p) != 0 {
		t.Fatal("forest respawn retained underground layer")
	}
}

func TestRockyMouthSidesAreSolidAndWalkingOutReturnsToDesert(t *testing.T) {
	for _, direction := range []float64{-1, 1} {
		w, p := testWorld()
		p.x, p.z, p.inCave = 120, CaveMouthZ-1, true
		for step := 0; step < 20; step++ {
			w.Input("one", Input{X: direction, Jump: true})
			w.Tick(.05)
		}
		// The horizontal mouth flares wider than the stairs. Test the player
		// capsule against that aperture, then prove its next step hits solid rock.
		if !p.inCave || math.Abs(p.x-120)+.42 > caveHalfWidth(p.z) {
			t.Fatalf("jump clipped through the visible rock cheek: (%v,%v) cave=%v", p.x, playerY(p), p.inCave)
		}
		if !w.solidBlockedAt(p.x+direction*.3, playerY(p), p.z, .42) {
			t.Fatal("flared entrance stopped movement without a matching solid rock cheek")
		}
		stoppedX := p.x
		for step := 0; step < 10; step++ {
			w.Input("one", Input{X: direction})
			w.Tick(.05)
		}
		if p.x != stoppedX {
			t.Fatal("walking after landing passed through the rock cheek")
		}
		for step := 0; step < 15; step++ {
			w.Input("one", Input{Z: 1})
			w.Tick(.05)
		}
		if p.inCave || groundY(p) != 0 || p.z < CaveMouthZ {
			t.Fatal("walking out of the horizontal mouth did not return to desert")
		}
	}
}

func TestCampfirePlacementAndBakingRespectCaveLayer(t *testing.T) {
	w, p := testWorld()
	p.x, p.z, p.inCave = 120, -117, true
	for _, item := range []string{"wood", "stick", "stone", "dough"} {
		setItem(p, item, 10)
	}
	nodes := len(w.nodes)
	if err := w.Act("one", Action{Action: "craft", Item: "fire"}); err == nil || !strings.Contains(err.Error(), "outside the clay cave") {
		t.Fatalf("cave campfire should explain where to build instead: %v", err)
	}
	if len(w.nodes) != nodes || p.profile.Inventory["wood"] != 10 || p.profile.Inventory["stone"] != 10 || p.profile.Inventory["stick"] != 10 {
		t.Fatal("rejected underground campfire spent materials or created a surface fire")
	}
	p.inCave = false
	mustAct(t, w, Action{Action: "craft", Item: "fire"})
	if len(w.nodes) != nodes+1 {
		t.Fatal("campfire placement on the desert surface should still work")
	}
	advance(w, 1.1)
	p.inCave = true
	if err := w.Act("one", Action{Action: "craft", Item: "sugar"}); err == nil {
		t.Fatal("underground player baked using a campfire through the cave roof")
	}
}
