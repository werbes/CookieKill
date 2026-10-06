package game

import (
	"math"
	"testing"
)

func TestMapHasSeparatedPlotsClearingsAndCurvedCoast(t *testing.T) {
	w := New(nil)
	layout := w.StaticLayout()
	if layout.MaxX-layout.MinX < 400 || layout.MaxZ-layout.MinZ < 400 {
		t.Fatal("world was not expanded")
	}
	if len(layout.Plots) != 12 || len(layout.Zones) != 4 {
		t.Fatal("missing multiplayer plots or housing clearings")
	}
	seen := map[string]bool{}
	for _, p := range layout.Plots {
		if seen[p.ID] {
			t.Fatal("duplicate plot")
		}
		seen[p.ID] = true
		if w.staticBlocked(p.X, p.Z, .42) || w.staticBlocked(p.X, p.Z+p.Depth/2, .42) {
			t.Fatalf("plot %s has no accessible interior/door", p.ID)
		}
		for _, road := range layout.Paths {
			if circleRect(p.X, p.Z, 0.1, road.X, road.Z, road.Width+p.Width, road.Depth+p.Depth) {
				t.Fatalf("plot %s occupies street %s", p.ID, road.ID)
			}
		}
	}
	for _, z := range layout.Zones {
		if !w.safeSpace(z.X, z.Z, 8) {
			t.Fatalf("clearing %s cannot fit protected home", z.ID)
		}
		if z.Width*z.Depth > 2500 {
			t.Fatal("housing clearing consumes too much wilderness")
		}
		if zoneAt(z.X, z.Z) != z.Name {
			t.Fatal("missing zone notification label")
		}
	}
	if w.safeSpace(-35, -35, 8) {
		t.Fatal("safe home permitted outside designated clearing")
	}
	low, high := 1000., -1000.
	for _, p := range layout.Coast {
		low = math.Min(low, p.Z)
		high = math.Max(high, p.Z)
		if area(p.X-.01, p.Z-1) != "beach" || area(p.X-.01, p.Z+2) != "shallows" || area(p.X-.01, p.Z+20) != "deep sea" {
			t.Fatal("coastline and gameplay water disagree")
		}
	}
	if high-low < 15 {
		t.Fatal("coastline is still rectangular")
	}
	layout.Props[0].X = 99999
	if w.StaticLayout().Props[0].X == 99999 {
		t.Fatal("layout exposes mutable simulation state")
	}
}

func TestSceneryBlocksPlayersAndCookiesButDoorsStayOpen(t *testing.T) {
	w := New(nil)
	for _, c := range w.colliders {
		if !w.staticBlocked(c.X, c.Z, .42) {
			t.Fatalf("solid %s permits walking through", c.ID)
		}
	}
	// Saffron's house has a 3.2m doorway on its southern wall.
	if w.staticBlocked(32, -26.5, .42) {
		t.Fatal("village doorway blocked")
	}
	if _, hit := w.staticShotHit(32, 1.5, -24, 32, 1.5, -30); hit {
		t.Fatal("projectile blocked by open doorway")
	}
	if _, hit := w.staticShotHit(35, 1.5, -24, 35, 1.5, -30); !hit {
		t.Fatal("projectile passed through solid front wall")
	}
	w.Join("walker", "Walker")
	p := w.players["walker"]
	p.x, p.z = 35, -24
	for i := 0; i < 35; i++ {
		w.Input("walker", Input{Z: -1})
		w.Tick(.05)
	}
	if p.z < -26.5+.42 {
		t.Fatalf("player crossed village wall: %.2f", p.z)
	}
	// Test the solid furniture supplied to the same renderer coordinates.
	if !w.staticBlocked(42, 18, .42) || !w.staticBlocked(21.4, 69.5, .42) {
		t.Fatal("gym treadmill or bakery oven is not solid")
	}
}

func TestOceanPopulationGroupsHealthAndInjuryRates(t *testing.T) {
	w := New(nil)
	ids := map[string]bool{}
	for _, spec := range animalSpecs {
		groups := map[string][]*Animal{}
		count, injured := 0, 0
		fishSpeeds := map[float64]bool{}
		for _, a := range w.animals {
			if a.Species != spec.species {
				continue
			}
			count++
			if ids[a.ID] {
				t.Fatal("duplicate animal ID")
			}
			ids[a.ID] = true
			if a.Need != "" {
				injured++
			}
			if !waterAt(a.X, a.Z) {
				t.Fatalf("%s spawned on land", a.ID)
			}
			groups[a.GroupID] = append(groups[a.GroupID], a)
			if a.baseSpeed < spec.speedMin || a.baseSpeed > spec.speedMax {
				t.Fatal("species speed outside intended range")
			}
			if a.MaxHealth != math.Round(spec.health*a.Scale*a.Scale) {
				t.Fatal("larger animals do not get more health")
			}
			fishSpeeds[a.baseSpeed] = true
		}
		if count != spec.count || injured != int(math.Round(float64(count)*spec.injury)) {
			t.Fatalf("%s wrong population/injury rate: %d/%d", spec.species, injured, count)
		}
		for id, group := range groups {
			if len(group) < spec.minGroup || len(group) > spec.maxGroup {
				t.Fatalf("wrong group size %s: %d", id, len(group))
			}
			for _, a := range group {
				if a.baseSpeed != group[0].baseSpeed {
					t.Fatal("school does not share a swimming speed")
				}
			}
		}
		if spec.species == "fish" && len(fishSpeeds) < 2 {
			t.Fatal("all fish schools swim the same speed")
		}
	}
	for _, a := range w.animals {
		if a.Species == "whale" && a.MaxHealth < 300 {
			t.Fatal("whales need much greater health")
		}
	}
}

func TestAnimalsSwimAndJumpWithoutTurningWholeOceanInjured(t *testing.T) {
	w := New(nil)
	positions := map[string]Point{}
	for _, a := range w.animals {
		positions[a.ID] = Point{a.X, a.Z}
	}
	sawJump := false
	for i := 0; i < 400; i++ {
		w.time += .05
		w.moveAnimals(.05)
		for _, a := range w.animals {
			if !waterAt(a.X, a.Z) {
				t.Fatal("swimming animal left the ocean")
			}
			if a.Species == "dolphin" && a.Y > .1 {
				sawJump = true
				if math.Abs(a.Speed/a.baseSpeed-.75) > .001 {
					t.Fatal("jumping dolphin did not slow to 75 percent")
				}
			}
		}
	}
	if !sawJump {
		t.Fatal("no dolphins jumped")
	}
	for _, a := range w.animals {
		old := positions[a.ID]
		d := distance(old.X, old.Z, a.X, a.Z)
		if a.Need == "trapped" && d > .001 {
			t.Fatal("trapped turtle swam away")
		}
		if a.Need != "trapped" && d < .01 {
			t.Fatalf("%s remained stationary", a.ID)
		}
	}
	for i := 0; i < 12000; i++ {
		w.time += .05
		w.moveAnimals(.05)
	}
	for _, spec := range animalSpecs {
		injured := 0
		for _, a := range w.animals {
			if a.Species == spec.species && a.Need != "" {
				injured++
			}
		}
		if injured != int(math.Round(float64(spec.count)*spec.injury)) {
			t.Fatalf("injury renewal exceeded %s cap: %d", spec.species, injured)
		}
	}
}
