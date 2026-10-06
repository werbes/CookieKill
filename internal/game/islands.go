package game

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

type Island struct {
	Owner    string          `json:"owner"`
	Visitors bool            `json:"visitors"`
	Builders map[string]bool `json:"builders"`
	Chest    map[string]int  `json:"chest"`
	Objects  []BuildObject   `json:"objects"`
}

type BuildObject struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Z        float64 `json:"z"`
	Rotation float64 `json:"rotation"`
	Open     bool    `json:"open"`
	Lit      bool    `json:"lit"`
	Planted  string  `json:"planted,omitempty"`
	ReadyAt  float64 `json:"readyAt,omitempty"`
	Adopted  string  `json:"adopted,omitempty"`
}

type BuildRecipe struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Category string         `json:"category"`
	Cost     map[string]int `json:"cost"`
	Width    float64        `json:"width"`
	Depth    float64        `json:"depth"`
	Height   float64        `json:"height"`
}

var buildRecipes = []BuildRecipe{
	{"wall", "Wall", "building", map[string]int{"wood": 4, "stone": 2}, 4, .3, 3},
	{"floor", "Floor", "building", map[string]int{"wood": 4}, 4, 4, .2},
	{"roof", "Roof", "building", map[string]int{"wood": 5, "stick": 2}, 4, 4, .5},
	{"stairs", "Stairs", "building", map[string]int{"wood": 6, "stone": 2}, 2, 4, 3},
	{"window", "Window wall", "building", map[string]int{"wood": 3, "stone": 2}, 4, .3, 3},
	{"door", "Door", "building", map[string]int{"wood": 3, "stick": 2}, 1.4, .25, 2.6},
	{"table", "Table", "decor", map[string]int{"wood": 4}, 2.4, 1.4, 1},
	{"chair", "Chair", "decor", map[string]int{"wood": 2, "stick": 2}, 1, 1, 1.3},
	{"bed", "Bed", "decor", map[string]int{"wood": 5, "stick": 3}, 1.8, 2.8, .8},
	{"plant", "Potted plant", "decor", map[string]int{"stone": 2, "berry": 1}, .7, .7, 1.1},
	{"lantern", "Lantern", "decor", map[string]int{"stone": 2, "stick": 2}, .5, .5, .8},
	{"chest", "Shared materials chest", "decor", map[string]int{"wood": 5, "stone": 1}, 1.7, 1, 1},
	{"mixer", "Mixer", "appliances", map[string]int{"stone": 8, "wood": 4}, 1.5, 1.2, 1.6},
	{"oven", "Oven", "appliances", map[string]int{"stone": 12, "wood": 4}, 2, 1.7, 2},
	{"garden_bed", "Nut and berry garden", "appliances", map[string]int{"wood": 4, "stone": 3}, 3, 2, .6},
	{"pond", "Adoption pond", "appliances", map[string]int{"stone": 14, "wood": 4}, 5, 4, .3},
	{"fire", "Campfire", "appliances", map[string]int{"wood": 2, "stick": 2, "stone": 3}, 1.4, 1.4, .7},
}

func buildRecipeFor(id string) (BuildRecipe, bool) {
	for _, r := range buildRecipes {
		if r.ID == id {
			return r, true
		}
	}
	return BuildRecipe{}, false
}

func cloneIsland(h *Island) *Island {
	if h == nil {
		return nil
	}
	cp := *h
	cp.Builders = cloneMap(h.Builders)
	cp.Chest = cloneMap(h.Chest)
	cp.Objects = append([]BuildObject{}, h.Objects...)
	return &cp
}

func cloneIslandProfile(p *Profile) {
	p.Friends = cloneMap(p.Friends)
	p.FriendRequests = cloneMap(p.FriendRequests)
	p.HomeIsland = cloneIsland(p.HomeIsland)
}

func normalizeIslandProfile(p *Profile) {
	if p.Friends == nil {
		p.Friends = map[string]bool{}
	}
	if p.FriendRequests == nil {
		p.FriendRequests = map[string]bool{}
	}
	delete(p.Friends, p.ID)
	delete(p.FriendRequests, p.ID)
	// Main-island shelters migrate to land ownership; their resources are returned
	// through recovery so a full bag cannot erase a player's previous investment.
	if p.Home != nil {
		if p.HomeIsland == nil {
			p.HomeIsland = &Island{Owner: p.ID}
		}
		cost := map[string]int{"wood": 6, "stick": 8, "stone": 4}
		if p.Home.Kind == "cabin" {
			cost = map[string]int{"wood": 18, "stick": 8, "stone": 12}
		}
		for item, n := range cost {
			p.Recovery[item] += n
		}
		p.Home = nil
	}
	if h := p.HomeIsland; h != nil {
		h.Owner = p.ID
		if h.Builders == nil {
			h.Builders = map[string]bool{}
		}
		if h.Chest == nil {
			h.Chest = map[string]int{}
		}
		for item, n := range h.Chest {
			if !buildingMaterial(item) || n <= 0 {
				delete(h.Chest, item)
			} else {
				h.Chest[item] = min(n, inventoryLimit)
			}
		}
		kept := []BuildObject{}
		seen := map[string]bool{}
		for _, o := range h.Objects {
			_, ok := buildRecipeFor(o.Kind)
			if !ok || o.ID == "" || seen[o.ID] || !validBuildPosition(o.X, o.Y, o.Z, o.Rotation) {
				continue
			}
			o.Rotation = normalizeRotation(o.Rotation)
			seen[o.ID] = true
			kept = append(kept, o)
			if len(kept) >= 500 {
				break
			}
		}
		h.Objects = kept
	}
}

func normalizeRotation(degrees float64) float64 {
	v := math.Mod(math.Round(degrees/45)*45, 360)
	if v < 0 {
		v += 360
	}
	return v
}
func validBuildPosition(x, y, z, r float64) bool {
	return finite(x) && finite(y) && finite(z) && finite(r) && math.Hypot(x, z) <= 32 && y >= 0 && y <= 12
}
func buildingMaterial(item string) bool {
	return validChoice(item, "wood", "stick", "stone", "berry", "nut", "cactus", "shell", "pearl", "sea_salt")
}

func (w *World) currentIsland(p *player) (*Island, bool) {
	owner, ok := w.profileByID(p.island)
	if !ok || owner.HomeIsland == nil {
		return nil, false
	}
	return owner.HomeIsland, true
}
func (w *World) canBuildIsland(p *player, h *Island) bool {
	return h.Owner == p.profile.ID || h.Builders[p.profile.ID] && p.profile.Friends[h.Owner]
}
func (w *World) saveIsland(h *Island) {
	owner, ok := w.profileByID(h.Owner)
	if ok {
		owner.HomeIsland = h
		w.storeProfile(owner)
	}
}
func (w *World) setIsland(p *player, id string) {
	p.island = id
	p.posture = ""
	p.ridingCamel = false
	p.y = 0
	p.input = Input{}
	if id == "" {
		// Arrive outside the rock arch, with room to see and enter the cave.
		p.x, p.z = 120, -93
	} else {
		p.x, p.z = 0, 20
	}
	// A teleport cannot leave an old projectile behind that hits a new instance.
	shots := w.projectiles[:0]
	for _, shot := range w.projectiles {
		if shot.Owner != p.profile.ID {
			shots = append(shots, shot)
		}
	}
	w.projectiles = shots
}

func (w *World) islandAction(p *player, a Action) error {
	switch a.Action {
	case "eat_lollipop":
		if p.island != "" {
			return errors.New("Visit the clay cave in Sunbaked Sands.")
		}
		x := 111.0
		if a.Item == "blue" {
			x = 129
		} else if a.Item != "pink" {
			return errors.New("Choose the pink or blue lollipop.")
		}
		if distance(p.x, p.z, x, -124) > 4 {
			return errors.New("Walk to the lollipop at the end of the clay cave.")
		}
		if groundY(p) > -2 {
			return errors.New("Enter the underground clay cave to reach this lollipop.")
		}
		if a.Item == "pink" {
			p.homeOffer = true
			w.eventActor("The pink lollipop reveals a home island for 200 coins.", "notification", p.x, p.z, p.profile.ID)
		} else {
			p.friendsTeleport = true
		}
		return nil
	case "buy_home":
		if p.profile.HomeIsland != nil {
			return errors.New("You already own a home island.")
		}
		if !p.homeOffer {
			return errors.New("Taste the pink lollipop in the desert cave first.")
		}
		if p.profile.Coins < 200 {
			return errors.New("Your first home island costs 200 coins.")
		}
		p.profile.Coins -= 200
		p.profile.HomeIsland = &Island{Owner: p.profile.ID, Builders: map[string]bool{}, Chest: map[string]int{}, Objects: []BuildObject{{ID: "welcome_chest", Kind: "chest", X: 4, Z: 18}}}
		p.profile.Hats["builder"] = true
		w.eventActor("Your home island is ready. Teleport whenever you are ready!", "notification", p.x, p.z, p.profile.ID)
		return nil
	case "teleport_home":
		if p.profile.HomeIsland == nil {
			return errors.New("Buy your first island from the pink lollipop in the desert cave.")
		}
		w.setIsland(p, p.profile.ID)
		return nil
	case "teleport_main":
		if p.island == "" {
			return errors.New("You are already on the main island.")
		}
		w.setIsland(p, "")
		return nil
	case "visit_island":
		if !p.friendsTeleport {
			return errors.New("Taste the blue lollipop in the desert cave to unlock island visits.")
		}
		if !p.profile.Friends[a.Target] {
			return errors.New("You can visit islands belonging to your friends.")
		}
		other, ok := w.profileByID(a.Target)
		if !ok || other.HomeIsland == nil {
			return errors.New("This friend does not own an island.")
		}
		if !other.HomeIsland.Visitors {
			return errors.New("This island is closed to visitors.")
		}
		w.setIsland(p, a.Target)
		return nil
	case "island_visitors", "island_builder":
		h := p.profile.HomeIsland
		if h == nil {
			return errors.New("Buy a home island first.")
		}
		if a.Action == "island_visitors" {
			h.Visitors = a.Enabled
			if !a.Enabled {
				for _, visitor := range w.players {
					if visitor.island == p.profile.ID && visitor.profile.ID != p.profile.ID {
						w.setIsland(visitor, "")
					}
				}
				for _, visitor := range w.offline {
					if visitor.island == p.profile.ID && visitor.profile.ID != p.profile.ID {
						w.setIsland(visitor, "")
					}
				}
			}
		} else {
			if !p.profile.Friends[a.Target] {
				return errors.New("Choose a friend before granting building permission.")
			}
			if a.Enabled {
				h.Builders[a.Target] = true
			} else {
				delete(h.Builders, a.Target)
			}
		}
		return nil
	}
	h, ok := w.currentIsland(p)
	if !ok {
		return errors.New("Build and use furnishings on a home island.")
	}
	if a.Action == "chest_deposit" || a.Action == "chest_withdraw" {
		return w.chestAction(p, h, a)
	}
	if a.Action == "use_build" {
		return w.useBuild(p, h, a)
	}
	if !w.canBuildIsland(p, h) {
		return errors.New("The owner must specifically allow you to build on this island.")
	}
	h = cloneIsland(h)
	switch a.Action {
	case "build":
		r, ok := buildRecipeFor(a.Item)
		if !ok {
			return errors.New("Choose something from the build menu.")
		}
		if len(h.Objects) >= 500 {
			return errors.New("This island has reached its 500-object limit.")
		}
		if err := w.validatePlacement(p, h, a.Item, "", a); err != nil {
			return err
		}
		if !afford(p, r.Cost, 1) {
			return errors.New("Collect the listed materials, or withdraw them from the island chest.")
		}
		spend(p, r.Cost, 1)
		w.sequence++
		id := "build_" + strconv.FormatInt(w.now().UnixNano(), 36) + "_" + strconv.FormatUint(w.sequence, 36)
		h.Objects = append(h.Objects, BuildObject{ID: id, Kind: a.Item, X: a.X, Y: a.Y, Z: a.Z, Rotation: normalizeRotation(a.Rotation), Lit: a.Item == "fire"})
	case "move_build", "destroy_build":
		index := -1
		for i, o := range h.Objects {
			if o.ID == a.Target {
				index = i
				break
			}
		}
		if index < 0 {
			return errors.New("That furnishing is no longer here.")
		}
		o := h.Objects[index]
		if distance(p.x, p.z, o.X, o.Z) > 12 {
			return errors.New("Move within 12 metres of the furnishing.")
		}
		if a.Action == "move_build" {
			if err := w.validatePlacement(p, h, o.Kind, o.ID, a); err != nil {
				return err
			}
			o.X, o.Y, o.Z, o.Rotation = a.X, a.Y, a.Z, normalizeRotation(a.Rotation)
			h.Objects[index] = o
		} else {
			r, _ := buildRecipeFor(o.Kind)
			for item, n := range r.Cost {
				if n/2 > 0 && !gain(p, item, n/2) {
					return errors.New("Make space in your bag for the half-material refund.")
				}
			}
			h.Objects = append(h.Objects[:index], h.Objects[index+1:]...)
		}
	default:
		return errors.New("Unknown island action.")
	}
	w.saveIsland(h)
	return nil
}

func (w *World) validatePlacement(p *player, h *Island, kind, skip string, a Action) error {
	if !validBuildPosition(a.X, a.Y, a.Z, a.Rotation) {
		return errors.New("Build within the island's central 32 metres and below 12 metres high.")
	}
	if distance(p.x, p.z, a.X, a.Z) > 12 {
		return errors.New("Build within 12 metres of where you are standing.")
	}
	r, _ := buildRecipeFor(kind)
	if math.Hypot(a.X, a.Z)+math.Hypot(r.Width, r.Depth)/2 > 34 {
		return errors.New("Keep the whole object on your island.")
	}
	if r.Category == "appliances" {
		for _, o := range h.Objects {
			other, _ := buildRecipeFor(o.Kind)
			if o.ID != skip && other.Category == "appliances" && rectanglesOverlap(a.X, a.Z, r.Width, r.Depth, normalizeRotation(a.Rotation), o.X, o.Z, other.Width, other.Depth, o.Rotation) {
				return errors.New("Appliances need their own space and cannot be stacked on one another.")
			}
		}
	}
	for _, other := range w.players {
		if other.island != p.island {
			continue
		}
		o := BuildObject{X: a.X, Y: a.Y, Z: a.Z, Kind: kind, Rotation: normalizeRotation(a.Rotation)}
		if objectBlocks(o, r, other.x, other.y, other.z, .42) {
			return errors.New("A player is standing in that space.")
		}
	}
	return nil
}

// Separating axes for two rotated footprint rectangles. Appliance exclusions
// intentionally ignore height: an oven may never be stacked above a mixer.
func rectanglesOverlap(ax, az, aw, ad, ar, bx, bz, bw, bd, br float64) bool {
	aa, ba := ar*math.Pi/180, br*math.Pi/180
	axes := [][2]float64{{math.Cos(aa), -math.Sin(aa)}, {math.Sin(aa), math.Cos(aa)}, {math.Cos(ba), -math.Sin(ba)}, {math.Sin(ba), math.Cos(ba)}}
	for _, v := range axes {
		d := math.Abs((bx-ax)*v[0] + (bz-az)*v[1])
		ra := aw/2*math.Abs(v[0]*math.Cos(aa)-v[1]*math.Sin(aa)) + ad/2*math.Abs(v[0]*math.Sin(aa)+v[1]*math.Cos(aa))
		rb := bw/2*math.Abs(v[0]*math.Cos(ba)-v[1]*math.Sin(ba)) + bd/2*math.Abs(v[0]*math.Sin(ba)+v[1]*math.Cos(ba))
		if d >= ra+rb {
			return false
		}
	}
	return true
}

func objectLocal(o BuildObject, x, z float64) (float64, float64) {
	a := o.Rotation * math.Pi / 180
	dx, dz := x-o.X, z-o.Z
	return dx*math.Cos(a) - dz*math.Sin(a), dx*math.Sin(a) + dz*math.Cos(a)
}
func objectBlocks(o BuildObject, r BuildRecipe, x, y, z, radius float64) bool {
	if o.Kind == "door" && o.Open {
		return false
	}
	if o.Kind == "stairs" {
		lx, lz := objectLocal(o, x, z)
		if math.Abs(lx) > r.Width/2+radius || math.Abs(lz) > r.Depth/2+radius {
			return false
		}
		top := o.Y + max(0, min(1, lz/r.Depth+.5))*r.Height
		return top > y+.4 && y+1.7 > o.Y
	}
	if y >= o.Y+r.Height-.05 || y+1.7 <= o.Y {
		return false
	}
	// A ground floor is a walkable surface, not an invisible wall.
	if o.Kind == "floor" && o.Y <= y+.45 {
		return false
	}
	lx, lz := objectLocal(o, x, z)
	return circleRect(lx, lz, radius, 0, 0, r.Width, r.Depth)
}

func (w *World) islandBlocked(p *player, x, z float64) bool {
	if math.Hypot(x, z) > 40 {
		return true
	}
	h, ok := w.currentIsland(p)
	if !ok {
		return true
	}
	for _, o := range h.Objects {
		r, _ := buildRecipeFor(o.Kind)
		if objectBlocks(o, r, x, p.y, z, .42) {
			return true
		}
	}
	return false
}

func (w *World) islandGround(p *player) float64 {
	h, ok := w.currentIsland(p)
	if !ok {
		return 0
	}
	height := 0.0
	for _, o := range h.Objects {
		r, _ := buildRecipeFor(o.Kind)
		lx, lz := objectLocal(o, p.x, p.z)
		if math.Abs(lx) > r.Width/2 || math.Abs(lz) > r.Depth/2 {
			continue
		}
		top := o.Y + r.Height
		if o.Kind == "stairs" {
			top = o.Y + (lz/r.Depth+.5)*r.Height
			if top <= p.y+.4 {
				height = max(height, top)
			}
		} else if top <= p.y+.4 {
			height = max(height, top)
		}
	}
	return height
}

func (w *World) chestAction(p *player, h *Island, a Action) error {
	near := false
	for _, o := range h.Objects {
		if o.Kind == "chest" && distance(p.x, p.z, o.X, o.Z) <= 5 {
			near = true
			break
		}
	}
	if !near {
		return errors.New("Stand next to an island materials chest.")
	}
	if !buildingMaterial(a.Item) {
		return errors.New("The island chest stores building and garden materials.")
	}
	if a.Action == "chest_withdraw" && !w.canBuildIsland(p, h) {
		return errors.New("Only the owner and specifically permitted builders may withdraw materials.")
	}
	h = cloneIsland(h)
	if a.Action == "chest_deposit" {
		if h.Chest[a.Item] > inventoryLimit-a.Amount {
			return errors.New("The chest cannot hold more of that material.")
		}
		if !take(p, a.Item, a.Amount) {
			return errors.New("You do not have those materials.")
		}
		h.Chest[a.Item] += a.Amount
	} else {
		if h.Chest[a.Item] < a.Amount {
			return errors.New("The chest does not contain that many materials.")
		}
		if !gain(p, a.Item, a.Amount) {
			return errors.New("Make room in your bag first.")
		}
		h.Chest[a.Item] -= a.Amount
		if h.Chest[a.Item] == 0 {
			delete(h.Chest, a.Item)
		}
	}
	w.saveIsland(h)
	return nil
}

func (w *World) nearIslandAppliance(p *player, kind string) bool {
	h, ok := w.currentIsland(p)
	if !ok {
		return false
	}
	for _, o := range h.Objects {
		if o.Kind == kind && distance(p.x, p.z, o.X, o.Z) <= 5 && (kind != "fire" || o.Lit) {
			return true
		}
	}
	return false
}

func (w *World) useBuild(p *player, h *Island, a Action) error {
	h = cloneIsland(h)
	for i := range h.Objects {
		o := &h.Objects[i]
		if o.ID != a.Target {
			continue
		}
		if distance(p.x, p.z, o.X, o.Z) > 5 {
			return errors.New("Move within 5 metres to use this furnishing.")
		}
		switch o.Kind {
		case "chair", "bed":
			if p.posture != "" {
				p.posture = ""
			} else {
				p.posture = "sitting"
				if o.Kind == "bed" {
					p.posture = "lying"
				}
				r, _ := buildRecipeFor(o.Kind)
				p.x, p.y, p.z = o.X, o.Y+r.Height, o.Z
				p.yaw = o.Rotation * math.Pi / 180
				p.input = Input{}
				p.health = min(100, p.health+5)
			}
		case "door":
			if o.Open {
				closed := *o
				closed.Open = false
				r, _ := buildRecipeFor(o.Kind)
				for _, occupant := range w.players {
					if occupant.island == p.island && objectBlocks(closed, r, occupant.x, occupant.y, occupant.z, .42) {
						return errors.New("Someone is standing in the doorway. Wait until they pass before closing it.")
					}
				}
			}
			o.Open = !o.Open
		case "fire":
			if !w.canBuildIsland(p, h) {
				return errors.New("Only builders can light or put out the island's fire.")
			}
			o.Lit = !o.Lit
		case "mixer":
			if p.cooldowns["dough"] > w.time {
				return errors.New("The dough mixer is still working.")
			}
			if !gain(p, "dough", 2) {
				return errors.New("Make room for fresh dough.")
			}
			p.cooldowns["dough"] = w.time + 8
		case "oven":
			w.eventActor("Open your cookbook to bake cookies and cakes in this oven.", "notification", p.x, p.z, p.profile.ID)
		case "garden_bed":
			if !w.canBuildIsland(p, h) {
				return errors.New("Only permitted builders can tend this garden.")
			}
			if o.Planted == "" {
				if !validChoice(a.Item, "berry", "nut") {
					return errors.New("Choose one berry or nut to plant.")
				}
				if !take(p, a.Item, 1) {
					return errors.New("You need one berry or nut to plant.")
				}
				o.Planted = a.Item
				o.ReadyAt = float64(w.now().Unix() + 120)
			} else {
				if float64(w.now().Unix()) < o.ReadyAt {
					return errors.New("Your garden is still growing (2 minutes).")
				}
				if !gain(p, o.Planted, 4) {
					return errors.New("Make room for the harvest.")
				}
				o.Planted = ""
				o.ReadyAt = 0
			}
		case "pond":
			if o.Adopted != "" {
				w.eventActor("Your rescued "+o.Adopted+" is safe at home.", "notification", p.x, p.z, p.profile.ID)
			} else {
				w.eventActor("Find a wounded fish or turtle in the ocean and choose Adopt to bring it to this pond.", "notification", p.x, p.z, p.profile.ID)
			}
		case "chest":
			return nil
		default:
			return errors.New("This decoration can be moved or rebuilt with the build menu.")
		}
		w.saveIsland(h)
		return nil
	}
	return errors.New("That furnishing is no longer here.")
}

func (w *World) adoptAnimal(p *player, target string) error {
	if p.island != "" || p.profile.HomeIsland == nil {
		return errors.New("Build a pond on your home island, then find a wounded ocean fish or turtle.")
	}
	pond := -1
	for i, o := range p.profile.HomeIsland.Objects {
		if o.Kind == "pond" && o.Adopted == "" {
			pond = i
			break
		}
	}
	if pond < 0 {
		return errors.New("You need an empty pond on your home island.")
	}
	for _, a := range w.animals {
		if a.ID != target {
			continue
		}
		if a.Health <= 0 || a.Egg || a.Need != "wounded" || !validChoice(a.Species, "fish", "turtle") {
			return errors.New("Only wounded fish and turtles can be adopted.")
		}
		if distance(p.x, p.z, a.X, a.Z) > 5 {
			return errors.New("Swim within 5 metres to adopt this animal.")
		}
		p.profile.HomeIsland.Objects[pond].Adopted = a.Species
		a.Health = 0
		a.respawnAt = w.time + 120
		p.profile.Rescues++
		awardHats(&p.profile)
		w.eventActor("Your rescued "+a.Species+" is safe in your home pond.", "notification", p.x, p.z, p.profile.ID)
		return nil
	}
	return errors.New("That animal is no longer here.")
}

func (w *World) decorateIslandSnapshot(id string, out *Snapshot) {
	p := w.players[id]
	if p == nil {
		return
	}
	out.Me.Island = p.island
	out.Me.Posture = p.posture
	out.Me.RidingCamel = p.ridingCamel
	out.Me.HomeOffer = p.homeOffer
	out.Me.FriendsTeleport = p.friendsTeleport
	out.Me.CanBuildHome = false
	out.Me.HomeSafeReason = "Build on your home island using hotbar slot 6."
	out.Homes = []Home{}
	out.Friends = w.friendViews(p)
	out.BuildCatalog = make([]BuildRecipe, len(buildRecipes))
	for i, r := range buildRecipes {
		r.Cost = cloneMap(r.Cost)
		out.BuildCatalog[i] = r
	}
	players := out.Players[:0]
	for _, v := range out.Players {
		other := w.players[v.ID]
		if other == nil || other.island != p.island {
			continue
		}
		v.Island = other.island
		v.Posture = other.posture
		v.RidingCamel = other.ridingCamel
		if other.profile.PublicFriendCode {
			v.FriendCode = other.profile.FriendCode
		}
		if p.island != "" {
			v.Area = "home"
		}
		players = append(players, v)
	}
	out.Players = players
	nodes := out.Nodes[:0]
	for _, n := range out.Nodes {
		if n.Island == p.island {
			nodes = append(nodes, n)
		}
	}
	out.Nodes = nodes
	if p.island != "" {
		out.Me.Area = "home"
		out.Me.Zone = "home island"
		out.Animals = []Animal{}
		out.Projectiles = []Projectile{}
		if h, ok := w.currentIsland(p); ok {
			out.Island = cloneIsland(h)
			if !w.canBuildIsland(p, h) {
				out.Island.Chest = map[string]int{}
				out.Island.Builders = map[string]bool{}
			}
		}
	}
	events := out.Events[:0]
	for _, e := range out.Events {
		if e.Kind == "notification" && e.Actor != id {
			continue
		}
		if p.island != "" && e.Actor != id {
			continue
		}
		events = append(events, e)
	}
	out.Events = events
}

func (w *World) camelAction(p *player, a Action) error {
	if a.Action == "camel_ride" {
		if !p.profile.CamelOwned && p.profile.CamelRentalUntil <= w.now().Unix() {
			return errors.New("Rent or buy a camel from Abu Fanous first.")
		}
		if inWater(p) {
			return errors.New("Camels need dry land.")
		}
		p.ridingCamel = !p.ridingCamel
		return nil
	}
	if !w.near(p, "abu_fanous", 7) {
		return errors.New("Visit Abu Fanous on the far eastern edge of Sunbaked Sands.")
	}
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
	case "camel_rent":
		if p.profile.CamelOwned {
			return errors.New("You already own a camel.")
		}
		if p.profile.CamelRentalUntil > w.now().Unix() {
			return errors.New("Your current camel rental is still active.")
		}
		if p.profile.Coins < 50 {
			return errors.New("A 20-minute camel rental costs 50 coins.")
		}
		p.profile.Coins -= 50
		p.profile.CamelRentalUntil = w.now().Unix() + 1200
		p.ridingCamel = true
		return nil
	case "camel_buy":
		if p.profile.CamelOwned {
			return errors.New("You already own a camel.")
		}
		cost := 350
		if p.profile.CamelDiscount {
			cost = 175
		}
		if p.profile.Coins < cost {
			return fmt.Errorf("A permanent camel costs %d coins.", cost)
		}
		p.profile.Coins -= cost
		p.profile.CamelOwned = true
		p.profile.CamelRentalUntil = 0
		p.ridingCamel = true
		return nil
	}
	return errors.New("Choose a camel rental, purchase, or gift.")
}
