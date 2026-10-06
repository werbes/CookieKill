package game

import (
	"math"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
)

var recipes = []Recipe{
	{"sugar", "Sugar cookie", "A simple cookie: damage when thrown, health when eaten.", 18, 18, map[string]int{"dough": 1}, true},
	{"berry_cookie", "Berry burst", "Slows opponents for 5 seconds.", 20, 20, map[string]int{"dough": 1, "berry": 2}, true},
	{"nut_cookie", "Nut crunch", "Weakens opponents' damage and throwing range for 6 seconds.", 25, 22, map[string]int{"dough": 1, "nut": 2}, true},
	{"cactus_cookie", "Cactus crisp", "Eat for a 12-second speed boost.", 22, 24, map[string]int{"dough": 1, "cactus": 2}, true},
	{"sun_cookie", "Sun-baked cookie", "Bake in desert heat. Eat for 15 seconds of longer throws.", 26, 25, map[string]int{"dough": 1, "cactus": 1, "berry": 1}, true},
	{"salt_cookie", "Sea-salt swirl", "Eat for 15 seconds of faster swimming.", 28, 28, map[string]int{"dough": 1, "sea_salt": 2}, true},
	{"protein_cookie", "Protein power", "Bake with powder or a protein drink. Eat for 10 seconds of superstrength: one hit defeats a player.", 30, 25, map[string]int{"dough": 1, "protein_powder": 1}, true},
	{"cake", "Celebration cake", "A bakery specialty with extra damage and healing.", 45, 45, map[string]int{"dough": 3, "berry": 2, "nut": 2}, true},
}

var itemSet = map[string]bool{
	"wood": true, "stick": true, "stone": true, "dough": true,
	"berry": true, "nut": true, "cactus": true, "sea_salt": true,
	"shell": true, "trash": true, "pearl": true, "treasure": true,
	"old_coin": true, "rare_item": true, "protein_powder": true, "protein_drink": true,
	"sugar": true, "berry_cookie": true, "nut_cookie": true, "cactus_cookie": true,
	"sun_cookie": true, "salt_cookie": true, "protein_cookie": true, "cake": true,
	"paint_teal": true, "paint_blue": true, "paint_red": true, "paint_purple": true, "paint_sand": true, "paint_black": true, "paint_white": true,
}

const inventoryLimit = 1000000

func recipeFor(id string) (Recipe, bool) {
	for _, r := range recipes {
		if r.ID == id {
			return r, true
		}
	}
	return Recipe{}, false
}

// New restores saved economy/progression and creates a renewable shared world.
func New(saved map[string]Profile) *World {
	w := &World{players: map[string]*player{}, offline: map[string]*player{}, profiles: map[string]Profile{}, events: []Event{}, now: time.Now}
	for id, profile := range saved {
		profile.ID = id
		w.profiles[id] = sanitize(profile)
	}
	w.seedMap()
	w.seedAnimals()
	w.restoreProperties()
	w.restoreSocial()
	w.rebuildPropertyColliders()
	return w
}

func kindAmount(kind string) int {
	if kind == "trash" {
		return 2
	}
	return 1
}

func sanitize(p Profile) Profile {
	p = cloneProfile(p)
	if p.Inventory == nil {
		p.Inventory = map[string]int{}
	}
	for key, count := range p.Inventory {
		if !itemSet[key] || count < 0 {
			delete(p.Inventory, key)
		} else {
			p.Inventory[key] = min(count, inventoryLimit)
		}
	}
	p.Coins = max(0, min(p.Coins, inventoryLimit))
	if p.Levels == nil {
		p.Levels = map[string]int{}
	}
	for _, key := range []string{"legs", "stamina", "arms", "swim"} {
		p.Levels[key] = max(0, min(10, p.Levels[key]))
	}
	for key := range p.Levels {
		if key != "legs" && key != "stamina" && key != "arms" && key != "swim" {
			delete(p.Levels, key)
		}
	}
	// Every attainable upgrade is preserved; the wallet cap only bounds corrupt
	// saves whose next purchase could never have been afforded.
	p.BakeryLevel = max(0, min((inventoryLimit-50)/25+1, p.BakeryLevel))
	if !p.Bakery {
		p.BakeryLevel = 0
	}
	if p.Reputation == nil {
		p.Reputation = map[string]int{}
	}
	for key, value := range p.Reputation {
		p.Reputation[key] = max(-10, min(10, value))
	}
	normalizeInventory(&p)
	normalizeIdentity(&p)
	normalizeIslandProfile(&p)
	p.Kills = max(0, p.Kills)
	p.Deaths = max(0, p.Deaths)
	return p
}

func cloneMap[K comparable, V any](src map[K]V) map[K]V {
	dst := make(map[K]V, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
func cloneProfile(p Profile) Profile {
	p.Inventory = cloneMap(p.Inventory)
	p.Levels = cloneMap(p.Levels)
	p.Reputation = cloneMap(p.Reputation)
	p.Protected = append([]string{}, p.Protected...)
	p.Recovery = cloneMap(p.Recovery)
	p.Discovered = cloneMap(p.Discovered)
	p.Hats = cloneMap(p.Hats)
	p.Equipment = cloneMap(p.Equipment)
	p.Paint = cloneMap(p.Paint)
	if p.Home != nil {
		h := *p.Home
		p.Home = &h
	}
	cloneIslandProfile(&p)
	return p
}

func (w *World) Join(id, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Cookie explorer"
	}
	if len([]rune(name)) > 24 {
		name = string([]rune(name)[:24])
	}
	if _, ok := w.players[id]; ok {
		// Account login cannot bypass call-name cooldown.
		return
	}
	if p, ok := w.offline[id]; ok {
		// Account login cannot bypass call-name cooldown.
		p.input = Input{}
		p.cooldowns["last_input"] = w.time
		w.players[id] = p
		delete(w.offline, id)
		return
	}
	profile, exists := w.profiles[id]
	if !exists {
		profile = Profile{ID: id, Name: name, CallName: name, Inventory: map[string]int{"sugar": 10}, Selected: "sugar"}
	}
	profile = sanitize(profile)
	w.ensureFriendCode(&profile)
	if profile.CallName == "" {
		profile.CallName = name
	}
	profile.Name = profile.CallName
	profile.ID = id
	p := &player{profile: profile, health: 100, stamina: 100 + float64(profile.Levels["stamina"])*10, buffs: map[string]float64{}, cooldowns: map[string]float64{}}
	w.spawn(p)
	w.players[id] = p
	w.event(profile.CallName+" arrived in the forest.", "join", p.x, p.z)
}

func (w *World) Leave(id string) {
	if p, ok := w.players[id]; ok {
		w.profiles[id] = cloneProfile(p.profile)
		p.input = Input{}
		w.offline[id] = p
		delete(w.players, id)
	}
	shots := w.projectiles[:0]
	for _, shot := range w.projectiles {
		if shot.Owner != id {
			shots = append(shots, shot)
		}
	}
	w.projectiles = shots
}

func (w *World) spawn(p *player) {
	p.island, p.posture = "", ""
	p.y = 0
	p.x, p.z = -35, -35
	for attempt := 0; attempt < 24; attempt++ {
		x, z := -35+rand.Float64()*12-6, -35+rand.Float64()*12-6
		if !w.solidBlocked(x, z, .5) {
			p.x, p.z = x, z
			break
		}
	}
	p.health = 100
	p.stamina = 100 + float64(p.profile.Levels["stamina"])*10
	p.input = Input{}
	p.buffs = map[string]float64{"spawn_shield": w.time + 3}
}

func (w *World) Input(id string, in Input) {
	p, ok := w.players[id]
	if !ok {
		return
	}
	if !finite(in.X) || !finite(in.Z) || !finite(in.Yaw) || !finite(in.Pitch) {
		return
	}
	in.X = max(-1, min(1, in.X))
	in.Z = max(-1, min(1, in.Z))
	length := math.Hypot(in.X, in.Z)
	if length > 1 {
		in.X /= length
		in.Z /= length
	}
	in.Yaw = math.Remainder(in.Yaw, math.Pi*2)
	in.Pitch = max(-1.35, min(1.35, in.Pitch))
	p.input = in
	p.yaw = in.Yaw
	p.pitch = in.Pitch
	p.cooldowns["last_input"] = w.time
}

func finite(f float64) bool                        { return !math.IsNaN(f) && !math.IsInf(f, 0) }
func distance(x, z, x2, z2 float64) float64        { return math.Hypot(x-x2, z-z2) }
func (w *World) active(p *player, key string) bool { return p.buffs[key] > w.time }

func (w *World) event(message, kind string, x, z float64) {
	w.sequence++
	w.events = append(w.events, Event{ID: w.sequence, Text: message, Kind: kind, X: x, Z: z, Time: w.time})
	if len(w.events) > 64 {
		w.events = append([]Event{}, w.events[len(w.events)-64:]...)
	}
}

func (w *World) eventActor(message, kind string, x, z float64, actor string) {
	w.event(message, kind, x, z)
	w.events[len(w.events)-1].Actor = actor
}

func (w *World) Profiles() map[string]Profile {
	out := make(map[string]Profile, len(w.profiles)+len(w.players))
	for id, p := range w.profiles {
		out[id] = cloneProfile(p)
	}
	for id, p := range w.players {
		out[id] = cloneProfile(p.profile)
	}
	return out
}

func (w *World) remainingBuffs(p *player) map[string]float64 {
	out := map[string]float64{}
	for k, until := range p.buffs {
		if until > w.time {
			out[k] = until - w.time
		}
	}
	return out
}

func (w *World) Snapshot(id string) Snapshot {
	out := Snapshot{Time: w.time, Players: []PlayerView{}, Projectiles: []Projectile{}, Nodes: []Node{}, Animals: []Animal{}, Events: []Event{}, Recipes: []Recipe{}}
	layout := w.StaticLayout()
	out.Layout = &layout
	out.Homes = w.homes()
	out.ServerTime = w.now().Unix()
	profiles := w.profileViews()
	for pid, p := range w.players {
		buffs := w.remainingBuffs(p)
		if pid == id {
			out.Me = Self{Profile: cloneProfile(p.profile), X: p.x, Y: groundY(p), Z: p.z, Yaw: p.yaw, Pitch: p.pitch, Health: p.health, MaxHealth: 100, Stamina: p.stamina, MaxStamina: 100 + float64(p.profile.Levels["stamina"])*10, Area: area(p.x, p.z), Buffs: buffs}
			out.Me.Zone = zoneAt(p.x, p.z)
			out.Me.CanBuildHome = buildZone(p.x, p.z+3.5) != "" && p.profile.Home == nil
			out.Me.HomeSafeReason = w.homeSafeReason(p)
			continue
		}
		out.Players = append(out.Players, PlayerView{ID: pid, Name: p.profile.Name, X: p.x, Y: groundY(p), Z: p.z, Yaw: p.yaw, Pitch: p.pitch, Health: p.health, MaxHealth: 100, Area: area(p.x, p.z), Buffs: buffs, Username: p.profile.Username, CallName: p.profile.CallName, Avatar: p.profile.Avatar})
	}
	sort.Slice(out.Players, func(i, j int) bool { return out.Players[i].ID < out.Players[j].ID })
	for _, n := range w.nodes {
		cp := *n
		if n.Kind == "bakery_plot" {
			for owner, p := range profiles {
				if p.BakeryPlot == n.ID {
					cp.Owner = owner
					cp.CallName = p.CallName
					cp.Username = p.Username
					cp.Paint = cloneMap(p.Paint)
					cp.Equipment = cloneMap(p.Equipment)
					cp.BakeryLevel = p.BakeryLevel
					break
				}
			}
		}
		out.Nodes = append(out.Nodes, cp)
	}
	for _, shot := range w.projectiles {
		out.Projectiles = append(out.Projectiles, *shot)
	}
	for _, a := range w.animals {
		if a.Health <= 0 {
			continue
		}
		cp := *a
		cp.rewardAt = nil
		if p, ok := w.players[id]; ok {
			rep := p.profile.Reputation[a.ID]
			if rep >= 2 {
				cp.Disposition = "friendly"
			} else if rep < 0 {
				if a.Species == "shark" || a.Species == "sea_lion" {
					cp.Disposition = "hostile"
				} else {
					cp.Disposition = "wary"
				}
			}
		}
		out.Animals = append(out.Animals, cp)
	}
	for _, e := range w.events {
		if e.Kind == "craft" && e.Actor != "" && e.Actor != id {
			continue
		}
		if w.time-e.Time < 12 {
			out.Events = append(out.Events, e)
		}
	}
	for _, r := range recipes {
		r.Cost = cloneMap(r.Cost)
		r.Known = out.Me.Discovered[r.ID]
		if !r.Known {
			r.Name = "Undiscovered recipe"
			r.Description = ""
			r.Damage = 0
			r.Heal = 0
		}
		out.Recipes = append(out.Recipes, r)
	}
	w.decorateIslandSnapshot(id, &out)
	return out
}
