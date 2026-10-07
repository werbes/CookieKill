package game

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Camels retain their identity and parking place independently of their rider.
// RentalUntil is zero for a permanently owned camel. There is no herd size cap.
type Camel struct {
	ID          string  `json:"id"`
	Name        string  `json:"name,omitempty"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Z           float64 `json:"z"`
	Yaw         float64 `json:"yaw"`
	Island      string  `json:"island"`
	InCave      bool    `json:"inCave"`
	RentalUntil int64   `json:"rentalUntil"`
}

func nextCamelID(p *Profile) string {
	used := make(map[string]bool, len(p.Camels))
	for _, c := range p.Camels {
		used[c.ID] = true
	}
	for n := len(p.Camels) + 1; ; n++ {
		id := p.ID + "_camel_" + strconv.Itoa(n)
		if !used[id] {
			return id
		}
	}
}

func camelCompatibility(p *Profile) {
	p.CamelOwned, p.CamelRentalUntil = false, 0
	for _, c := range p.Camels {
		if c.RentalUntil == 0 {
			p.CamelOwned = true
		} else {
			p.CamelRentalUntil = max(p.CamelRentalUntil, c.RentalUntil)
		}
	}
}

func normalizeCamels(p *Profile) {
	// Old saves represented a single companion with a boolean/expiry. Park that
	// companion beside Abu Fanous so it remains reachable after migration.
	if p.Camels == nil || len(p.Camels) == 0 && (p.CamelOwned || p.CamelRentalUntil > 0) {
		p.Camels = []Camel{}
		if p.CamelOwned || p.CamelRentalUntil > 0 {
			until := p.CamelRentalUntil
			if p.CamelOwned {
				until = 0
			}
			p.Camels = append(p.Camels, Camel{ID: nextCamelID(p), X: 224, Z: -126, RentalUntil: until})
		}
	}
	seen := map[string]bool{}
	for i := range p.Camels {
		c := &p.Camels[i]
		if name, err := objectName(c.Name, 24); err == nil {
			c.Name = name
		} else {
			c.Name = ""
		}
		if !strings.HasPrefix(c.ID, p.ID+"_camel_") || seen[c.ID] {
			c.ID = nextCamelID(p)
		}
		seen[c.ID] = true
		if !finite(c.X) || !finite(c.Y) || !finite(c.Z) || !finite(c.Yaw) ||
			c.X < MinX || c.X > MaxX || c.Z < MinZ || c.Z > MaxZ ||
			c.Island != "" && (math.Hypot(c.X, c.Z) > 40 || c.Y < 0 || c.Y > 15) ||
			c.Island == "" && (waterAt(c.X, c.Z) || c.InCave && !caveFootprint(c.X, c.Z)) {
			c.X, c.Y, c.Z, c.Yaw, c.Island, c.InCave = 224, 0, -126, 0, "", false
		}
		if c.Island != "" {
			c.InCave = false
		} else if c.InCave {
			c.Y = caveGroundY(c.X, c.Z)
		} else {
			c.Y = 0
		}
		c.RentalUntil = max(0, c.RentalUntil)
	}
	camelCompatibility(p)
}

func mountedCamel(p *player) *Camel {
	if !p.ridingCamel {
		return nil
	}
	for i := range p.profile.Camels {
		if p.profile.Camels[i].ID == p.mountedCamelID {
			return &p.profile.Camels[i]
		}
	}
	return nil
}

func syncMountedCamel(p *player) {
	if c := mountedCamel(p); c != nil {
		c.X, c.Y, c.Z, c.Yaw = p.x, groundY(p), p.z, p.yaw
		c.Island, c.InCave = p.island, p.inCave
	}
}

func (w *World) expireCamels(p *player) {
	if p.profile.CamelRentalUntil == 0 || p.profile.CamelRentalUntil > w.now().Unix() {
		return
	}
	kept := p.profile.Camels[:0]
	for _, c := range p.profile.Camels {
		if c.RentalUntil > 0 && c.RentalUntil <= w.now().Unix() {
			if c.ID == p.mountedCamelID {
				p.ridingCamel, p.mountedCamelID = false, ""
			}
			continue
		}
		kept = append(kept, c)
	}
	p.profile.Camels = kept
	camelCompatibility(&p.profile)
}

func (w *World) updateMountedCamel(p *player) {
	// Also supports an in-memory legacy companion during a rolling migration.
	if p.ridingCamel && p.mountedCamelID == "" {
		normalizeCamels(&p.profile)
		if len(p.profile.Camels) > 0 {
			p.mountedCamelID = p.profile.Camels[0].ID
		}
	}
	w.expireCamels(p)
	if !p.ridingCamel {
		return
	}
	if mountedCamel(p) == nil || inWater(p) {
		// The camel waits at its last dry position if the rider enters water.
		p.ridingCamel, p.mountedCamelID = false, ""
		return
	}
	syncMountedCamel(p)
}

func (w *World) dismountCamel(p *player) {
	syncMountedCamel(p)
	if c := mountedCamel(p); c != nil {
		// Leave room to see and approach the camel again. Spread a parked herd
		// around the rider, retaining the original spot if nearby land is blocked.
		for attempt := 0; attempt < len(p.profile.Camels)+12; attempt++ {
			angle := p.yaw + float64(attempt)*2.4
			radius := 2.2 + math.Sqrt(float64(attempt))*.8
			x, z := p.x+math.Cos(angle)*radius, p.z+math.Sin(angle)*radius
			candidate := *p
			candidate.x, candidate.z = x, z
			if p.island != "" {
				if w.islandBlocked(&candidate, x, z) {
					continue
				}
				candidate.y = w.islandGround(&candidate)
			} else if inWater(&candidate) || p.inCave && !caveFootprint(x, z) || !p.inCave && caveMouth(x, z) ||
				w.solidBlockedAt(x, groundY(&candidate), z, .6) || w.homeBlocked(&candidate, x, z) {
				continue
			}
			if math.Abs(groundY(&candidate)-groundY(p)) > .5 {
				continue
			}
			occupied := false
			for _, other := range p.profile.Camels {
				if other.ID != c.ID && other.Island == p.island && other.InCave == p.inCave && distance(x, z, other.X, other.Z) < 2 {
					occupied = true
					break
				}
			}
			if !occupied {
				c.X, c.Y, c.Z = x, groundY(&candidate), z
				break
			}
		}
	}
	p.ridingCamel, p.mountedCamelID = false, ""
	resetMovement(p)
}

func (w *World) mountCamel(p *player, target string) error {
	w.expireCamels(p)
	if p.ridingCamel {
		return errors.New("Press Q to dismount your current camel first.")
	}
	if inWater(p) || !grounded(p) {
		return errors.New("Stand on dry land to mount your camel.")
	}
	var nearest *Camel
	best := 5.0
	for i := range p.profile.Camels {
		c := &p.profile.Camels[i]
		if target != "" && c.ID != target || c.Island != p.island || c.InCave != p.inCave || math.Abs(c.Y-playerY(p)) > 2 {
			continue
		}
		if d := distance(p.x, p.z, c.X, c.Z); d <= best {
			nearest, best = c, d
		}
	}
	if nearest == nil {
		return errors.New("Move within 5 metres of one of your camels and press E to mount.")
	}
	// Nearby is not sufficient when a wall separates rider and camel. Check the
	// short approach so mounting cannot bypass houses or placed island walls.
	candidate := *p
	steps := max(1, int(math.Ceil(best/.25)))
	for step := 1; step <= steps; step++ {
		t := float64(step) / float64(steps)
		x, z := p.x+(nearest.X-p.x)*t, p.z+(nearest.Z-p.z)*t
		if p.island != "" {
			if w.islandBlocked(&candidate, x, z) {
				return errors.New("Walk around the obstacle to reach your camel.")
			}
			candidate.x, candidate.z = x, z
			candidate.y = w.islandGround(&candidate)
		} else {
			if !w.moveMainland(&candidate, x, z, groundY(&candidate)) || candidate.inCave != p.inCave || waterAt(x, z) {
				return errors.New("Walk around the obstacle to reach your camel.")
			}
			candidate.x, candidate.z = x, z
		}
	}
	p.x, p.y, p.z, p.yaw = nearest.X, nearest.Y, nearest.Z, nearest.Yaw
	p.posture, p.ridingCamel, p.mountedCamelID = "", true, nearest.ID
	resetMovement(p)
	return nil
}

func (w *World) camelAction(p *player, a Action) error {
	switch a.Action {
	case "camel_dismount":
		w.dismountCamel(p)
		return nil
	case "camel_mount":
		return w.mountCamel(p, a.Target)
	case "camel_ride": // Older clients still get proximity-checked mounting.
		if p.ridingCamel {
			w.dismountCamel(p)
			return nil
		}
		return w.mountCamel(p, a.Target)
	}
	if !w.near(p, "abu_fanous", 7) {
		return errors.New("Visit Abu Fanous on the far eastern edge of Sunbaked Sands.")
	}
	w.expireCamels(p)
	switch a.Action {
	case "camel_gift":
		if p.profile.CamelDiscount {
			return errors.New("Abu Fanous has already given you his permanent discount.")
		}
		for _, r := range recipes {
			if p.profile.Inventory[r.ID] < 1 {
				return errors.New("Abu Fanous would love one of every kind of cookie and cake.")
			}
		}
		for _, r := range recipes {
			take(p, r.ID, 1)
		}
		p.profile.CamelDiscount = true
		return nil
	case "camel_rent", "camel_buy":
		cost, until := 350, int64(0)
		if p.profile.CamelDiscount {
			cost = 175
		}
		if a.Action == "camel_rent" {
			if p.profile.CamelRentalUntil > w.now().Unix() {
				return errors.New("Your current camel rental is still active.")
			}
			cost, until = 50, w.now().Unix()+1200
		}
		if p.profile.Coins < cost {
			return fmt.Errorf("This camel costs %d coins.", cost)
		}
		w.dismountCamel(p)
		p.profile.Coins -= cost
		c := Camel{ID: nextCamelID(&p.profile), X: p.x, Y: groundY(p), Z: p.z, Yaw: p.yaw, Island: p.island, InCave: p.inCave, RentalUntil: until}
		p.profile.Camels = append(p.profile.Camels, c)
		camelCompatibility(&p.profile)
		p.ridingCamel, p.mountedCamelID, p.posture = true, c.ID, ""
		return nil
	}
	return errors.New("Choose a camel rental, purchase, or gift.")
}

func (w *World) camelNodes(viewer *player) []Node {
	nodes := []Node{}
	for owner, profile := range w.profileViews() {
		for _, c := range profile.Camels {
			if c.Island != viewer.island || c.RentalUntil > 0 && c.RentalUntil <= w.now().Unix() {
				continue
			}
			if rider := w.players[owner]; rider != nil && rider.ridingCamel && rider.mountedCamelID == c.ID {
				continue
			}
			label := profile.CallName + "'s camel"
			if owner == viewer.profile.ID {
				label = "Your camel"
			}
			if c.Name != "" {
				label = c.Name
			}
			nodes = append(nodes, Node{ID: c.ID, Kind: "camel", Label: label, Owner: owner, X: c.X, Y: c.Y, Z: c.Z, Yaw: c.Yaw, InCave: c.InCave, Island: c.Island, Available: true})
		}
	}
	return nodes
}
