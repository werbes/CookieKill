package game

import (
	"errors"
	"fmt"
	"math"
)

const CamelTrainingIsland = "camel_training"

// Scalar progress keeps each camel independently saved and snapshots detached.
type CamelTrainingProgress struct {
	Speed     float64 `json:"speed"`
	Jump      float64 `json:"jump"`
	Stamina   float64 `json:"stamina"`
	Happiness float64 `json:"happiness"`
}

func trainingValue(progress *CamelTrainingProgress, stat string) *float64 {
	switch stat {
	case "speed":
		return &progress.Speed
	case "jump":
		return &progress.Jump
	case "stamina":
		return &progress.Stamina
	case "happiness":
		return &progress.Happiness
	}
	return nil
}

func camelStat(c *Camel, stat string) *float64 {
	switch stat {
	case "speed":
		return &c.Speed
	case "jump":
		return &c.Jump
	case "stamina":
		return &c.Stamina
	case "happiness":
		return &c.Happiness
	}
	return nil
}

type CamelTrainingCourse struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	CenterX       float64 `json:"centerX"`
	HalfWidth     float64 `json:"halfWidth"`
	HalfDepth     float64 `json:"halfDepth"`
	LaneWidth     float64 `json:"laneWidth"`
	Checkpoints   []Point `json:"checkpoints"`
	Hurdles       []Point `json:"hurdles"`
	PointsPerLap  float64 `json:"pointsPerLap"`
	PointsPerGain float64 `json:"pointsPerGain"`
	Gain          float64 `json:"gain"`
	Cap           float64 `json:"cap"`
	Description   string  `json:"description"`
}

func camelTrainingCourses() []CamelTrainingCourse {
	result := []CamelTrainingCourse{}
	for i, stat := range []string{"speed", "jump", "stamina", "happiness"} {
		cx := -42 + float64(i)*28
		c := CamelTrainingCourse{ID: stat, CenterX: cx, HalfWidth: 8, HalfDepth: 24, LaneWidth: 5, PointsPerLap: 25, PointsPerGain: 100,
			Checkpoints: []Point{{cx + 8, 24}, {cx - 8, 24}, {cx - 8, -24}, {cx + 8, -24}}, Hurdles: []Point{}}
		switch stat {
		case "speed":
			c.Name, c.Gain, c.Cap, c.Description = "Speed laps", .1, 16, "Ride a complete forward lap through all four gates."
		case "jump":
			c.Name, c.Gain, c.Cap, c.Description = "Jump hurdles", .1, 10, "Jump all three east-lane hurdles, then complete the lap."
			c.Hurdles = []Point{{cx + 8, -12}, {cx + 8, 0}, {cx + 8, 12}}
		case "stamina":
			c.Name, c.Gain, c.Cap, c.Description = "Endurance lane", 2, 200, "Sprint for at least 40 metres during a complete lap."
		case "happiness":
			c.Name, c.Gain, c.Cap, c.Description = "Calm care circuit", 2, 100, "Walk without sprinting and pause for one second at every gate."
		}
		result = append(result, c)
	}
	return result
}

func trainingCourse(id string) (CamelTrainingCourse, bool) {
	for _, c := range camelTrainingCourses() {
		if c.ID == id {
			return c, true
		}
	}
	return CamelTrainingCourse{}, false
}

type camelTraining struct {
	camelID, exercise                                  string
	returnTo                                           camelRaceReturn
	checkpoint, completedLaps                          int
	progress, lapDistance, sprintDistance, holdSeconds float64
	hurdles                                            [3]bool
}

type CamelTrainingView struct {
	InArena        bool                  `json:"inArena"`
	Exercise       string                `json:"exercise"`
	Checkpoint     int                   `json:"checkpoint"`
	CompletedLaps  int                   `json:"completedLaps"`
	LapDistance    float64               `json:"lapDistance"`
	SprintDistance float64               `json:"sprintDistance"`
	HurdlesCleared int                   `json:"hurdlesCleared"`
	HoldSeconds    float64               `json:"holdSeconds"`
	OwnCamel       *Camel                `json:"ownCamel,omitempty"`
	Courses        []CamelTrainingCourse `json:"courses"`
}

func (w *World) seedCamelTraining() {
	w.nodes = append(w.nodes, &Node{ID: "camel_training_lollipop", Kind: "camel_training_lollipop", Label: "Pink camel lollipop · training grounds", X: -231, Z: 14, Available: true})
}

func (w *World) camelTrainingAction(p *player, a Action) error {
	if a.Action == "training_leave" {
		return w.leaveCamelTraining(p)
	}
	if a.Action == "training_select" {
		if p.island != CamelTrainingIsland || p.training == nil {
			return errors.New("Visit the camel training grounds first.")
		}
		if _, ok := trainingCourse(a.Item); !ok {
			return errors.New("Choose speed, jump, stamina or happiness training.")
		}
		w.selectCamelTraining(p, a.Item)
		return nil
	}
	if a.Action != "training_enter" || a.Target != "camel_training_lollipop" || p.island != "" || p.inCave || !grounded(p) || distance(p.x, p.z, -231, 14) > 4 {
		return errors.New("Taste the pink camel lollipop inside Camel Cove to visit the training grounds.")
	}
	w.expireCamels(p)
	var c *Camel
	if a.Item != "" {
		c = ownCamel(p, a.Item)
	} else {
		c = mountedCamel(p)
		if c == nil || c.RentalUntil > 0 {
			for i := range p.profile.Camels {
				if p.profile.Camels[i].RentalUntil == 0 {
					c = &p.profile.Camels[i]
					break
				}
			}
		}
	}
	if c == nil || c.RentalUntil > 0 {
		return errors.New("Bring a permanently owned camel to train. Rentals and race loans cannot train.")
	}
	syncMountedCamel(p)
	p.training = &camelTraining{camelID: c.ID, returnTo: camelRaceReturn{x: p.x, y: p.y, z: p.z, yaw: p.yaw, pitch: p.pitch, island: p.island, inCave: p.inCave, ridingCamel: p.ridingCamel, mountedCamelID: p.mountedCamelID, stamina: p.stamina, sprintExhausted: p.sprintExhausted, fallen: p.fallen, standAt: p.standAt, fallDistance: p.fallDistance}}
	p.island, p.inCave, p.y = CamelTrainingIsland, false, 0
	p.ridingCamel, p.mountedCamelID = true, c.ID
	p.camelCalls = nil
	p.posture, p.buildMode, p.editingBuildID, p.bakeryBuildMode, p.editingBakeryFurnitureID = "", false, "", false, ""
	p.fallen, p.standAt, p.fallDistance = false, 0, 0
	p.stamina, p.sprintExhausted = c.Stamina, false
	w.selectCamelTraining(p, "speed")
	return nil
}

func (w *World) selectCamelTraining(p *player, stat string) {
	c, _ := trainingCourse(stat)
	t := p.training
	t.exercise, t.checkpoint, t.progress, t.lapDistance, t.sprintDistance, t.holdSeconds = stat, 0, 0, 0, 0, 0
	t.hurdles = [3]bool{}
	p.x, p.y, p.z, p.yaw, p.pitch = c.CenterX+c.HalfWidth, 0, -c.HalfDepth, math.Pi, 0
	resetMovement(p)
}

func (w *World) leaveCamelTraining(p *player) error {
	if p.training == nil || p.island != CamelTrainingIsland {
		return errors.New("You are not at the camel training grounds.")
	}
	s := p.training.returnTo
	p.training = nil
	p.x, p.y, p.z, p.yaw, p.pitch = s.x, s.y, s.z, s.yaw, s.pitch
	p.island, p.inCave, p.ridingCamel, p.mountedCamelID = s.island, s.inCave, s.ridingCamel, s.mountedCamelID
	p.stamina, p.sprintExhausted = s.stamina, s.sprintExhausted
	p.fallen, p.standAt, p.fallDistance = s.fallen, s.standAt, s.fallDistance
	resetMovement(p)
	if p.ridingCamel && mountedCamel(p) == nil {
		p.ridingCamel, p.mountedCamelID = false, ""
	}
	syncMountedCamel(p)
	return nil
}

func trainingTrackContains(c CamelTrainingCourse, x, z, y float64) bool {
	x -= c.CenterX
	margin := c.LaneWidth/2 - .6
	if !finite(x) || !finite(z) || math.Abs(x) > c.HalfWidth+margin || math.Abs(z) > c.HalfDepth+margin || math.Abs(x) < c.HalfWidth-margin && math.Abs(z) < c.HalfDepth-margin {
		return false
	}
	for _, h := range c.Hurdles {
		if math.Abs(x+c.CenterX-h.X) < 2.5 && math.Abs(z-h.Z) < .35 && y < .5 {
			return false
		}
	}
	return true
}

func trainingAngle(c CamelTrainingCourse, x, z float64) float64 {
	return math.Atan2(z/c.HalfDepth, (x-c.CenterX)/c.HalfWidth)
}

func (w *World) tickCamelTraining(dt float64) {
	for _, p := range w.players {
		if p.training == nil || p.island != CamelTrainingIsland {
			continue
		}
		if p.disconnected || ownCamel(p, p.training.camelID) == nil {
			_ = w.leaveCamelTraining(p)
			continue
		}
		t := p.training
		c, _ := trainingCourse(t.exercise)
		if w.time-p.cooldowns["last_input"] > .5 {
			p.input = Input{Yaw: p.yaw, Pitch: p.pitch}
			p.jumpQueued = false
		}
		moving := math.Hypot(p.input.X, p.input.Z) > .01
		if p.stamina <= 0 {
			p.sprintExhausted = true
		}
		if p.sprintExhausted && !p.sprintHeld && p.stamina >= staminaCapacity(p)*.2 {
			p.sprintExhausted = false
		}
		p.sprinting = moving && p.input.Sprint && !p.sprintExhausted
		speed := camelBaseSpeed(p)
		if p.sprinting {
			speed *= 1.65
			p.stamina = max(0, p.stamina-18*dt)
		} else {
			p.stamina = min(staminaCapacity(p), p.stamina+14*dt)
		}
		if p.jumpQueued && grounded(p) && p.cooldowns["jump"] <= w.time {
			p.jumpVelocity = camelJumpSpeed(p)
		}
		p.jumpQueued = false
		if !grounded(p) {
			p.jumpOffset += p.jumpVelocity*dt - jumpGravity*dt*dt/2
			p.jumpVelocity -= jumpGravity * dt
			if p.jumpOffset <= 0 {
				p.jumpOffset, p.jumpVelocity = 0, 0
				p.cooldowns["jump"] = w.time + .12
			}
		}
		dx, dz := p.input.X*speed*dt, p.input.Z*speed*dt
		steps := max(1, int(math.Ceil(math.Hypot(dx, dz)/.15)))
		travel := 0.
		for i := 0; i < steps; i++ {
			x, z := p.x, p.z
			if trainingTrackContains(c, p.x+dx/float64(steps), p.z, p.jumpOffset) {
				p.x += dx / float64(steps)
			}
			if trainingTrackContains(c, p.x, p.z+dz/float64(steps), p.jumpOffset) {
				p.z += dz / float64(steps)
			}
			d := distance(x, z, p.x, p.z)
			travel += d
			t.lapDistance += d
			if p.sprinting {
				t.sprintDistance += d
			}
			t.progress += math.Remainder(trainingAngle(c, p.x, p.z)-trainingAngle(c, x, z), 2*math.Pi)
			for j, h := range c.Hurdles {
				if z < h.Z && p.z >= h.Z && math.Abs(p.x-h.X) < 2.4 && p.jumpOffset >= .5 {
					t.hurdles[j] = true
				}
			}
		}
		gate := c.Checkpoints[t.checkpoint]
		near := distance(p.x, p.z, gate.X, gate.Z) <= 2.4 && t.progress+1e-6 >= float64(t.checkpoint+1)*math.Pi/2-.08
		if t.exercise == "happiness" {
			if near && travel < .01 && grounded(p) && !p.input.Sprint {
				t.holdSeconds += dt
			} else {
				t.holdSeconds = 0
			}
			near = near && t.holdSeconds >= 1
		}
		if !near {
			continue
		}
		t.checkpoint++
		t.holdSeconds = 0
		if t.checkpoint < 4 {
			continue
		}
		valid := t.lapDistance >= 110 && t.progress >= 2*math.Pi-.08
		if t.exercise == "jump" {
			valid = valid && t.hurdles[0] && t.hurdles[1] && t.hurdles[2]
		}
		if t.exercise == "stamina" {
			valid = valid && t.sprintDistance >= 40
		}
		if t.exercise == "happiness" {
			valid = valid && t.sprintDistance < 1
		}
		if valid {
			t.completedLaps++
			w.awardCamelTraining(p, c)
		} else {
			w.eventActor("Complete the exercise requirements during the whole lap to earn training progress.", "camel", p.x, p.z, p.profile.ID)
		}
		t.checkpoint, t.lapDistance, t.sprintDistance, t.holdSeconds = 0, 0, 0, 0
		t.progress -= 2 * math.Pi
		t.hurdles = [3]bool{}
	}
}

func (w *World) awardCamelTraining(p *player, course CamelTrainingCourse) {
	c := ownCamel(p, p.training.camelID)
	if c == nil {
		return
	}
	stat, progress, boost := camelStat(c, course.ID), trainingValue(&c.Training, course.ID), trainingValue(&c.TrainingBoost, course.ID)
	if *stat >= course.Cap {
		return
	}
	bonus := min(*boost, course.PointsPerLap)
	*boost -= bonus
	*progress += course.PointsPerLap + bonus
	for *progress >= course.PointsPerGain {
		*progress -= course.PointsPerGain
		*stat = math.Round(min(course.Cap, *stat+course.Gain)*100) / 100
		w.eventActor(fmt.Sprintf("Your camel's %s improved through training!", course.ID), "camel", p.x, p.z, p.profile.ID)
	}
	if *stat >= course.Cap {
		*progress = 0
	}
}

func (w *World) camelTrainingSnapshot(p *player) CamelTrainingView {
	view := CamelTrainingView{Courses: camelTrainingCourses()}
	if p == nil || p.training == nil || p.island != CamelTrainingIsland {
		return view
	}
	t := p.training
	view.InArena, view.Exercise, view.Checkpoint, view.CompletedLaps = true, t.exercise, t.checkpoint, t.completedLaps
	view.LapDistance, view.SprintDistance, view.HoldSeconds = t.lapDistance, t.sprintDistance, t.holdSeconds
	for _, done := range t.hurdles {
		if done {
			view.HurdlesCleared++
		}
	}
	if c := ownCamel(p, t.camelID); c != nil {
		cp := *c
		cp.X, cp.Y, cp.Z, cp.Yaw, cp.Island, cp.InCave = p.x, playerY(p), p.z, p.yaw, CamelTrainingIsland, false
		if until := p.cooldowns["camel_eating:"+c.ID]; until > w.time {
			cp.EatingUntil = until
		}
		view.OwnCamel = &cp
	}
	return view
}

func (w *World) decorateCamelTrainingSnapshot(p *player, out *Snapshot) {
	out.CamelTraining = w.camelTrainingSnapshot(p)
	if !out.CamelTraining.InArena {
		return
	}
	out.Me.Area, out.Me.Zone = "camel training", "Camel Cove training grounds"
	out.Me.CanBuildHome, out.Me.BuildMode, out.Me.BakeryBuildMode = false, false, false
	out.Nodes, out.Animals, out.Homes = []Node{}, []Animal{}, []Home{}
	out.Island = nil
	for i := range out.Players {
		out.Players[i].Area = "camel training"
	}
}
