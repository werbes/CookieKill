package game

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
)

type animalSpec struct {
	species                            string
	count, minGroup, maxGroup          int
	injury, speedMin, speedMax, health float64
}

// More wildlife, with the same small, independently moving schools and pods.
var animalSpecs = []animalSpec{
	{"turtle", 9, 1, 3, .15, 1.5, 2.1, 72},
	{"dolphin", 9, 2, 3, .05, 6, 8, 108},
	{"fish", 18, 3, 6, .15, 2.2, 5.4, 22},
	{"sea_lion", 9, 2, 3, .10, 3.8, 4.2, 135},
	{"shark", 6, 1, 1, .05, 6, 6, 175},
	{"whale", 6, 1, 2, .10, 2.2, 2.8, 450},
}

const eggHatchSeconds = 120.0
const untreatedSeconds = 180.0

func (w *World) seedAnimals() {
	rng := rand.New(rand.NewPCG(8413, 91832))
	for speciesIndex, spec := range animalSpecs {
		start, index, group := len(w.animals), 0, 0
		for index < spec.count {
			count := min(spec.count-index, spec.minGroup+rng.IntN(spec.maxGroup-spec.minGroup+1))
			if remaining := spec.count - index - count; remaining > 0 && remaining < spec.minGroup {
				if spec.count-index <= spec.maxGroup {
					count = spec.count - index
				} else {
					count -= spec.minGroup - remaining
				}
			}
			groupID := fmt.Sprintf("%s_pod_%d", spec.species, group)
			group++
			cx, cz := -27-rng.Float64()*177, 76+rng.Float64()*83
			radius, phase := 9+rng.Float64()*10, rng.Float64()*math.Pi*2
			for j := 0; j < count; j++ {
				id := fmt.Sprintf("animal_%s_%d", spec.species, index)
				if index < 2 {
					id = fmt.Sprintf("animal_%d", speciesIndex+index*6)
				}
				scale := .84 + rng.Float64()*.28
				speed := spec.speedMin + rng.Float64()*(spec.speedMax-spec.speedMin)
				offset := float64(j) * 2 * math.Pi / float64(count)
				x, z := cx+math.Sin(phase)*radius+.4*math.Sin(offset), cz+math.Cos(phase)*radius+.4*math.Cos(offset)
				hp := math.Round(spec.health * scale * scale)
				w.animals = append(w.animals, &Animal{ID: id, Species: spec.species, X: x, Z: z, Y: .02, Scale: scale, Health: hp, MaxHealth: hp, Disposition: "calm", Group: groupID, GroupID: groupID, Speed: speed, Heading: phase + math.Pi/2, Yaw: phase + math.Pi/2, groupX: cx, groupZ: cz, orbitRadius: radius, baseSpeed: speed, injuryRate: spec.injury, homeX: x, homeZ: z, phase: phase, wanderPhase: rng.Float64() * math.Pi * 2, jumpPhase: float64(index) * 18 / float64(spec.count), Age: rng.Float64() * 600, lifespan: 1200 + rng.Float64()*1200, needAt: 45 + rng.Float64()*140, rewardAt: map[string]float64{}})
				index++
			}
		}
		indices := rng.Perm(spec.count)
		for _, i := range indices[:int(math.Round(float64(spec.count)*spec.injury))] {
			w.injureAnimal(w.animals[start+i])
		}
	}
}

func (w *World) injureAnimal(a *Animal) {
	a.Need = "wounded"
	a.Health = math.Min(a.Health, math.Round(a.MaxHealth*.7))
	if a.Species == "turtle" && int(a.wanderPhase*100)%2 == 0 {
		a.Need, a.Health = "trapped", a.MaxHealth
	}
	a.needAt, a.sickUntil = w.time+120, w.time+untreatedSeconds
}

func (w *World) renewAnimalNeeds(dt float64) {
	if int(w.time/30) == int((w.time-dt)/30) {
		return
	}
	for _, spec := range animalSpecs {
		live := []*Animal{}
		injured := 0
		for _, a := range w.animals {
			if a.Species == spec.species && a.Health > 0 && !a.Egg {
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

func (w *World) disturbEgg(p *player, a *Animal) error {
	if p.island != "" || a.Health <= 0 || distance(p.x, p.z, a.X, a.Z) > 5 {
		return errors.New("Swim closer to that egg.")
	}
	a.eggSince, a.HatchIn = w.time, eggHatchSeconds
	p.cooldowns["interact"] = w.time + .5
	w.eventActor("The egg was disturbed. Give it two quiet minutes to hatch.", "animal", a.X, a.Z, p.profile.ID)
	return nil
}

func (w *World) beginEgg(a *Animal) {
	a.Egg, a.Health, a.Need, a.Age = true, 1, "", 0
	a.Speed, a.Y, a.Disposition = 0, .02, "nesting"
	a.eggSince, a.HatchIn, a.sickUntil = w.time, eggHatchSeconds, 0
	a.rewardAt = map[string]float64{}
	// A replacement is a new animal: trust and grudges are not inherited.
	for _, p := range w.players {
		delete(p.profile.Reputation, a.ID)
	}
	for _, p := range w.offline {
		delete(p.profile.Reputation, a.ID)
	}
	for id, p := range w.profiles {
		delete(p.Reputation, a.ID)
		w.profiles[id] = p
	}
}

func (w *World) hatchEgg(a *Animal) {
	a.Egg, a.Health, a.Need, a.Disposition = false, a.MaxHealth, "", "calm"
	a.Age, a.HatchIn, a.needAt = 0, 0, w.time+180
	// Hatch where the egg rested; a distant pod must not pull it across the ocean.
	a.GroupID = fmt.Sprintf("%s_hatch_%d", a.ID, int(w.time))
	a.Group = a.GroupID
	a.phase = -w.time * a.baseSpeed / math.Max(1, a.orbitRadius)
	a.groupX, a.groupZ = a.X, a.Z-math.Max(1, a.orbitRadius)
	if a.lifespan <= 0 {
		a.lifespan = 1800
	}
	w.event("A "+a.Species+" egg hatched!", "animal", a.X, a.Z)
}

func (w *World) moveAnimals(dt float64) {
	w.renewAnimalNeeds(dt)
	groups := map[string][]*Animal{}
	old := map[*Animal]Point{}
	for _, a := range w.animals {
		if a.Health <= 0 {
			if a.respawnAt <= w.time {
				w.beginEgg(a)
			}
			continue
		}
		if a.Egg {
			for _, p := range w.players {
				if p.island == "" && distance(p.x, p.z, a.X, a.Z) < 1.5 {
					a.eggSince = w.time
				}
			}
			a.HatchIn = math.Max(0, eggHatchSeconds-(w.time-a.eggSince))
			if a.HatchIn <= 0 {
				w.hatchEgg(a)
			}
			continue
		}
		a.Age += dt
		if a.Need == "" {
			a.sickUntil = 0
		} else if a.sickUntil == 0 {
			a.sickUntil = w.time + untreatedSeconds
		}
		if (a.lifespan > 0 && a.Age >= a.lifespan) || (a.sickUntil > 0 && a.sickUntil <= w.time) {
			a.Health, a.Speed, a.Need, a.respawnAt = 0, 0, "", w.time+30
			continue
		}
		groups[a.GroupID] = append(groups[a.GroupID], a)
		old[a] = Point{a.X, a.Z}
	}
	for _, group := range groups {
		average := 0.0
		for _, a := range group {
			average += a.baseSpeed
		}
		average /= float64(len(group))
		for _, a := range group {
			speed := a.baseSpeed
			if speed == 0 {
				speed = 4
			}
			a.Speed = speed * (.9 + .1*math.Sin(w.time*.8+a.wanderPhase))
			a.Y = .02
			if a.Species == "dolphin" {
				jump := math.Mod(w.time+a.jumpPhase, 18)
				if jump > 16 {
					a.Y = math.Sin((jump-16)*math.Pi/2) * 1.7
					a.Speed = speed * .75
				}
			}
			var threat *player
			closest := 35.0
			for _, p := range w.players {
				d := distance(p.x, p.z, a.X, a.Z)
				if p.island == "" && inWater(p) && p.profile.Reputation[a.ID] < 0 && d < closest {
					threat, closest = p, d
				}
			}
			if a.Need == "trapped" {
				a.Speed = 0
				continue
			}
			radius := math.Max(1, a.orbitRadius)
			angle := a.phase + w.time*average/radius
			tx := a.groupX + math.Sin(angle)*radius + .6*math.Sin(w.time*.67+a.wanderPhase)
			tz := a.groupZ + math.Cos(angle)*radius + .6*math.Cos(w.time*.91+a.wanderPhase)
			a.Disposition = "calm"
			if threat != nil {
				dx, dz := threat.x-a.X, threat.z-a.Z
				direction := -1.0
				if a.Species == "shark" || a.Species == "sea_lion" {
					direction = 1
					a.Disposition = "hunting"
					if math.Hypot(dx, dz) < 2 && a.attackAt <= w.time {
						a.attackAt = w.time + 1.4
						w.damagePlayer(nil, threat, 12, "a "+a.Species)
					}
				} else {
					a.Disposition = "fleeing"
				}
				tx, tz = a.X+dx*direction, a.Z+dz*direction
			}
			dx, dz := tx-a.X, tz-a.Z
			length := math.Hypot(dx, dz)
			if length > .0001 {
				step := math.Min(length, dt*a.Speed)
				a.X += dx / length * step
				a.Z += dz / length * step
			}
			a.X = math.Max(MinX+8, math.Min(-9, a.X))
			a.Z = math.Max(coastZ(a.X)+17, math.Min(MaxZ-7, a.Z))
		}
		// A connected chain keeps each member within one metre of a peer without
		// assigning rigid rows, a common speed, or synchronized headings.
		for i, a := range group {
			if a.Need == "trapped" {
				group[0], group[i] = a, group[0]
				break
			}
		}
		for i := 1; i < len(group); i++ {
			a := group[i]
			peer := group[0]
			nearest := distance(a.X, a.Z, peer.X, peer.Z)
			for _, b := range group[:i] {
				if d := distance(a.X, a.Z, b.X, b.Z); d < nearest {
					peer, nearest = b, d
				}
			}
			if nearest > .95 && a.Need != "trapped" {
				a.X = peer.X + (a.X-peer.X)*.95/nearest
				a.Z = peer.Z + (a.Z-peer.Z)*.95/nearest
			}
		}
		for _, a := range group {
			pos := old[a]
			if distance(pos.X, pos.Z, a.X, a.Z) > .00001 {
				a.Heading = math.Atan2(-(a.X - pos.X), -(a.Z - pos.Z))
				a.Yaw = a.Heading
			}
		}
	}
}
