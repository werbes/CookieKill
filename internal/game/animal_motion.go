package game

import (
	"math"
	"strings"
)

// AnimalMotion is an immutable description of an undisturbed swim route.
// Epoch and HatchAt use the same world-seconds clock as Snapshot.Time. The
// server samples the same route as browsers and remains authoritative for
// collisions, damage, interactions, aging, and changes of behavior.
type AnimalMotion struct {
	Kind         string  `json:"kind"`
	Epoch        float64 `json:"epoch"`
	Transition   float64 `json:"transition"`
	FromX        float64 `json:"fromX"`
	FromZ        float64 `json:"fromZ"`
	Heading      float64 `json:"heading"`
	CenterX      float64 `json:"centerX"`
	CenterZ      float64 `json:"centerZ"`
	Radius       float64 `json:"radius"`
	Phase        float64 `json:"phase"`
	AngularSpeed float64 `json:"angularSpeed"`
	WanderPhase  float64 `json:"wanderPhase"`
	JumpPhase    float64 `json:"jumpPhase"`
	Speed        float64 `json:"speed"`
	Age          float64 `json:"age"`
	HatchAt      float64 `json:"hatchAt"`
}

// AnimalPose is the display and collision pose at a point on a route.
type AnimalPose struct {
	X, Y, Z, Yaw, Heading, Speed, Age, HatchIn float64
}

func (m AnimalMotion) Sample(species string, egg bool, now float64) AnimalPose {
	u := math.Max(0, now-m.Epoch)
	p := AnimalPose{X: m.FromX, Y: .02, Z: m.FromZ, Heading: m.Heading, Yaw: m.Heading, Age: m.Age}
	if egg {
		p.HatchIn = math.Max(0, m.HatchAt-now)
	} else {
		p.Age += u
	}
	if m.Kind != "orbit" {
		return p
	}
	theta := m.Phase + m.AngularSpeed*u
	tx := m.CenterX + math.Sin(theta)*m.Radius + .3*math.Sin(now*.67+m.WanderPhase)
	tz := m.CenterZ + math.Cos(theta)*m.Radius + .3*math.Cos(now*.91+m.WanderPhase)
	vx := math.Cos(theta)*m.Radius*m.AngularSpeed + .201*math.Cos(now*.67+m.WanderPhase)
	vz := -math.Sin(theta)*m.Radius*m.AngularSpeed - .273*math.Sin(now*.91+m.WanderPhase)
	alpha, derivative := 1.0, 0.0
	if m.Transition > 0 && u < m.Transition {
		q := u / m.Transition
		alpha, derivative = q*q*(3-2*q), 6*q*(1-q)/m.Transition
	}
	p.X, p.Z = m.FromX+(tx-m.FromX)*alpha, m.FromZ+(tz-m.FromZ)*alpha
	vx, vz = vx*alpha+(tx-m.FromX)*derivative, vz*alpha+(tz-m.FromZ)*derivative
	if math.Hypot(vx, vz) > .00001 {
		p.Heading = math.Atan2(-vx, -vz)
		p.Yaw = p.Heading
	}
	p.Speed = m.Speed * (.9 + .1*math.Sin(now*.8+m.WanderPhase))
	if species == "dolphin" {
		jump := math.Mod(now+m.JumpPhase, 18)
		if jump > 16 {
			p.Y = math.Sin((jump-16)*math.Pi/2) * 1.7
			p.Speed = m.Speed * .75
		}
	}
	return p
}

func (w *World) stillAnimalMotion(a *Animal) *AnimalMotion {
	m := &AnimalMotion{Kind: "still", Epoch: w.time, FromX: a.X, FromZ: a.Z, Heading: a.Heading, Age: a.Age}
	if a.Egg {
		m.HatchAt = a.eggSince + eggHatchSeconds
	}
	return m
}

// An entire pod switches to authoritative movement when a member responds to
// a swimmer, because the pod's cohesion also changes its peers' positions.
func (w *World) reactiveAnimalGroup(group []*Animal) bool {
	for _, p := range w.players {
		if p.island != "" || p.inCave || !inWater(p) {
			continue
		}
		for _, a := range group {
			rep := p.profile.Reputation[a.Species]
			d := distance(p.x, p.z, a.X, a.Z)
			if rep < 0 && d < 35 || rep > 0 && d < 25 {
				return true
			}
		}
	}
	return false
}

func (w *World) planAnimalGroup(group []*Animal, average float64) {
	var key strings.Builder
	var trapped *Animal
	for _, a := range group {
		key.WriteString(a.ID)
		key.WriteByte('/')
		if a.Need == "trapped" {
			trapped = a
			key.WriteString("trapped/")
		}
	}
	groupKey := key.String()
	valid := true
	for _, a := range group {
		if a.Motion == nil || a.motionGroup != groupKey || a.X != a.motionX || a.Z != a.motionZ {
			valid = false
			break
		}
	}
	if valid {
		return
	}
	lead := group[0]
	radius := math.Max(1, math.Min(30, lead.orbitRadius))
	// Every point of the orbit (including individual wander) is in open water.
	// Keeping the whole circle inside these bounds avoids client-specific coast
	// clipping and ensures the analytic trajectory is identical on both sides.
	cx := math.Max(MinX+8+radius+.3, math.Min(-9-radius-.3, lead.groupX))
	cz := math.Max(65+radius+.3, math.Min(MaxZ-7-radius-.3, lead.groupZ))
	for _, a := range group {
		if a.Z >= 65 {
			continue
		}
		// A pod returning from the shallows establishes its patrol locally.
		// Otherwise a diagonal back to a distant home circle can cross a
		// coastal bend. |coastZ'(x)| <= 11*.021+5*.057 = .516, so raising
		// the whole destination circle by this bound keeps every interpolated
		// point offshore without requiring a terrain sampler in the browser.
		cx = math.Max(MinX+8+radius+.3, math.Min(-9-radius-.3, lead.X))
		for _, member := range group {
			span := math.Abs(cx-member.X) + radius + .3
			cz = math.Max(cz, coastZ(member.X)+3+.516*span+radius+.3)
		}
		break
	}
	phase, rate := lead.phase+w.time*average/radius, average/radius
	if trapped != nil {
		cx, cz, radius, rate = trapped.X, trapped.Z, 0, 0
	}
	transition := 2.0
	for _, a := range group {
		speed := math.Max(1, a.baseSpeed)
		tx, tz := cx+math.Sin(phase)*radius, cz+math.Cos(phase)*radius
		transition = math.Max(transition, distance(a.X, a.Z, tx, tz)/speed)
	}
	for _, a := range group {
		m := w.stillAnimalMotion(a)
		if a.Need != "trapped" {
			m.Kind, m.Transition, m.CenterX, m.CenterZ = "orbit", transition, cx, cz
			m.Radius, m.Phase, m.AngularSpeed = radius, phase, rate
			m.WanderPhase, m.JumpPhase, m.Speed = a.wanderPhase, a.jumpPhase, a.baseSpeed
			if m.Speed == 0 {
				m.Speed = 4
			}
		}
		a.Motion, a.motionGroup, a.Disposition = m, groupKey, "calm"
	}
}

func sampleAnimalMotion(a *Animal, now float64) {
	p := a.Motion.Sample(a.Species, a.Egg, now)
	a.X, a.Y, a.Z, a.Heading, a.Yaw, a.Speed = p.X, p.Y, p.Z, p.Heading, p.Yaw, p.Speed
	a.motionX, a.motionZ = a.X, a.Z
}
