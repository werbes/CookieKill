package game

import (
	"errors"
	"math"
	"sort"
)

const rollerCoasterDuration = 48.0
const rollerCoasterX = 165.0
const rollerCoasterZ = -161.0

type CoasterPoint struct {
	T     float64 `json:"t"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Z     float64 `json:"z"`
	Yaw   float64 `json:"yaw"`
	Pitch float64 `json:"pitch"`
	Bank  float64 `json:"bank"`
}

type CoasterCar struct {
	CoasterPoint
	ID         string  `json:"id"`
	PlayerID   string  `json:"playerId"`
	PlayerName string  `json:"playerName"`
	CamelID    string  `json:"camelId"`
	CamelName  string  `json:"camelName"`
	StartedAt  float64 `json:"startedAt"`
	Progress   float64 `json:"progress"`
	Remaining  float64 `json:"remaining"`
}

type RollerCoasterView struct {
	Station     CoasterPoint   `json:"station"`
	Duration    float64        `json:"duration"`
	Track       []CoasterPoint `json:"track"`
	Cars        []CoasterCar   `json:"cars"`
	Ride        *CoasterCar    `json:"ride"`
	NextBoardIn float64        `json:"nextBoardIn"`
}

type rollerCoasterRide struct {
	startedAt float64
	returnTo  camelRaceReturn
	camelID   string
	camelName string
}

// Each knot gives elapsed fraction, angle fraction, and cart floor height.
// The long lift leads into a quick drop, a second hill, and banked return turns.
var coasterKnots = [][3]float64{
	{0, 0, 1.1}, {.10, .045, 1.1}, {.40, .23, 24}, {.48, .38, 2},
	{.62, .58, 14}, {.76, .79, 3}, {.90, .94, 5.5}, {1, 1, 1.1},
}

func coasterPosition(elapsed float64) (x, y, z float64) {
	u := max(0, min(1, elapsed/rollerCoasterDuration))
	for i := 0; i < len(coasterKnots)-1; i++ {
		a, b := coasterKnots[i], coasterKnots[i+1]
		if u > b[0] && i < len(coasterKnots)-2 {
			continue
		}
		t := (u - a[0]) / (b[0] - a[0])
		// Monotone cubic interpolation smooths changes in horizontal speed.
		slope := (b[1] - a[1]) / (b[0] - a[0])
		m0, m1 := 0.0, 0.0
		if i > 0 {
			prev := coasterKnots[i-1]
			v := (a[1] - prev[1]) / (a[0] - prev[0])
			m0 = 2 * slope * v / (slope + v)
		}
		if i+2 < len(coasterKnots) {
			next := coasterKnots[i+2]
			v := (next[1] - b[1]) / (next[0] - b[0])
			m1 = 2 * slope * v / (slope + v)
		}
		angle := ((2*t*t*t-3*t*t+1)*a[1] + (t*t*t-2*t*t+t)*(b[0]-a[0])*m0 + (-2*t*t*t+3*t*t)*b[1] + (t*t*t-t*t)*(b[0]-a[0])*m1) * 2 * math.Pi
		y = a[2] + (b[2]-a[2])*t*t*(3-2*t)
		return rollerCoasterX + 42*math.Sin(angle), y, -192 + 27*math.Cos(angle)
	}
	return rollerCoasterX, 1.1, -165
}

func coasterPose(elapsed float64) CoasterPoint {
	elapsed = max(0, min(rollerCoasterDuration, elapsed))
	x, y, z := coasterPosition(elapsed)
	x0, y0, z0 := coasterPosition(max(0, elapsed-.01))
	x1, y1, z1 := coasterPosition(min(rollerCoasterDuration, elapsed+.01))
	return CoasterPoint{T: elapsed, X: x, Y: y, Z: z, Yaw: math.Atan2(x0-x1, z0-z1), Pitch: math.Atan2(y1-y0, math.Hypot(x1-x0, z1-z0)), Bank: .24 * math.Sin(math.Pi*elapsed/rollerCoasterDuration)}
}

var coasterTrack = func() []CoasterPoint {
	points := make([]CoasterPoint, 201)
	for i := range points {
		points[i] = coasterPose(float64(i) * rollerCoasterDuration / 200)
	}
	return points
}()

func coasterReserved(x, z, radius float64) bool {
	return circleRect(x, z, max(.001, radius), 165, -192, 96, 76)
}

func (w *World) seedRollerCoaster() {
	w.nodes = append(w.nodes, &Node{ID: "desert_coaster", Kind: "roller_coaster", Label: "Dune Dipper roller coaster - ride with your camel", X: rollerCoasterX, Z: rollerCoasterZ, Available: true})
	w.layout.Props = append(w.layout.Props, WorldProp{ID: "desert_coaster", Kind: "roller_coaster", X: 165, Z: -192, Width: 84, Depth: 54, Height: 24, Scale: 1})
}

func (w *World) boardRollerCoaster(p *player, target string) error {
	if target != "desert_coaster" || p.coaster != nil || p.disconnected || p.island != "" || p.inCave || p.fallen || !grounded(p) || math.Abs(playerY(p)) > .5 || distance(p.x, p.z, rollerCoasterX, rollerCoasterZ) > 5 {
		return errors.New("Visit the Dune Dipper station in the desert to board on foot or on your camel.")
	}
	if w.time < w.coasterNextBoardAt {
		return errors.New("The previous car is leaving. The next car boards in a moment.")
	}
	w.updateMountedCamel(p)
	saved := camelRaceReturn{x: p.x, y: p.y, z: p.z, yaw: p.yaw, pitch: p.pitch, island: p.island, inCave: p.inCave, ridingCamel: p.ridingCamel, mountedCamelID: p.mountedCamelID, stamina: p.stamina, sprintExhausted: p.sprintExhausted}
	r := &rollerCoasterRide{startedAt: w.time, returnTo: saved}
	if camel := mountedCamel(p); camel != nil {
		r.camelID, r.camelName = camel.ID, camel.Name
	}
	p.coaster = r
	w.coasterNextBoardAt = w.time + 3
	w.clearCookieArenaShots(p.profile.ID)
	w.leaveJohnPorkBehind(p)
	p.posture, p.buildMode, p.editingBuildID, p.bakeryBuildMode, p.editingBakeryFurnitureID = "", false, "", false, ""
	resetMovement(p)
	w.positionCoasterRider(p)
	w.eventActor("All aboard Dune Dipper! Enjoy the lift, hills, and turns. Your camel can ride too. Return to station ends the ride safely.", "notification", p.x, p.z, p.profile.ID)
	return nil
}

func (w *World) positionCoasterRider(p *player) {
	pose := coasterPose(w.time - p.coaster.startedAt)
	p.x, p.y, p.z, p.yaw = pose.X, pose.Y, pose.Z, pose.Yaw
	p.posture = "coaster"
	p.jumpOffset, p.jumpVelocity, p.fallDistance = 0, 0, 0
	p.jumpQueued, p.sprinting = false, false
}

func (w *World) leaveRollerCoaster(p *player) {
	if p.coaster == nil {
		return
	}
	saved := p.coaster.returnTo
	p.coaster = nil
	p.x, p.y, p.z, p.yaw, p.pitch = saved.x, saved.y, saved.z, saved.yaw, saved.pitch
	p.island, p.inCave, p.ridingCamel, p.mountedCamelID = saved.island, saved.inCave, saved.ridingCamel, saved.mountedCamelID
	p.stamina, p.sprintExhausted = saved.stamina, saved.sprintExhausted
	p.fallen, p.standAt, p.fallDistance, p.posture = false, 0, 0, ""
	resetMovement(p)
	w.expireCamels(p)
	if p.ridingCamel && mountedCamel(p) == nil {
		p.ridingCamel, p.mountedCamelID = false, ""
	}
	syncMountedCamel(p)
}

func (w *World) tickRollerCoaster() {
	for _, p := range w.players {
		if p.coaster == nil {
			continue
		}
		if p.disconnected || p.island != "" || p.inCave || w.time-p.coaster.startedAt >= rollerCoasterDuration {
			w.leaveRollerCoaster(p)
			continue
		}
		w.positionCoasterRider(p)
	}
}

func (w *World) coasterCar(p *player) CoasterCar {
	r := p.coaster
	elapsed := max(0, min(rollerCoasterDuration, w.time-r.startedAt))
	return CoasterCar{CoasterPoint: coasterPose(elapsed), ID: "coaster_" + p.profile.ID, PlayerID: p.profile.ID, PlayerName: p.profile.CallName, CamelID: r.camelID, CamelName: r.camelName, StartedAt: r.startedAt, Progress: elapsed / rollerCoasterDuration, Remaining: rollerCoasterDuration - elapsed}
}

func (w *World) rollerCoasterSnapshot(viewer *player) RollerCoasterView {
	view := RollerCoasterView{Station: CoasterPoint{X: rollerCoasterX, Z: rollerCoasterZ}, Duration: rollerCoasterDuration, Track: append([]CoasterPoint{}, coasterTrack...), Cars: []CoasterCar{}, NextBoardIn: max(0, w.coasterNextBoardAt-w.time)}
	if viewer == nil || viewer.island != "" || viewer.inCave {
		return view
	}
	for _, p := range w.players {
		if p.coaster == nil || p.disconnected {
			continue
		}
		car := w.coasterCar(p)
		view.Cars = append(view.Cars, car)
		if p == viewer {
			view.Ride = &car
		}
	}
	sort.Slice(view.Cars, func(i, j int) bool { return view.Cars[i].ID < view.Cars[j].ID })
	// An unoccupied demonstration car runs between rides as a visible invitation.
	if len(view.Cars) == 0 {
		elapsed := math.Mod(w.time, rollerCoasterDuration)
		view.Cars = append(view.Cars, CoasterCar{CoasterPoint: coasterPose(elapsed), ID: "coaster_empty", StartedAt: w.time - elapsed, Progress: elapsed / rollerCoasterDuration, Remaining: rollerCoasterDuration - elapsed})
	}
	return view
}

func (w *World) rollerCoasterAction(p *player, a Action) error {
	if a.Action == "coaster_leave" {
		if p.coaster == nil {
			return errors.New("You are already off the roller coaster.")
		}
		w.leaveRollerCoaster(p)
		w.eventActor("Back at the Dune Dipper station. Thanks for riding!", "notification", p.x, p.z, p.profile.ID)
		return nil
	}
	if a.Action == "coaster_board" {
		return w.boardRollerCoaster(p, a.Target)
	}
	return errors.New("Choose a car at the Dune Dipper station.")
}
