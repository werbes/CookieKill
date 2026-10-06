package game

import (
	"math"
	"testing"
)

func TestSparsePodsStayCloseWhileDolphinsJumpIndividually(t *testing.T) {
	w := New(nil)
	if len(w.animals) != 57 {
		t.Fatalf("expected 1.5 times the previous 38 animals, got %d", len(w.animals))
	}
	differentHeadings := false
	for i := 0; i < 600; i++ {
		w.time += .05
		w.moveAnimals(.05)
		groups := map[string][]*Animal{}
		jumping := 0
		for _, a := range w.animals {
			groups[a.GroupID] = append(groups[a.GroupID], a)
			if a.Species == "dolphin" && a.Y > .1 {
				jumping++
			}
		}
		if jumping > 1 {
			t.Fatal("dolphins jumped together")
		}
		for _, group := range groups {
			if len(group) < 2 {
				continue
			}
			for _, a := range group {
				nearest := math.Inf(1)
				for _, b := range group {
					if a != b {
						nearest = math.Min(nearest, distance(a.X, a.Z, b.X, b.Z))
						if math.Abs(a.Heading-b.Heading) > .05 {
							differentHeadings = true
						}
					}
				}
				if nearest > 1.00001 {
					t.Fatalf("%s strayed %.2fm from its group", a.ID, nearest)
				}
			}
		}
	}
	if !differentHeadings {
		t.Fatal("pods still move in rigid straight lines")
	}
}

func TestOldAgeReplacementEggNeedsTwoUndisturbedMinutes(t *testing.T) {
	w := New(nil)
	w.Join("caretaker", "Caretaker")
	p := w.players["caretaker"]
	a := w.animals[0]
	w.animals = []*Animal{a}
	a.Need = ""
	a.Age, a.lifespan = 9.9, 10
	p.profile.Reputation[a.ID] = 5
	w.time = .2
	w.moveAnimals(.2)
	if a.Health > 0 || len(w.Snapshot("caretaker").Animals) != 0 {
		t.Fatal("old animal did not despawn")
	}
	w.time = a.respawnAt
	w.moveAnimals(.05)
	if !a.Egg || a.HatchIn != 120 || p.profile.Reputation[a.ID] != 0 {
		t.Fatal("replacement did not start as a new egg")
	}
	w.time += 119
	w.moveAnimals(.05)
	if !a.Egg {
		t.Fatal("egg hatched too early")
	}
	p.x, p.z = a.X, a.Z
	w.moveAnimals(.05)
	if a.HatchIn != 120 {
		t.Fatal("touching egg did not reset its quiet period")
	}
	p.x, p.z = -35, -35
	w.time += 119
	w.moveAnimals(.05)
	if !a.Egg {
		t.Fatal("disturbed egg kept its old hatch timer")
	}
	w.time += 1.1
	w.moveAnimals(.05)
	if a.Egg || a.Health != a.MaxHealth || a.Age != 0 {
		t.Fatal("quiet egg did not hatch into a healthy newborn")
	}
}

func TestUntreatedAnimalsDespawnButTreatmentStopsDeadline(t *testing.T) {
	w := New(nil)
	w.Join("helper", "Helper")
	p := w.players["helper"]
	a, b := w.animals[0], w.animals[1]
	w.animals = []*Animal{a, b}
	w.injureAnimal(a)
	w.injureAnimal(b)
	p.x, p.z = b.X, b.Z
	w.time = untreatedSeconds - 1
	if err := w.helpAnimal(p, b); err != nil {
		t.Fatal(err)
	}
	w.time = untreatedSeconds + 1
	w.moveAnimals(.05)
	if a.Health > 0 {
		t.Fatal("untreated animal survived its deadline")
	}
	if b.Health <= 0 || b.Need != "" {
		t.Fatal("treated animal retained its sickness deadline")
	}
}

func TestProjectileDisturbanceRestartsEggIncubation(t *testing.T) {
	w := New(nil)
	a := w.animals[0]
	w.beginEgg(a)
	w.time = 100
	w.damageAnimal(nil, a, 1000)
	if !a.Egg || a.Health <= 0 || a.eggSince != 100 {
		t.Fatal("egg hit did not restart incubation")
	}
}
