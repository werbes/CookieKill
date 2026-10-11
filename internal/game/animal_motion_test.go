package game

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestQuietAnimalRoutesStayStableAndMatchAuthority(t *testing.T) {
	w := New(nil)
	w.time = .05
	w.moveAnimals(.05)
	motions := make(map[string]*AnimalMotion)
	for _, a := range w.animals {
		if a.Motion == nil {
			t.Fatalf("%s has no quiet route", a.ID)
		}
		motions[a.ID] = a.Motion
	}
	for range 200 {
		w.time += .05
		w.moveAnimals(.05)
		for _, a := range w.animals {
			if a.Motion != motions[a.ID] {
				t.Fatalf("%s regenerated its undisturbed route", a.ID)
			}
			p := a.Motion.Sample(a.Species, a.Egg, w.time)
			if a.X != p.X || a.Z != p.Z || a.Y != p.Y || a.Heading != p.Heading || a.Speed != p.Speed {
				t.Fatalf("%s authority diverged from advertised route", a.ID)
			}
			if math.Abs(a.Age-p.Age) > 1e-8 {
				t.Fatalf("%s age diverged from route clock", a.ID)
			}
		}
	}
	view := w.Snapshot("")
	view.Animals[0].Motion.CenterX = 12345
	if w.animals[0].Motion.CenterX == 12345 {
		t.Fatal("snapshot motion aliases mutable world state")
	}
}

func TestAnimalSwimmerInteractionAndContinuousRouteReturn(t *testing.T) {
	w, p := testWorld()
	a := findAnimal(w, "fish")
	a.Need, a.needAt, a.lifespan = "", 1e9, 1e9
	w.animals = []*Animal{a}
	w.time = 1
	w.moveAnimals(.05)
	quiet := a.Motion
	if quiet == nil {
		t.Fatal("quiet fish has no route")
	}
	p.profile.Reputation[a.Species] = 2
	p.x, p.z = a.X+8, a.Z
	w.time += .05
	before := distance(a.X, a.Z, p.x, p.z)
	w.moveAnimals(.05)
	if a.Motion != nil || a.Disposition != "friendly" || distance(a.X, a.Z, p.x, p.z) >= before {
		t.Fatal("nearby trusted swimmer did not get authoritative following")
	}
	p.profile.Reputation[a.Species] = -2
	w.time += .05
	before = distance(a.X, a.Z, p.x, p.z)
	w.moveAnimals(.05)
	if a.Motion != nil || a.Disposition != "fleeing" || distance(a.X, a.Z, p.x, p.z) <= before {
		t.Fatal("nearby disliked swimmer did not get authoritative fleeing")
	}
	p.x, p.z = 0, -100
	x, z := a.X, a.Z
	w.time += .05
	w.moveAnimals(.05)
	resumed := a.Motion
	if resumed == nil || resumed == quiet || a.X != x || a.Z != z {
		t.Fatal("return to route teleported or failed to create a new plan")
	}
	for range 200 {
		w.time += .05
		w.moveAnimals(.05)
		if a.Motion != resumed || !waterAt(a.X, a.Z) {
			t.Fatal("resumed route changed without interaction or left the ocean")
		}
	}
}

func TestAnimalReturnFromShallowsStaysInOcean(t *testing.T) {
	w := New(nil)
	w.time = 60
	for x := MinX + 8; x <= -9; x += 5 {
		for phase := 0.0; phase < 2*math.Pi; phase += .7 {
			a := &Animal{ID: "returning", Species: "fish", X: x, Z: coastZ(x) + 3, Health: 10,
				groupX: -210, groupZ: 80, orbitRadius: 19, baseSpeed: 4, phase: phase}
			w.planAnimalGroup([]*Animal{a}, 4)
			for elapsed := 0.0; elapsed <= 120; elapsed += .25 {
				p := a.Motion.Sample(a.Species, false, w.time+elapsed)
				if !waterAt(p.X, p.Z) || p.X < MinX+8-1e-8 || p.X > -9+1e-8 || p.Z > MaxZ-7+1e-8 {
					t.Fatalf("return from x=%.1f crossed the coast or bounds at %.2fs: %.2f,%.2f", x, elapsed, p.X, p.Z)
				}
			}
		}
	}
}

func TestAnimalPositionChangeInvalidatesSeededRoute(t *testing.T) {
	w := New(nil)
	a := findAnimal(w, "fish")
	w.animals = []*Animal{a}
	w.moveAnimals(0)
	prior := a.Motion
	a.X, a.Z = -100, 120
	w.time = .05
	w.moveAnimals(.05)
	if a.Motion == prior || a.X != -100 || a.Z != 120 {
		t.Fatal("externally moved animal snapped back to its old route")
	}
}

func TestAnimalRouteChangesForTrapsPodMembershipAndEggClocks(t *testing.T) {
	w := New(nil)
	a := findAnimal(w, "turtle")
	group := []*Animal{}
	for _, other := range w.animals {
		if other.GroupID == a.GroupID {
			other.Need, other.needAt, other.lifespan = "", 1e9, 1e9
			group = append(group, other)
		}
	}
	w.animals = group
	w.time = 1
	w.moveAnimals(.05)
	prior := a.Motion
	a.Need = "trapped"
	w.time += .05
	w.moveAnimals(.05)
	if a.Motion == prior || a.Motion.Kind != "still" {
		t.Fatal("trapping an animal did not stop its advertised route")
	}
	x, z := a.X, a.Z
	for range 100 {
		w.time += .05
		w.moveAnimals(.05)
	}
	if a.X != x || a.Z != z {
		t.Fatal("trapped animal drifted")
	}
	for _, other := range group[1:] {
		if distance(a.X, a.Z, other.X, other.Z) > 1 {
			t.Fatal("pod abandoned trapped member")
		}
	}
	if len(group) > 1 {
		other := group[1]
		prior = other.Motion
		a.Health, a.respawnAt = 0, w.time+30
		w.time += .05
		w.moveAnimals(.05)
		if other.Motion == prior || other.Motion.Radius == 0 {
			t.Fatal("surviving pod did not resume after trapped member left")
		}
	}
	w.beginEgg(a)
	prior = a.Motion
	w.time += 10
	w.moveAnimals(.05)
	if a.Motion != prior || a.Motion.Sample(a.Species, true, w.time).HatchIn != a.HatchIn {
		t.Fatal("quiet egg requires changing countdown updates")
	}
	w.damageAnimal(nil, a, 1)
	for _, view := range w.Snapshot("").Animals {
		if view.ID == a.ID && view.Motion.Sample(view.Species, true, w.time).HatchIn != 120 {
			t.Fatal("egg disturbance was not reflected in snapshot deadline")
		}
	}
}

// This fixture is shared with the browser tests so both implementations must
// agree at transition and jump boundaries, not merely look plausible alone.
func TestAnimalMotionSharedBrowserFixtures(t *testing.T) {
	data, err := os.ReadFile("../../scripts/fixtures/animal-motion.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name     string  `json:"name"`
		Time     float64 `json:"time"`
		Animal   Animal  `json:"animal"`
		Expected struct {
			X, Y, Z, Heading, Speed, Age, HatchIn float64
		} `json:"expected"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			p := f.Animal.Motion.Sample(f.Animal.Species, f.Animal.Egg, f.Time)
			got := []float64{p.X, p.Y, p.Z, p.Heading, p.Speed, p.Age, p.HatchIn}
			want := []float64{f.Expected.X, f.Expected.Y, f.Expected.Z, f.Expected.Heading, f.Expected.Speed, f.Expected.Age, f.Expected.HatchIn}
			for i := range got {
				if math.Abs(got[i]-want[i]) > 1e-10 {
					t.Fatalf("component %d = %.15g, want %.15g", i, got[i], want[i])
				}
			}
		})
	}
}
