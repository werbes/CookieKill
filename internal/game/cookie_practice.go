package game

import (
	"errors"
	"fmt"
	"math"
)

const practiceFee = 2
const practiceLevelCap = 10

type PracticeStationView struct {
	ID           string  `json:"id"`
	Kind         string  `json:"kind"`
	Name         string  `json:"name"`
	Stat         string  `json:"stat"`
	X            float64 `json:"x"`
	Z            float64 `json:"z"`
	Width        float64 `json:"width"`
	Depth        float64 `json:"depth"`
	Goal         float64 `json:"goal"`
	Unit         string  `json:"unit"`
	Instructions string  `json:"instructions"`
	OccupiedBy   string  `json:"occupiedBy,omitempty"`
	Active       bool    `json:"active"`
}
type PracticeTargetView struct {
	ID         string  `json:"id"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Z          float64 `json:"z"`
	Radius     float64 `json:"radius"`
	Moving     bool    `json:"moving"`
	FlashUntil float64 `json:"flashUntil"`
}
type PracticeExerciseView struct {
	Station   string  `json:"station"`
	Stat      string  `json:"stat"`
	Progress  float64 `json:"progress"`
	Goal      float64 `json:"goal"`
	Unit      string  `json:"unit"`
	Active    bool    `json:"active"`
	RestIn    float64 `json:"restIn"`
	NextRepIn float64 `json:"nextRepIn"`
	Status    string  `json:"status"`
	Lane      int     `json:"lane"`
	NextGate  *Point  `json:"nextGate,omitempty"`
}
type PracticePoolView struct {
	MinX  float64   `json:"minX"`
	MaxX  float64   `json:"maxX"`
	MinZ  float64   `json:"minZ"`
	MaxZ  float64   `json:"maxZ"`
	Lanes []float64 `json:"lanes"`
}
type CookiePracticeView struct {
	Stations []PracticeStationView `json:"stations"`
	Targets  []PracticeTargetView  `json:"targets"`
	Exercise PracticeExerciseView  `json:"exercise"`
	Hits     int                   `json:"hits"`
	Fee      int                   `json:"fee"`
	LevelCap int                   `json:"levelCap"`
	Pool     PracticePoolView      `json:"pool"`
}
type practiceSession struct {
	station         string
	progress        float64
	lastRep         float64
	lastX, lastZ    float64
	lane, poolStage int
	poolDistance    float64
	status          string
	active          bool
	hits            int
	lastTarget      string
	lastHit         float64
}

func practiceStations() []PracticeStationView {
	return []PracticeStationView{
		{ID: "practice_treadmill", Kind: "practice_treadmill", Name: "Treadmill", Stat: "stamina", X: -16, Z: 6, Width: 2.4, Depth: 4, Goal: 20, Unit: "seconds", Instructions: "E to step on. Hold W to run for 20 seconds. E or Q to step off."},
		{ID: "practice_bench_press", Kind: "practice_bench_press", Name: "Bench press", Stat: "arms", X: -8, Z: 6, Width: 2.2, Depth: 3, Goal: 10, Unit: "reps", Instructions: "E to sit. Press and release Space for 10 controlled repetitions. E or Q to stop."},
		{ID: "practice_leg_press", Kind: "practice_leg_press", Name: "Leg press", Stat: "legs", X: -16, Z: 14, Width: 2.2, Depth: 3, Goal: 10, Unit: "reps", Instructions: "E to sit. Press and release Space for 10 controlled repetitions. E or Q to stop."},
		{ID: "practice_pool", Kind: "practice_pool", Name: "Lane swimming", Stat: "swim", X: 6, Z: 14, Width: 1, Depth: 1, Goal: 2, Unit: "lengths", Instructions: "Enter a lane at the south end. Swim to its far marker and back, staying in the same lane."},
	}
}
func practiceStation(id string) (PracticeStationView, bool) {
	for _, s := range practiceStations() {
		if s.ID == id {
			return s, true
		}
	}
	return PracticeStationView{}, false
}
func cookiePracticeWater(p *player) bool {
	return p.island == CookiePracticeIsland && p.x > 8 && p.x < 20 && p.z > -8 && p.z < 16
}
func cookiePracticeWaterAt(x, z float64) bool { return x > 8 && x < 20 && z > -8 && z < 16 }
func cookiePracticeColliders() []Collider {
	list := []Collider{}
	for _, s := range practiceStations() {
		if s.ID == "practice_pool" {
			continue
		}
		height := 1.4
		if s.ID == "practice_treadmill" {
			height = 1.2
		}
		list = append(list, Collider{ID: s.ID, X: s.X, Z: s.Z, Width: s.Width, Depth: s.Depth, Height: height})
	}
	return list
}
func cookiePracticeMovementLocked(p *player) bool {
	return p.island == CookiePracticeIsland && p.practice != nil && p.practice.station != "" && p.practice.station != "practice_pool"
}
func cookiePracticeGround(p *player, x, z float64) float64 {
	if cookiePracticeMovementLocked(p) {
		switch p.practice.station {
		case "practice_treadmill":
			return .2
		case "practice_bench_press":
			return .45
		case "practice_leg_press":
			return .3
		}
	}
	if x > 8 && x < 20 && z > -8 && z < 16 {
		return -.6
	}
	return 0
}
func cookiePracticeBlocked(p *player, x, z float64) bool {
	if math.Abs(x) > 24.5 || math.Abs(z) > 24.5 {
		return true
	}
	for _, s := range practiceStations() {
		if s.ID == "practice_pool" || p.practice != nil && p.practice.station == s.ID {
			continue
		}
		if circleRect(x, z, .42, s.X, s.Z, s.Width, s.Depth) {
			return true
		}
	}
	return false
}
func (w *World) cookiePracticeEnter(p *player) {
	p.practice = &practiceSession{lastX: p.x, lastZ: p.z, lastRep: -10, lastHit: -10, lane: -1, status: "Aim at the targets, step onto an apparatus, or swim a lane."}
}
func (w *World) cookiePracticeLeave(p *player) { p.practice = nil; p.posture = "" }
func (w *World) practiceStop(p *player) {
	if p.practice == nil {
		return
	}
	s := p.practice
	if station, ok := practiceStation(s.station); ok && station.ID != "practice_pool" {
		p.x, p.z, p.y = station.X, station.Z+station.Depth/2+.75, 0
	}
	s.station, s.progress, s.active, s.poolDistance, s.poolStage = "", 0, false, 0, 0
	s.status = "Workout stopped. Completed levels stay with you."
	s.lastX, s.lastZ = p.x, p.z
	p.posture = ""
	resetMovement(p)
}
func (w *World) cookiePracticeAction(p *player, a Action) error {
	if p.island != CookiePracticeIsland || p.practice == nil {
		return errors.New("Climb to the tree hut and take the silver cookie to the practice center.")
	}
	if a.Action == "practice_stop" {
		w.practiceStop(p)
		return nil
	}
	if a.Action == "practice_rep" {
		return w.practiceRepetition(p)
	}
	station, ok := practiceStation(a.Target)
	if !ok || distance(p.x, p.z, station.X, station.Z) > 2.8 || math.Abs(playerY(p)) > 2 {
		return errors.New("Stand next to a practice apparatus to use it.")
	}
	if station.ID == "practice_pool" {
		w.eventActor(station.Instructions, "hint", p.x, p.z, p.profile.ID)
		return nil
	}
	if p.practice.station == station.ID {
		w.practiceStop(p)
		return nil
	}
	if p.practice.station != "" {
		return errors.New("Step off your current apparatus before choosing another.")
	}
	if p.profile.Levels[station.Stat] >= practiceLevelCap {
		return errors.New("You have already reached level 10 in this workout.")
	}
	for _, other := range w.players {
		if other != p && other.island == CookiePracticeIsland && other.practice != nil && other.practice.station == station.ID {
			return errors.New("This apparatus is in use. Choose another exercise while you wait.")
		}
	}
	p.practice.station, p.practice.progress, p.practice.active, p.practice.status = station.ID, 0, false, station.Instructions
	p.x, p.z, p.yaw, p.pitch = station.X, station.Z, 0, 0
	resetMovement(p)
	p.posture = map[string]string{"practice_treadmill": "treadmill", "practice_bench_press": "bench_press", "practice_leg_press": "leg_press"}[station.ID]
	p.y = cookiePracticeGround(p, p.x, p.z)
	p.practice.lastX, p.practice.lastZ = p.x, p.z
	return nil
}
func (w *World) practiceComplete(p *player, stat string) bool {
	if p.profile.Levels[stat] >= practiceLevelCap {
		p.practice.status = "Maximum level reached. Step off to try another exercise."
		return false
	}
	if p.profile.Coins < practiceFee {
		p.practice.status = "Set finished. You need 2 coins to complete this workout."
		return false
	}
	p.profile.Coins -= practiceFee
	p.profile.Levels[stat]++
	rest := 4.
	if w.active(p, "protein") {
		rest = 1.5
	}
	p.cooldowns["train"] = w.time + rest
	p.practice.progress, p.practice.poolDistance, p.practice.poolStage = 0, 0, 0
	p.practice.status = fmt.Sprintf("Set complete: %s level %d. Recover before the next set.", stat, p.profile.Levels[stat])
	w.eventActor(p.practice.status, "train", p.x, p.z, p.profile.ID)
	return true
}
func (w *World) practiceRepInterval(p *player) float64 {
	if w.active(p, "protein") {
		return .45
	}
	return .8
}
func (w *World) practiceRepetition(p *player) error {
	if !cookiePracticeMovementLocked(p) || !validChoice(p.practice.station, "practice_bench_press", "practice_leg_press") {
		return errors.New("Sit at a bench press or leg press before doing repetitions.")
	}
	if p.disconnected || w.time < p.cooldowns["train"] || w.time-p.practice.lastRep+1e-9 < w.practiceRepInterval(p) {
		return nil
	}
	station, _ := practiceStation(p.practice.station)
	if p.profile.Levels[station.Stat] >= practiceLevelCap {
		p.practice.status = "Maximum level reached."
		return nil
	}
	p.practice.lastRep = w.time
	p.practice.progress = min(station.Goal, p.practice.progress+1)
	p.practice.status = "Controlled repetition completed. Release Space before pressing again."
	if p.practice.progress >= station.Goal {
		w.practiceComplete(p, station.Stat)
	}
	return nil
}

// Space must be a fresh physical key edge; holding it never repeats a press.
func (w *World) cookiePracticeInput(p *player, in Input) {
	if cookiePracticeMovementLocked(p) && in.Jump && !p.jumpHeld {
		_ = w.practiceRepetition(p)
	}
}
func (w *World) tickCookiePractice(dt float64) {
	for _, p := range w.players {
		if p.island != CookiePracticeIsland || p.practice == nil || p.disconnected {
			continue
		}
		s := p.practice
		d := distance(s.lastX, s.lastZ, p.x, p.z)
		s.lastX, s.lastZ = p.x, p.z
		s.active = false
		fresh := w.time-p.cooldowns["last_input"] <= .5
		if cookiePracticeMovementLocked(p) {
			p.jumpQueued = false
			p.jumpOffset, p.jumpVelocity = 0, 0
			station, _ := practiceStation(s.station)
			if s.station == "practice_treadmill" {
				// Movement input is already in world coordinates. Recover the
				// forward key intent so looking around does not stop the belt.
				forward := -p.input.X*math.Sin(p.yaw) - p.input.Z*math.Cos(p.yaw)
				sideways := p.input.X*math.Cos(p.yaw) - p.input.Z*math.Sin(p.yaw)
				s.active = fresh && forward > .6 && math.Abs(sideways) < .6 && w.time >= p.cooldowns["train"] && p.profile.Levels[station.Stat] < practiceLevelCap
				if s.active {
					s.progress = min(station.Goal, s.progress+dt)
					s.status = "Keep running forward on the belt."
					if s.progress+1e-8 >= station.Goal {
						w.practiceComplete(p, station.Stat)
					}
				}
			} else {
				s.active = w.time-s.lastRep < .45
			}
			continue
		}
		if !cookiePracticeWater(p) {
			if s.station == "practice_pool" {
				s.station = ""
				s.progress = 0
				s.poolDistance = 0
				s.poolStage = 0
				s.status = "Start again at the south end of a swimming lane."
			}
			continue
		}
		moving := fresh && math.Hypot(p.input.X, p.input.Z) > .1 && d > .001 && d < 1 && grounded(p)
		if s.station == "" && moving && math.Abs(p.z-13) <= 1.2 {
			for i, x := range []float64{10, 14, 18} {
				if math.Abs(p.x-x) <= 1.2 {
					s.station = "practice_pool"
					s.lane = i
					s.poolStage = 0
					s.poolDistance = 0
					s.progress = 0
					break
				}
			}
		}
		if s.station != "practice_pool" {
			continue
		}
		laneX := 10 + float64(s.lane)*4
		if math.Abs(p.x-laneX) > 1.5 {
			s.station = ""
			s.progress = 0
			s.poolStage = 0
			s.poolDistance = 0
			s.status = "Stay in one lane; begin again at its south marker."
			continue
		}
		if !moving || w.time < p.cooldowns["train"] || p.profile.Levels["swim"] >= practiceLevelCap {
			continue
		}
		s.active = true
		s.poolDistance += d
		goalZ := -5.
		if s.poolStage == 1 {
			goalZ = 13
		}
		s.progress = float64(s.poolStage) + max(0, min(1, 1-math.Abs(p.z-goalZ)/18))
		s.status = "Swim to the next lane marker."
		if math.Abs(p.z-goalZ) <= 1 {
			if s.poolStage == 0 {
				s.poolStage = 1
				s.progress = 1
			} else if s.poolDistance >= 30 {
				s.progress = 2
				w.practiceComplete(p, "swim")
			}
		}
	}
}
func (w *World) practiceTargets() []PracticeTargetView {
	list := []PracticeTargetView{}
	for i, x := range []float64{-16, -8, 0, 12 + math.Sin(w.time*.8)*5} {
		target := PracticeTargetView{ID: fmt.Sprintf("practice_target_%d", i), X: x, Y: 1.8, Z: -21, Radius: .75, Moving: i == 3}
		for _, p := range w.players {
			if p.practice != nil && p.practice.lastTarget == target.ID {
				target.FlashUntil = max(target.FlashUntil, p.practice.lastHit+.6)
			}
		}
		list = append(list, target)
	}
	return list
}
func (w *World) cookiePracticeShotHit(shot *Projectile, nx, ny, nz float64) (float64, string, bool) {
	nearest, target := 2., ""
	dz := nz - shot.Z
	if math.Abs(dz) < 1e-9 {
		return nearest, target, false
	}
	for _, t := range w.practiceTargets() {
		fraction := (t.Z - shot.Z) / dz
		if fraction < 0 || fraction > 1 || fraction >= nearest {
			continue
		}
		x, y := shot.X+(nx-shot.X)*fraction, shot.Y+(ny-shot.Y)*fraction
		if math.Hypot(x-t.X, y-t.Y) <= t.Radius {
			nearest, target = fraction, t.ID
		}
	}
	return nearest, target, target != ""
}
func (w *World) hitCookiePracticeTarget(owner *player, target string, shot *Projectile) {
	if owner == nil || owner.island != CookiePracticeIsland || owner.practice == nil {
		return
	}
	for _, t := range w.practiceTargets() {
		if t.ID == target {
			owner.practice.hits++
			owner.practice.lastTarget = target
			owner.practice.lastHit = w.time
			w.eventActor(fmt.Sprintf("Target hit! %d accurate practice throws.", owner.practice.hits), "hit", t.X, t.Z, owner.profile.ID)
			return
		}
	}
}
func (w *World) cookiePracticeSnapshot(viewer *player, out *Snapshot) {
	if viewer == nil || viewer.island != CookiePracticeIsland || viewer.practice == nil {
		return
	}
	s := viewer.practice
	view := &CookiePracticeView{Stations: practiceStations(), Targets: w.practiceTargets(), Hits: s.hits, Fee: practiceFee, LevelCap: practiceLevelCap, Pool: PracticePoolView{MinX: 8, MaxX: 20, MinZ: -8, MaxZ: 16, Lanes: []float64{10, 14, 18}}}
	view.Exercise = PracticeExerciseView{Station: s.station, Progress: s.progress, Active: s.active, RestIn: max(0, viewer.cooldowns["train"]-w.time), NextRepIn: max(0, w.practiceRepInterval(viewer)-(w.time-s.lastRep)), Status: s.status, Lane: s.lane}
	if station, ok := practiceStation(s.station); ok {
		view.Exercise.Stat, view.Exercise.Goal, view.Exercise.Unit = station.Stat, station.Goal, station.Unit
	}
	if s.station == "practice_pool" {
		z := -5.
		if s.poolStage == 1 {
			z = 13
		}
		view.Exercise.NextGate = &Point{10 + float64(s.lane)*4, z}
	}
	for i := range view.Stations {
		station := &view.Stations[i]
		for _, p := range w.players {
			if p.island == CookiePracticeIsland && p.practice != nil && p.practice.station == station.ID && station.ID != "practice_pool" {
				station.OccupiedBy, station.Active = p.profile.ID, p.practice.active
				break
			}
		}
		out.Nodes = append(out.Nodes, Node{ID: station.ID, Kind: station.Kind, Label: station.Name, X: station.X, Z: station.Z, Available: true, OccupiedBy: station.OccupiedBy})
	}
	out.CookiePractice = view
}
