package game

import (
	"errors"
	"fmt"
	"sort"
)

func (w *World) restoreProperties() {
	ids := []string{}
	for id := range w.profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	claimed := map[string]bool{}
	for _, id := range ids {
		p := w.profiles[id]
		if p.Bakery {
			if !w.isPlot(p.BakeryPlot) || claimed[p.BakeryPlot] {
				p.BakeryPlot = ""
				for _, n := range w.nodes {
					if n.Kind == "bakery_plot" && !claimed[n.ID] {
						p.BakeryPlot = n.ID
						break
					}
				}
			}
			if p.BakeryPlot != "" {
				claimed[p.BakeryPlot] = true
			}
		}
		if p.Home != nil {
			h := p.Home
			if !finite(h.X) || !finite(h.Z) || buildZone(h.X, h.Z) == "" || !validChoice(h.Kind, "tent", "cabin") || h.Health <= 0 {
				p.Home = nil
			} else {
				h.Owner = id
				h.CallName = p.CallName
				h.Username = p.Username
				h.MaxHealth = homeHealth(h.Kind)
				h.Health = min(h.Health, h.MaxHealth)
				h.Safe = h.Safe && w.safeSpace(h.X, h.Z, 8)
			}
		}
		w.profiles[id] = p
	}
}
func (w *World) isPlot(id string) bool {
	for _, n := range w.nodes {
		if n.ID == id && n.Kind == "bakery_plot" {
			return true
		}
	}
	return false
}
func (w *World) plotOwner(id string) string {
	if id == "" {
		return ""
	}
	for owner, p := range w.profileViews() {
		if p.Bakery && p.BakeryPlot == id {
			return owner
		}
	}
	return ""
}
func (w *World) homes() []Home {
	out := []Home{}
	w.eachHome(func(h Home) bool { out = append(out, h); return false })
	sort.Slice(out, func(i, j int) bool { return out[i].Owner < out[j].Owner })
	return out
}

// Collision checks run many times each tick. Visit homes directly without
// constructing a map containing every account's full inventory on each check.
func (w *World) eachHome(visit func(Home) bool) bool {
	inspect := func(id string, home *Home, callName, username string) bool {
		if home == nil || home.Health <= 0 {
			return false
		}
		h := *home
		h.Owner = id
		h.CallName = callName
		h.Username = username
		return visit(h)
	}
	for id, p := range w.players {
		if inspect(id, p.profile.Home, p.profile.CallName, p.profile.Username) {
			return true
		}
	}
	for id, p := range w.offline {
		if inspect(id, p.profile.Home, p.profile.CallName, p.profile.Username) {
			return true
		}
	}
	for id, p := range w.profiles {
		if w.players[id] != nil || w.offline[id] != nil {
			continue
		}
		if inspect(id, p.Home, p.CallName, p.Username) {
			return true
		}
	}
	return false
}
func homeHealth(kind string) float64 {
	if kind == "cabin" {
		return 260
	}
	return 120
}
func homeRadius(kind string) float64 {
	if kind == "cabin" {
		return 3.7
	}
	return 3.2
}

// Internal read-only views avoid cloning every inventory for collision checks.
func (w *World) profileViews() map[string]Profile {
	out := make(map[string]Profile, len(w.profiles)+len(w.players))
	for id, p := range w.profiles {
		out[id] = p
	}
	for id, p := range w.offline {
		out[id] = p.profile
	}
	for id, p := range w.players {
		out[id] = p.profile
	}
	return out
}
func (w *World) homeSafeReason(p *player) string {
	h := p.profile.Home
	if h == nil {
		return "Build a tent or cabin in a marked clearing first."
	}
	if distance(p.x, p.z, h.X, h.Z) > 8 {
		return "Visit your home to create its safe zone."
	}
	if !w.safeSpace(h.X, h.Z, 8) {
		return "An obstacle or path is within 8 metres of your home."
	}
	for _, other := range w.homes() {
		if other.Owner != p.profile.ID && distance(h.X, h.Z, other.X, other.Z) < 16 {
			return "Another home is too close."
		}
	}
	for id, other := range w.players {
		if id != p.profile.ID && distance(h.X, h.Z, other.x, other.z) < 8.4 {
			return "Another player is inside the proposed safe zone."
		}
	}
	return ""
}
func (w *World) buildHome(p *player, kind string) error {
	if !validChoice(kind, "tent", "cabin") {
		return errors.New("Choose a tent or a cabin.")
	}
	if p.profile.Home != nil {
		return errors.New("You already own a home.")
	}
	x, z := p.x, p.z+3.5
	if buildZone(x, z) == "" {
		return errors.New("Build inside a marked forest or desert building clearing.")
	}
	radius := homeRadius(kind)
	if !w.safeSpace(x, z, radius) {
		return errors.New("Keep the whole shelter clear of obstacles, paths and the edge of the building clearing.")
	}
	for _, h := range w.homes() {
		if distance(x, z, h.X, h.Z) < radius+homeRadius(h.Kind) || (h.Safe && distance(x, z, h.X, h.Z) < 8.4+radius) {
			return errors.New("Another home occupies this space.")
		}
	}
	for id, other := range w.players {
		if id != p.profile.ID && distance(x, z, other.x, other.z) < radius+.42 {
			return errors.New("Another player is standing where this shelter would be built.")
		}
	}
	cost := map[string]int{"wood": 6, "stick": 8, "stone": 4}
	if kind == "cabin" {
		cost = map[string]int{"wood": 18, "stick": 8, "stone": 12}
	}
	if !afford(p, cost, 1) {
		return fmt.Errorf("A %s needs %d wood, %d sticks and %d stones.", kind, cost["wood"], cost["stick"], cost["stone"])
	}
	spend(p, cost, 1)
	hp := homeHealth(kind)
	p.profile.Home = &Home{Owner: p.profile.ID, CallName: p.profile.CallName, Username: p.profile.Username, Kind: kind, X: x, Z: z, Health: hp, MaxHealth: hp}
	awardHats(&p.profile)
	w.event(p.profile.CallName+" built a "+kind+". Clear 8 metres around it to create a safe zone.", "build", x, z)
	return nil
}
func (w *World) createSafeZone(p *player) error {
	if p.profile.Home == nil {
		return errors.New("Build a home first.")
	}
	h := p.profile.Home
	if distance(p.x, p.z, h.X, h.Z) > 8 {
		return errors.New("Visit your home to create its safe zone.")
	}
	if reason := w.homeSafeReason(p); reason != "" {
		return errors.New(reason)
	}
	h.Safe = true
	return nil
}
func (w *World) homeBlocked(p *player, x, z float64) bool {
	return w.eachHome(func(h Home) bool {
		dx, dz := x-h.X, z-h.Z
		if h.Safe && h.Owner != p.profile.ID && distance(x, z, h.X, h.Z) < 8.4 {
			return true
		}
		if h.Kind == "tent" {
			if dz > -.5 && dz < 2.1 && dx > -2.3 && dx < 2.3 {
				return true
			}
			if dz >= -2.2 && dz <= -.5 && ((dx >= -2.3 && dx <= -1.25) || (dx >= 1.25 && dx <= 2.3)) {
				return true
			}
		} else {
			if dz > 1.65 && dz < 2.4 && dx > -2.8 && dx < 2.8 {
				return true
			}
			if dz > -2.4 && dz < 2.4 && ((dx > 1.95 && dx < 2.8) || (dx < -1.95 && dx > -2.8)) {
				return true
			}
			if dz > -2.4 && dz < -1.65 && ((dx < -1 && dx > -2.8) || (dx > 1 && dx < 2.8)) {
				return true
			}
		}
		return false
	})
}
func (w *World) insideSafe(x, z float64) bool {
	return w.eachHome(func(h Home) bool { return h.Safe && distance(x, z, h.X, h.Z) < 8 })
}
func (w *World) damageHome(owner string, damage float64) {
	update := func(p *Profile) {
		if p.Home == nil || p.Home.Safe {
			return
		}
		p.Home.Health -= damage
		if p.Home.Health <= 0 {
			w.event(p.CallName+"'s unprotected "+p.Home.Kind+" was destroyed.", "defeat", p.Home.X, p.Home.Z)
			p.Home = nil
		}
	}
	if p := w.players[owner]; p != nil {
		update(&p.profile)
		return
	}
	if p := w.offline[owner]; p != nil {
		update(&p.profile)
		w.profiles[owner] = cloneProfile(p.profile)
		return
	}
	p, ok := w.profiles[owner]
	if ok {
		update(&p)
		w.profiles[owner] = p
	}
}
