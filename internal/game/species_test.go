package game

import (
	"reflect"
	"testing"
)

func TestLegacyIndividualReputationsMigrateToSpeciesOnce(t *testing.T) {
	p := Profile{Reputation: map[string]int{"animal_0": 4, "animal_turtle_3": 8, "animal_1": -2, "fish": 3, "animal_fish_5": -10, "unknown": 100}}
	normalizeSpeciesReputation(&p)
	want := map[string]int{"turtle": 6, "dolphin": -2, "fish": 3}
	if !reflect.DeepEqual(p.Reputation, want) {
		t.Fatalf("migration = %+v, want %+v", p.Reputation, want)
	}
	normalizeSpeciesReputation(&p)
	if !reflect.DeepEqual(p.Reputation, want) {
		t.Fatal("loading species reputations again changed their values")
	}
}

func TestWholeSpeciesAttitudeAndFriendlyApproach(t *testing.T) {
	w, p := testWorld()
	p.x, p.z = -100, 120
	a := findAnimal(w, "turtle")
	a.X, a.Z, a.Need, a.groupX, a.groupZ = -110, 120, "", -110, 120
	a.baseSpeed, a.lifespan, a.needAt = 2, 1e9, 1e9
	b := *a
	b.ID, b.X, b.GroupID = "another_turtle", -115, "another_pod"
	w.animals = []*Animal{a, &b}
	p.profile.Reputation["turtle"] = 1
	before := distance(p.x, p.z, a.X, a.Z)
	w.moveAnimals(1)
	if distance(p.x, p.z, a.X, a.Z) >= before {
		t.Fatal("friendly species did not swim toward player")
	}
	for _, view := range w.Snapshot("one").Animals {
		if view.Disposition != "friendly" {
			t.Fatal("members of the same species disagree about player")
		}
	}
	p.profile.Reputation["turtle"] = -1
	before = distance(p.x, p.z, a.X, a.Z)
	w.moveAnimals(1)
	if distance(p.x, p.z, a.X, a.Z) <= before || a.Disposition != "fleeing" {
		t.Fatal("disliked peaceful species did not flee")
	}
	p.profile.Reputation["turtle"] = 0
	for _, view := range w.Snapshot("one").Animals {
		if view.Disposition != "calm" {
			t.Fatal("neutral score retained a personal grudge")
		}
	}
}

func TestSpeciesGreetingsShareCooldownAndSometimesBringGifts(t *testing.T) {
	w, p := testWorld()
	a := findAnimal(w, "dolphin")
	b := *a
	b.ID = "another_dolphin"
	p.profile.Reputation["dolphin"] = 1
	for visit := 1; visit <= 9; visit++ {
		w.time = float64(visit * 61)
		if !w.greetAnimal(p, a) || w.greetAnimal(p, &b) {
			t.Fatal("greetings should be shared across the species")
		}
	}
	for _, item := range []string{"shell", "pearl", "treasure"} {
		if p.profile.Inventory[item] != 1 {
			t.Fatalf("occasional species gift %s missing", item)
		}
	}
	if len(p.profile.Reputation) != 1 || p.profile.Reputation["dolphin"] != 1 {
		t.Fatal("greetings created personal animal opinions")
	}
}

func TestFriendlySpeciesCanGreetSwimmersInShallowWater(t *testing.T) {
	w, p := testWorld()
	p.x, p.z = -100, coastZ(-100)+2
	p.profile.Reputation["turtle"] = 1
	a := findAnimal(w, "turtle")
	a.X, a.Z, a.Need = p.x, p.z+18, ""
	a.baseSpeed, a.lifespan = 2, 1e9
	w.animals = []*Animal{a}
	for range 30 {
		w.moveAnimals(.5)
	}
	if p.cooldowns["ocean_greeting:turtle"] == 0 {
		t.Fatal("friendly turtle could not reach a swimmer in the shallows to greet them")
	}
	if a.Z < coastZ(a.X) {
		t.Fatal("friendly approach stranded the turtle on land")
	}
}

func TestPersonalAnimalGreetingsAndGiftsOnlyReachTheirPlayer(t *testing.T) {
	w, p := testWorld()
	w.Join("two", "Neighbor")
	a := findAnimal(w, "dolphin")
	p.profile.Reputation[a.Species] = 1
	p.cooldowns["ocean_visits:dolphin"] = 2
	w.greetAnimal(p, a)
	w.event("A dolphin egg hatched.", "animal", a.X, a.Z)
	for _, id := range []string{"one", "two"} {
		personal, public := 0, 0
		for _, e := range w.Snapshot(id).Events {
			if e.Kind != "animal" {
				continue
			}
			if e.Actor != "" {
				personal++
			} else {
				public++
			}
		}
		if public != 1 || id == "one" && personal != 2 || id == "two" && personal != 0 {
			t.Fatalf("%s received %d personal and %d public animal events", id, personal, public)
		}
	}
}

func TestPondAdoptionImprovesReputationWithItsWholeSpecies(t *testing.T) {
	w, p := testWorld()
	p.profile.HomeIsland = &Island{Owner: p.profile.ID, Objects: []BuildObject{{ID: "pond", Kind: "pond"}}}
	a := findAnimal(w, "fish")
	a.Need = "wounded"
	p.x, p.z = a.X, a.Z
	mustAct(t, w, Action{Action: "adopt_animal", Target: a.ID})
	if p.profile.Reputation["fish"] != 3 || p.profile.Reputation["turtle"] != 0 || p.profile.Rescues != 1 {
		t.Fatal("adoption did not improve only the rescued species' reputation")
	}
	if err := w.Act("one", Action{Action: "adopt_animal", Target: a.ID}); err == nil || p.profile.Reputation["fish"] != 3 {
		t.Fatal("repeated adoption duplicated the rescue reward")
	}
}
