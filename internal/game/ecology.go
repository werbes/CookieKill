package game

import (
	"fmt"
	"math"
	"math/rand/v2"
)

type animalSpec struct {
	species                            string
	count, minGroup, maxGroup          int
	injury, speedMin, speedMax, health float64
}

var animalSpecs = []animalSpec{
	{"turtle", 20, 1, 3, .15, 1.5, 2.1, 72},
	{"dolphin", 20, 1, 4, .05, 6, 8, 108},
	{"fish", 40, 2, 6, .15, 2.2, 5.4, 22},
	{"sea_lion", 20, 1, 3, .10, 3.8, 4.2, 135},
	{"shark", 20, 1, 1, .05, 6, 6, 175},
	{"whale", 20, 1, 2, .10, 2.2, 2.8, 450},
}

func (w *World) seedAnimals() {
	rng := rand.New(rand.NewPCG(8413, 91832))
	for speciesIndex, spec := range animalSpecs {
		start := len(w.animals)
		index := 0
		group := 0
		for index < spec.count {
			count := spec.minGroup + rng.IntN(spec.maxGroup-spec.minGroup+1)
			if spec.species == "turtle" {
				count = 1 + rng.IntN(2)
				if rng.IntN(6) == 0 {
					count = 3
				}
			}
			if count > spec.count-index {
				count = spec.count - index
			}
			if spec.species == "fish" && spec.count-index-count == 1 {
				if count < 6 {
					count++
				} else {
					count--
				}
			}
			groupID := fmt.Sprintf("%s_pod_%d", spec.species, group)
			group++
			cx := -27 - rng.Float64()*177
			cz := 76 + rng.Float64()*83
			radius := 9 + rng.Float64()*10
			speed := spec.speedMin + rng.Float64()*(spec.speedMax-spec.speedMin)
			phase := rng.Float64() * math.Pi * 2
			for j := 0; j < count; j++ {
				id := fmt.Sprintf("animal_%s_%d", spec.species, index)
				if index < 2 {
					id = fmt.Sprintf("animal_%d", speciesIndex+index*6)
				}
				scale := .84 + rng.Float64()*.28
				spacing := 1.8
				if spec.species == "whale" {
					spacing = 6
				}
				if spec.species == "fish" {
					spacing = .8
				}
				angle := phase + float64(j)*spacing/radius
				x, z := cx+math.Sin(angle)*radius, cz+math.Cos(angle)*radius
				hp := math.Round(spec.health * scale * scale)
				w.animals = append(w.animals, &Animal{ID: id, Species: spec.species, X: x, Z: z, Y: .02, Scale: scale, Health: hp, MaxHealth: hp, Disposition: "calm", Group: groupID, GroupID: groupID, Speed: speed, Heading: angle + math.Pi/2, Yaw: angle + math.Pi/2, groupX: cx, groupZ: cz, orbitRadius: radius, baseSpeed: speed, injuryRate: spec.injury, homeX: x, homeZ: z, phase: angle, needAt: 45 + rng.Float64()*140, rewardAt: map[string]float64{}})
				index++
			}
		}
		// Choose a representative small fraction, not every animal in a pod.
		indices := rng.Perm(spec.count)
		for _, i := range indices[:int(math.Round(float64(spec.count)*spec.injury))] {
			a := w.animals[start+i]
			w.injureAnimal(a)
		}
	}
}
func (w *World) injureAnimal(a *Animal) {
	a.Need = "wounded"
	a.Health = math.Min(a.Health, math.Round(a.MaxHealth*.7))
	if a.Species == "turtle" && int(a.phase*100)%2 == 0 {
		a.Need = "trapped"
		a.Health = a.MaxHealth
	}
	a.needAt = w.time + 120
}
func (w *World) renewAnimalNeeds(dt float64) {
	if int(w.time/30) == int((w.time-dt)/30) {
		return
	}
	for _, spec := range animalSpecs {
		live := []*Animal{}
		injured := 0
		for _, a := range w.animals {
			if a.Species == spec.species && a.Health > 0 {
				live = append(live, a)
				if a.Need != "" {
					injured++
				}
			}
		}
		target := int(math.Round(float64(len(live)) * spec.injury))
		if injured >= target || len(live) == 0 {
			continue
		}
		offset := int(w.time/30) % len(live)
		for i := 0; i < len(live) && injured < target; i++ {
			a := live[(i+offset)%len(live)]
			if a.Need == "" && a.needAt <= w.time {
				w.injureAnimal(a)
				injured++
			}
		}
	}
}

func (w *World) moveAnimals(dt float64) {
	w.renewAnimalNeeds(dt)
	for _, a := range w.animals {
		if a.Health <= 0 {
			if a.respawnAt <= w.time {
				a.Health = a.MaxHealth
				a.Need = ""
				a.X = a.homeX
				a.Z = a.homeZ
				a.needAt = w.time + 90
				a.Disposition = "calm"
			}
			continue
		}
		speed := a.baseSpeed
		if speed == 0 {
			speed = 4
		} // Supports hand-made test animals.
		a.Y = .02
		a.Speed = speed
		if a.Species == "dolphin" {
			jump := math.Mod(w.time+a.groupX*.13+100, 14)
			if jump > 12 {
				a.Y = math.Sin((jump-12)*math.Pi/2) * 1.7
				a.Speed = speed * .75
			}
		}
		var threat *player
		closest := 35.0
		for _, p := range w.players {
			d := distance(p.x, p.z, a.X, a.Z)
			if inWater(p) && p.profile.Reputation[a.ID] < 0 && d < closest {
				threat = p
				closest = d
			}
		}
		oldX, oldZ := a.X, a.Z
		if threat != nil {
			dx, dz := threat.x-a.X, threat.z-a.Z
			length := math.Max(.001, math.Hypot(dx, dz))
			direction := -1.0
			predator := a.Species == "shark" || a.Species == "sea_lion"
			if predator {
				direction = 1
				a.Disposition = "hunting"
				if length < 2.0 && a.attackAt <= w.time {
					a.attackAt = w.time + 1.4
					w.damagePlayer(nil, threat, 12, "a "+a.Species)
				}
			} else {
				a.Disposition = "fleeing"
			}
			if a.Need != "trapped" {
				a.X += dx / length * dt * a.Speed * direction
				a.Z += dz / length * dt * a.Speed * direction
			}
		} else {
			a.Disposition = "calm"
			if a.Need != "trapped" {
				radius := math.Max(1, a.orbitRadius)
				angle := a.phase + w.time*a.baseSpeed/radius
				tx, tz := a.groupX+math.Sin(angle)*radius, a.groupZ+math.Cos(angle)*radius
				dx, dz := tx-a.X, tz-a.Z
				length := math.Hypot(dx, dz)
				if length > .0001 {
					step := math.Min(length, dt*a.Speed)
					a.X += dx / length * step
					a.Z += dz / length * step
				}
			}
		}
		a.X = math.Max(MinX+8, math.Min(-9, a.X))
		a.Z = math.Max(coastZ(a.X)+17, math.Min(MaxZ-7, a.Z))
		if distance(oldX, oldZ, a.X, a.Z) > .00001 {
			a.Heading = math.Atan2(-(a.X - oldX), -(a.Z - oldZ))
			a.Yaw = a.Heading
		}
		// School fish share an exact nominal swimming speed; each school varies.
		if a.Need == "trapped" {
			a.Speed = 0
		}
	}
}
