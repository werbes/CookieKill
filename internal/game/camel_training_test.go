package game

import (
	"math"
	"reflect"
	"testing"
)

func trainingWorld(t *testing.T) (*World, *player) {
	t.Helper()
	w, p := testWorld()
	p.x, p.z = -231, 14
	p.profile.Camels = []Camel{{ID: "one_camel_1", X: 224, Z: -126, Speed: 10, Jump: 6.2, Stamina: 120, Happiness: 60, TraitsVersion: 1}}
	camelCompatibility(&p.profile)
	mustAct(t, w, Action{Action: "training_enter", Target: "camel_training_lollipop"})
	return w, p
}

// Drive the real normalized input/physics pipeline, including edge-triggered
// jumps and stamina exhaustion. No test driver assigns course progress.
func driveTrainingLap(t *testing.T, w *World, p *player, sprint, jump, care bool) {
	t.Helper()
	c, _ := trainingCourse(p.training.exercise)
	for _, gate := range c.Checkpoints {
		start := w.time
		for distance(p.x, p.z, gate.X, gate.Z) > .06 {
			dx, dz := gate.X-p.x, gate.Z-p.z
			d := math.Hypot(dx, dz)
			pressJump := false
			if jump && grounded(p) && dz > 0 && math.Abs(p.x-(c.CenterX+8)) < 1 {
				for _, h := range c.Hurdles {
					if h.Z-p.z > 0 && h.Z-p.z < 3 {
						pressJump = true
					}
				}
			}
			w.Input("one", Input{X: dx / d, Z: dz / d, Sprint: sprint, Jump: pressJump})
			w.Tick(min(.03, d/(camelBaseSpeed(p)*1.65)))
			if w.time-start > 25 {
				t.Fatalf("driver stuck exercise=%s gate=%+v pos=(%.2f,%.2f) jump=%.2f, checkpoint=%d progress=%.3f", c.ID, gate, p.x, p.z, p.jumpOffset, p.training.checkpoint, p.training.progress)
			}
		}
		w.Input("one", Input{})
		if care {
			w.Tick(1)
			w.Tick(.1)
		} else {
			w.Tick(.05)
		}
	}
}

func TestCamelTrainingEntryOwnershipIsolationAndSafeReturn(t *testing.T) {
	w, p := testWorld()
	p.x, p.z = -231, 14
	for _, camels := range [][]Camel{nil, {{ID: "one_camel_1", RentalUntil: w.now().Unix() + 100}}} {
		p.profile.Camels = camels
		if err := w.Act("one", Action{Action: "training_enter", Target: "camel_training_lollipop"}); err == nil {
			t.Fatal("training accepted missing or rental camel")
		}
	}
	p.profile.Camels = []Camel{{ID: "one_camel_1", X: -231, Z: 14}}
	normalizeCamels(&p.profile)
	before := p.profile.Camels[0]
	p.x = -220
	if err := w.Act("one", Action{Action: "training_enter", Target: "camel_training_lollipop"}); err == nil {
		t.Fatal("remote training entry accepted")
	}
	p.x = -231
	p.ridingCamel, p.mountedCamelID = true, before.ID
	mustAct(t, w, Action{Action: "interact", Target: "camel_training_lollipop"})
	if p.island != CamelTrainingIsland || !p.ridingCamel || p.profile.Camels[0] != before {
		t.Fatal("entry changed saved camel or did not mount trainee")
	}
	for _, a := range []Action{{Action: "camel_dismount"}, {Action: "teleport_main"}, {Action: "drop_item", From: "hotbar", FromSlot: 0}, {Action: "training_select", Item: "made_up"}} {
		if err := w.Act("one", a); err == nil {
			t.Fatalf("training accepted invalid action %+v", a)
		}
	}
	w.Join("observer", "Observer")
	s := w.Snapshot("one")
	if !s.CamelTraining.InArena || s.CamelTraining.OwnCamel == nil || s.Island != nil || len(s.Nodes) != 0 || len(s.Animals) != 0 || len(s.Players) != 0 || w.johnEligible(p) {
		t.Fatal("training snapshot or John exclusion leaked world state")
	}
	s.CamelTraining.OwnCamel.Speed = 999
	s.CamelTraining.Courses[0].Checkpoints[0].X = 999
	if w.Snapshot("one").CamelTraining.Courses[0].Checkpoints[0].X == 999 || p.profile.Camels[0].Speed == 999 {
		t.Fatal("training snapshot aliases state")
	}
	mustAct(t, w, Action{Action: "training_leave"})
	if p.x != -231 || p.z != 14 || p.island != "" || !p.ridingCamel || p.profile.Camels[0] != before {
		t.Fatal("training return lost original mount or parking")
	}
}

func TestCamelTrainingNoIdleSpamWallOrWrongWayProgress(t *testing.T) {
	w, p := trainingWorld(t)
	before := p.profile.Camels[0]
	for i := 0; i < 100; i++ {
		w.Input("one", Input{Jump: i%2 == 0})
		w.Tick(.1)
	}
	if p.profile.Camels[0] != before || p.training.completedLaps != 0 {
		t.Fatal("idle jumping awarded stats")
	}
	for i := 0; i < 100; i++ {
		w.Input("one", Input{X: 999})
		w.Tick(.05)
	}
	if p.x > -32.1 || p.training.completedLaps != 0 {
		t.Fatal("course boundary bypassed")
	}
	for i := 0; i < 8; i++ {
		mustAct(t, w, Action{Action: "training_select", Item: "speed"})
	}
	if p.profile.Camels[0] != before {
		t.Fatal("exercise selection awarded stats")
	}
	// The first edge in reverse crosses the finish gate, not the next gate.
	for i := 0; i < 100; i++ {
		w.Input("one", Input{X: -1})
		w.Tick(.03)
	}
	if p.training.checkpoint != 0 || p.training.progress >= 0 || p.profile.Camels[0] != before {
		t.Fatal("wrong-way course motion earned progress")
	}
}

func TestCamelTrainingMeasuredLapsTreatBoostAndPersistence(t *testing.T) {
	w, p := trainingWorld(t)
	setItem(p, "swift_dates", 3)
	before := p.profile.Camels[0]
	mustAct(t, w, Action{Action: "feed_camel", Target: before.ID, Item: "swift_dates"})
	if p.profile.Camels[0].Speed != before.Speed || p.profile.Camels[0].TrainingBoost.Speed != 120 {
		t.Fatal("treat bypassed training")
	}
	fed := p.profile.Camels[0]
	for i := 0; i < 10; i++ {
		w.Tick(1)
	}
	if p.profile.Camels[0] != fed {
		t.Fatal("waiting spent training energy or granted exercise progress")
	}
	driveTrainingLap(t, w, p, false, false, false)
	if p.training.completedLaps != 1 || p.profile.Camels[0].Training.Speed != 50 || p.profile.Camels[0].Speed != before.Speed || p.profile.Camels[0].TrainingBoost.Speed != 95 {
		t.Fatalf("first measured lap failed %+v %+v", p.training, p.profile.Camels[0])
	}
	driveTrainingLap(t, w, p, false, false, false)
	if p.training.completedLaps != 2 || p.profile.Camels[0].Training.Speed != 0 || p.profile.Camels[0].Speed != before.Speed+.1 || p.profile.Camels[0].TrainingBoost.Speed != 70 {
		t.Fatal("two boosted laps did not earn one gradual improvement")
	}
	trained := p.profile.Camels[0]
	if trained.X != before.X || trained.Z != before.Z || trained.Island != before.Island {
		t.Fatal("training teleported persistent parked camel")
	}
	w.Leave("one")
	w.Join("one", "Returning trainer")
	p = w.players["one"]
	if p.training != nil || p.island != "" || p.profile.Camels[0] != trained {
		t.Fatal("logout did not preserve training and return")
	}
	if got := New(w.Profiles()).Profiles()["one"].Camels[0]; got != trained {
		t.Fatalf("training restart persistence failed: %+v", got)
	}
}

func TestCamelTrainingRealJumpEnduranceAndCareExercises(t *testing.T) {
	for _, stat := range []string{"jump", "stamina", "happiness"} {
		t.Run(stat, func(t *testing.T) {
			w, p := trainingWorld(t)
			mustAct(t, w, Action{Action: "training_select", Item: stat})
			if stat == "jump" {
				for i := 0; i < 50; i++ {
					w.Input("one", Input{Z: 1})
					w.Tick(.05)
				}
				if p.z > -12.35 || p.training.hurdles[0] {
					t.Fatal("grounded camel crossed a jump hurdle")
				}
				mustAct(t, w, Action{Action: "training_select", Item: stat})
			}
			if stat == "stamina" {
				driveTrainingLap(t, w, p, false, false, false)
				if p.training.completedLaps != 0 || p.profile.Camels[0].Training.Stamina != 0 {
					t.Fatal("walking lap granted endurance training")
				}
			}
			for i := 0; i < 4; i++ {
				driveTrainingLap(t, w, p, stat == "stamina", stat == "jump", stat == "happiness")
			}
			c, _ := trainingCourse(stat)
			want := map[string]float64{"jump": 6.3, "stamina": 122, "happiness": 62}[stat]
			if p.training.completedLaps != 4 || math.Abs(*camelStat(&p.profile.Camels[0], stat)-want) > 1e-8 || *trainingValue(&p.profile.Camels[0].Training, stat) != 0 {
				t.Fatalf("exercise did not earn gradual %s gain %v: %+v %+v", stat, c.Gain, p.training, p.profile.Camels[0])
			}
		})
	}
}

func TestCamelTrainingRaceGatherDisconnectAndBoostNormalization(t *testing.T) {
	w, p := testWorld()
	p.x, p.z = -227, 20
	p.profile.Camels = []Camel{{ID: "one_camel_1", X: 224, Z: -126}}
	p.profile.Camels = append(p.profile.Camels, Camel{ID: "one_camel_2", X: 223, Z: -126})
	normalizeCamels(&p.profile)
	mustAct(t, w, Action{Action: "race_enter", Target: "camel_race_lollipop"})
	p.x, p.z = -231, 14
	mustAct(t, w, Action{Action: "training_enter", Target: "camel_training_lollipop", Item: "one_camel_2"})
	before := p.profile.Camels[0]
	w.time = 100
	w.tickCamelRace(0)
	if p.training != nil || p.island != CamelRaceIsland || w.camelRace.entries["one"].returnTo.island != "" || p.profile.Camels[0] != before {
		t.Fatal("race gathering did not safely exit training")
	}
	if p.raceCamel.ID != "one_camel_2" {
		t.Fatal("race gathering left the selected training camel behind")
	}
	mustAct(t, w, Action{Action: "race_leave"})
	if p.island != "" || p.x != -231 || p.z != 14 {
		t.Fatal("race returned to orphan training world")
	}
	mustAct(t, w, Action{Action: "training_enter", Target: "camel_training_lollipop"})
	w.SetConnected("one", false)
	w.Tick(.05)
	if p.island != "" || p.training != nil {
		t.Fatal("disconnect grace left training session active")
	}
	c := p.profile.Camels[0]
	c.Training.Speed = math.NaN()
	c.Training.Jump = 500
	c.TrainingBoost.Stamina = math.Inf(1)
	c.TrainingBoost.Happiness = 999
	camelTraits(&c)
	if c.Training.Speed != 0 || c.Training.Jump != 99.999 || c.TrainingBoost.Stamina != 0 || c.TrainingBoost.Happiness != 240 {
		t.Fatal("invalid saved training values not normalized")
	}
	stable := c
	camelTraits(&c)
	if !reflect.DeepEqual(stable, c) {
		t.Fatal("training normalization is not stable")
	}
}

func TestCamelTrainingCapsPreserveOtherCamelAndRequireWholeCourse(t *testing.T) {
	w, p := trainingWorld(t)
	other := p.profile.Camels[0]
	other.ID = "one_camel_2"
	p.profile.Camels = append(p.profile.Camels, other)
	p.profile.Camels[0].Speed, p.profile.Camels[0].Training.Speed = 15.95, 75
	driveTrainingLap(t, w, p, false, false, false)
	if p.profile.Camels[0].Speed != 16 || p.profile.Camels[0].Training.Speed != 0 || p.profile.Camels[1] != other {
		t.Fatal("training cap or per-camel isolation failed")
	}
	setItem(p, "swift_dates", 1)
	if err := w.Act("one", Action{Action: "feed_camel", Target: "one_camel_1", Item: "swift_dates"}); err == nil || p.profile.Inventory["swift_dates"] != 1 {
		t.Fatal("maxed stat consumed a training treat")
	}
	if err := w.Act("one", Action{Action: "feed_camel", Target: other.ID, Item: "swift_dates"}); err == nil || p.profile.Camels[1] != other {
		t.Fatal("training session fed a different camel from another world")
	}
	mustAct(t, w, Action{Action: "training_select", Item: "happiness"})
	// Driving through every gate without any care stop must earn nothing.
	driveTrainingLap(t, w, p, false, false, false)
	if p.training.checkpoint != 0 || p.profile.Camels[0].Training.Happiness != 0 || p.profile.Camels[0].Happiness != 60 {
		t.Fatal("care circuit awarded progress without calm stops")
	}
}
