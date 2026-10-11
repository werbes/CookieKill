package game

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

type Island struct {
	Owner           string                    `json:"owner"`
	Visitors        bool                      `json:"visitors"`
	Builders        map[string]bool           `json:"builders"`
	Chest           map[string]int            `json:"chest"`
	Objects         []BuildObject             `json:"objects"`
	Storage         map[string]map[string]int `json:"storage,omitempty"`
	UpgradeLevel    int                       `json:"upgradeLevel"`
	BuildRadius     float64                   `json:"buildRadius"`
	WalkRadius      float64                   `json:"walkRadius"`
	NextUpgradeCost int                       `json:"nextUpgradeCost"`
	MaxUpgrades     int                       `json:"maxUpgrades"`
}

type BuildObject struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	X               float64  `json:"x"`
	Y               float64  `json:"y"`
	Z               float64  `json:"z"`
	Rotation        float64  `json:"rotation"`
	Open            bool     `json:"open"`
	Lit             bool     `json:"lit"`
	Planted         string   `json:"planted,omitempty"`
	ReadyAt         float64  `json:"readyAt,omitempty"`
	Adopted         string   `json:"adopted,omitempty"`
	PondCapacity    int      `json:"pondCapacity,omitempty"`
	PondUpgradeCost int      `json:"pondUpgradeCost,omitempty"`
	AdoptedAnimals  []string `json:"adoptedAnimals,omitempty"`
	Color           string   `json:"color,omitempty"`
	EditingBy       string   `json:"editingBy,omitempty"`
}

type BuildRecipe struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Category string         `json:"category"`
	Cost     map[string]int `json:"cost"`
	CoinCost int            `json:"coinCost,omitempty"`
	Width    float64        `json:"width"`
	Depth    float64        `json:"depth"`
	Height   float64        `json:"height"`
}

const (
	islandPortalX      = 0.0
	islandPortalZ      = 28.0
	islandPortalRadius = 3.0
)

var buildRecipes = []BuildRecipe{
	{"wall", "Wall", "building", map[string]int{"wood": 4, "stone": 2}, 0, 4, .3, 3},
	{"floor", "Floor", "building", map[string]int{"wood": 4}, 0, 4, 4, .2},
	{"roof", "Roof", "building", map[string]int{"wood": 5, "stick": 2}, 0, 4, 4, .5},
	{"stairs", "Stairs", "building", map[string]int{"wood": 6, "stone": 2}, 0, 2, 4, 3},
	{"window", "Window wall", "building", map[string]int{"wood": 3, "stone": 2}, 0, 4, .3, 3},
	{"door", "Door", "building", map[string]int{"wood": 3, "stick": 2}, 0, 1.4, .25, 2.6},
	{"table", "Table", "decor", nil, 30, 2.4, 1.4, 1},
	{"chair", "Chair", "decor", nil, 12, 1, 1, 1.3},
	{"bed", "Bed", "decor", nil, 45, 1.8, 2.8, .8},
	{"plant", "Potted plant", "decor", nil, 15, .7, .7, 1.1},
	{"lantern", "Lantern", "decor", nil, 25, .5, .5, .8},
	{"chest", "Shared materials chest", "decor", nil, 40, 1.7, 1, 1},
	{"storage_chest", "Personal storage chest", "decor", nil, 50, 1.8, 1.1, 1},
	{"tree", "Garden tree", "garden", nil, 25, 2.4, 2.4, 4},
	{"flowers", "Flower bed", "garden", nil, 15, 1.8, 1.2, .6},
	{"mixer", "Mixer", "appliances", nil, 90, 1.5, 1.2, 1.6},
	{"oven", "Oven", "appliances", nil, 120, 2, 1.7, 2},
	{"garden_bed", "Nut and berry garden", "appliances", nil, 35, 3, 2, .6},
	{"pond", "Adoption pond", "appliances", nil, 100, 5, 4, .3},
	{"camel_pen", "Fenced camel enclosure", "appliances", nil, 150, 8, 8, 1.5},
	{"fire", "Campfire", "appliances", nil, 20, 1.4, 1.4, .7},
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
	for i := range cp.Objects {
		cp.Objects[i].AdoptedAnimals = append([]string(nil), h.Objects[i].AdoptedAnimals...)
	}
	normalizeIslandUpgrade(&cp)
	cp.Storage = make(map[string]map[string]int, len(h.Storage))
	for id, contents := range h.Storage {
		cp.Storage[id] = cloneMap(contents)
	}
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
		normalizeIslandUpgrade(h)
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
			if !ok || o.ID == "" || seen[o.ID] || !validIslandBuildPosition(h, o.X, o.Y, o.Z, o.Rotation) {
				continue
			}
			o.Rotation = normalizeRotation(o.Rotation)
			o.EditingBy = ""
			normalizePond(&o)
			if !validColor(o.Color) {
				o.Color = ""
			}
			seen[o.ID] = true
			kept = append(kept, o)
			if len(kept) >= 500 {
				break
			}
		}
		h.Objects = kept
		storage := map[string]map[string]int{}
		for _, o := range kept {
			if o.Kind != "storage_chest" {
				continue
			}
			contents := map[string]int{}
			for item, count := range h.Storage[o.ID] {
				if isInventoryItem(item) && count > 0 {
					contents[item] = min(count, inventoryLimit)
				}
			}
			storage[o.ID] = contents
		}
		h.Storage = storage
		clearIslandPortal(h)
	}
}

func overlapsIslandPortal(o BuildObject) bool {
	r, ok := buildObjectRecipe(o)
	if !ok {
		return false
	}
	x, z := objectLocal(o, islandPortalX, islandPortalZ)
	if circleRect(x, z, islandPortalRadius, 0, 0, r.Width, r.Depth) {
		return true
	}
	x, z = objectLocal(o, islandUpgradeX, islandUpgradeZ)
	return circleRect(x, z, 1.5, 0, 0, r.Width, r.Depth)
}

// Existing islands gain the return platform too. Move nearby saved furnishings
// to the nearest open spot without consuming materials or changing their IDs.
func clearIslandPortal(h *Island) {
	for i, o := range h.Objects {
		if !overlapsIslandPortal(o) {
			continue
		}
		r, _ := buildObjectRecipe(o)
		best, nearest := o, math.Inf(1)
		fallback, fallbackDistance := o, math.Inf(1)
		for x := -islandBuildRadius(h); x <= islandBuildRadius(h); x++ {
			for z := -islandBuildRadius(h); z <= islandBuildRadius(h); z++ {
				d := distance(o.X, o.Z, x, z)
				if d >= nearest || math.Hypot(x, z)+math.Hypot(r.Width, r.Depth)/2 > islandBuildRadius(h)+2 || !validIslandBuildPosition(h, x, o.Y, z, o.Rotation) {
					continue
				}
				candidate := o
				candidate.X, candidate.Z = x, z
				if overlapsIslandPortal(candidate) {
					continue
				}
				clear, allowed := true, true
				for j, other := range h.Objects {
					if i == j {
						continue
					}
					or, _ := buildObjectRecipe(other)
					if rectanglesOverlap(x, z, r.Width, r.Depth, o.Rotation, other.X, other.Z, or.Width, or.Depth, other.Rotation) {
						clear = false
						if r.Category == "appliances" && or.Category == "appliances" {
							allowed = false
							break
						}
					}
				}
				if clear {
					best, nearest = candidate, d
				}
				if allowed && d < fallbackDistance {
					fallback, fallbackDistance = candidate, d
				}
			}
		}
		// A paved or decorated island may have no empty footprint. Normal
		// placement allows building/decor overlap, so use the closest such spot
		// while still keeping appliances separate and every saved object intact.
		if math.IsInf(nearest, 1) {
			best = fallback
		}
		h.Objects[i] = best
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
	return validChoice(item, "wood", "stick", "stone", "berry", "nut", "cactus", "shell", "pearl", "sea_salt") || strings.HasPrefix(item, "paint_") && validColor(strings.TrimPrefix(item, "paint_"))
}

func packedBuildItem(item string) (kind, color string, ok bool) {
	if !strings.HasPrefix(item, "build_") {
		return "", "", false
	}
	kind, color, painted := strings.Cut(strings.TrimPrefix(item, "build_"), ":")
	_, exists := buildRecipeFor(kind)
	return kind, color, exists && (!painted || validChoice(kind, "chest", "storage_chest") && validColor(color) || kind == "pond" && validChoice(color, "capacity2", "capacity3", "capacity4"))
}

func packedBuildName(o BuildObject) string {
	item := "build_" + o.Kind
	if o.Kind == "pond" && pondCapacity(o) > 1 {
		return item + ":capacity" + strconv.Itoa(pondCapacity(o))
	}
	if o.Color != "" && validChoice(o.Kind, "chest", "storage_chest") {
		item += ":" + o.Color
	}
	return item
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
	w.leaveRollerCoaster(p)
	p.bakeryBuildMode, p.editingBakeryFurnitureID = false, ""
	p.buildMode, p.editingBuildID = false, ""
	w.updateMountedCamel(p)
	p.island = id
	w.leaveJohnPorkBehind(p)
	p.inCave = false
	p.posture = ""
	p.y = 0
	if id == "" {
		// Arrive outside the rock arch, with room to see and enter the cave.
		p.x, p.z = 120, -70
	} else {
		p.x, p.z = 0, 20
	}
	resetMovement(p)
	syncMountedCamel(p)
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
	case "cancel_build":
		p.editingBuildID = ""
		return nil
	case "build_mode":
		if !a.Enabled {
			p.buildMode, p.editingBuildID = false, ""
			return nil
		}
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
		if !p.inCave || groundY(p) > -2 {
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
	case "teleport_cave":
		if p.island == "" {
			return errors.New("Use a return platform on a home island.")
		}
		if distance(p.x, p.z, islandPortalX, islandPortalZ) > islandPortalRadius || playerY(p) > 2 {
			return errors.New("Step onto the island's return platform to teleport to the clay cave.")
		}
		w.setIsland(p, "")
		p.x, p.z = 120, -117
		p.inCave = true
		syncMountedCamel(p)
		w.eventActor("The return platform brought you to the Sunbaked clay cave.", "notification", p.x, p.z, p.profile.ID)
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
				// Offline visitors have no runtime player. Return their parked
				// camels through the saved profile so closing an island cannot
				// strand an animal behind its visitor restriction.
				for id, profile := range w.profiles {
					if id == p.profile.ID || w.players[id] != nil {
						continue
					}
					for i := range profile.Camels {
						c := &profile.Camels[i]
						if c.Island == p.profile.ID {
							c.X, c.Y, c.Z, c.Island, c.InCave = 120, 0, -70, "", false
						}
					}
					w.profiles[id] = profile
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
	if a.Action == "storage_deposit" || a.Action == "storage_withdraw" {
		return w.storageAction(p, h, a)
	}
	if a.Action == "use_build" {
		return w.useBuild(p, h, a)
	}
	if !w.canBuildIsland(p, h) {
		return errors.New("The owner must specifically allow you to build on this island.")
	}
	if a.Action == "build_mode" {
		p.buildMode = true
		return nil
	}
	if !p.buildMode {
		return errors.New("Enter build mode before placing or editing island objects.")
	}
	h = cloneIsland(h)
	switch a.Action {
	case "build":
		if p.editingBuildID != "" {
			return errors.New("Place or cancel the object you are editing first.")
		}
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
		color := ""
		capacity := 1
		if a.From != "" {
			kind, packedColor, valid := packedBuildItem(a.From)
			if h.Owner != p.profile.ID || !valid || kind != a.Item || !take(p, a.From, 1) {
				return errors.New("Place a packed furnishing from your own inventory on your own island.")
			}
			color = packedColor
			if kind == "pond" {
				if packedColor != "" {
					capacity, _ = strconv.Atoi(strings.TrimPrefix(packedColor, "capacity"))
				}
				color = ""
			}
		} else if r.CoinCost > 0 {
			if p.profile.Coins < r.CoinCost {
				return errors.New("You need more coins to place this furnishing.")
			}
			p.profile.Coins -= r.CoinCost
		} else if err := spendBuildMaterials(p, h, r.Cost); err != nil {
			return err
		}
		w.sequence++
		id := "build_" + strconv.FormatInt(w.now().UnixNano(), 36) + "_" + strconv.FormatUint(w.sequence, 36)
		object := BuildObject{ID: id, Kind: a.Item, X: a.X, Y: a.Y, Z: a.Z, Rotation: normalizeRotation(a.Rotation), Lit: a.Item == "fire", Color: color}
		if a.Item == "pond" {
			object.PondCapacity = capacity
			normalizePond(&object)
		}
		h.Objects = append(h.Objects, object)
	case "edit_build", "move_build", "destroy_build", "pack_build", "paint_build":
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
		if o.Kind == "camel_pen" && w.camelPenOccupied(h.Owner, o.ID) {
			return errors.New("Unhouse the camels assigned to this enclosure before editing or removing it.")
		}
		if distance(p.x, p.z, o.X, o.Z) > 12 {
			return errors.New("Move within 12 metres of the furnishing.")
		}
		if editor := w.buildEditor(h, o.ID); editor != "" && editor != p.profile.ID {
			return errors.New("Another builder is editing this object. Wait until they finish.")
		}
		if a.Action == "edit_build" {
			p.editingBuildID = o.ID
			return nil
		}
		if p.editingBuildID != o.ID {
			return errors.New("Look at this object and press E to take edit control first.")
		}
		if a.Action == "move_build" {
			if err := w.validatePlacement(p, h, o.Kind, o.ID, a); err != nil {
				return err
			}
			o.X, o.Y, o.Z, o.Rotation = a.X, a.Y, a.Z, normalizeRotation(a.Rotation)
			h.Objects[index] = o
		} else if a.Action == "paint_build" {
			if !validChoice(o.Kind, "chest", "storage_chest") || !validColor(a.Item) {
				return errors.New("Choose a chest and one of the available paint colors.")
			}
			if o.Color == a.Item {
				return nil
			}
			if err := spendBuildMaterials(p, h, map[string]int{"paint_" + a.Item: 1}); err != nil {
				return err
			}
			o.Color = a.Item
			h.Objects[index] = o
		} else {
			ownerProfile := w.productionOwnerProfile(h.Owner)
			if ownerProfile != nil && productionRemovalBlocked(ownerProfile, "home:"+o.ID) {
				return errors.New("Finish cooking and collect this appliance's output before removing it.")
			}
			if buildHasContents(h, o) {
				return errors.New("Empty this chest, harvest this garden, or rehome its animal before removing it.")
			}
			if a.Action == "pack_build" {
				if h.Owner != p.profile.ID {
					return errors.New("Only the island owner may pack objects into their inventory.")
				}
				if !gain(p, packedBuildName(o), 1) {
					return errors.New("Make room in your bag for this furnishing.")
				}
			} else {
				r, _ := buildRecipeFor(o.Kind)
				if err := refundBuildMaterials(p, h, r.Cost); err != nil {
					return err
				}
			}
			delete(h.Storage, o.ID)
			if ownerProfile != nil {
				delete(ownerProfile.Production, "home:"+o.ID)
			}
			h.Objects = append(h.Objects[:index], h.Objects[index+1:]...)
		}
		if a.Action != "paint_build" {
			p.editingBuildID = ""
		}
	default:
		return errors.New("Unknown island action.")
	}
	w.saveIsland(h)
	return nil
}

func spendBuildMaterials(p *player, h *Island, cost map[string]int) error {
	if h.Owner == p.profile.ID {
		if !afford(p, cost, 1) {
			return errors.New("Collect the listed materials or withdraw them from your shared chest.")
		}
		spend(p, cost, 1)
		return nil
	}
	for item, count := range cost {
		if h.Chest[item] < count {
			return errors.New("The shared materials chest needs the listed supplies before a guest can build.")
		}
	}
	for item, count := range cost {
		h.Chest[item] -= count
		if h.Chest[item] == 0 {
			delete(h.Chest, item)
		}
	}
	return nil
}

func refundBuildMaterials(p *player, h *Island, cost map[string]int) error {
	if h.Owner != p.profile.ID {
		for item, n := range cost {
			if h.Chest[item] > inventoryLimit-n/2 {
				return errors.New("Make room in the shared materials chest for the refund.")
			}
		}
		for item, n := range cost {
			if n/2 > 0 {
				h.Chest[item] += n / 2
			}
		}
		return nil
	}
	// Validate the complete multi-material refund before changing live inventory.
	draft := *p
	draft.profile = cloneProfile(p.profile)
	for item, n := range cost {
		if n/2 > 0 && !gain(&draft, item, n/2) {
			return errors.New("Make space in your bag for the half-material refund.")
		}
	}
	p.profile = draft.profile
	return nil
}

func (w *World) buildEditor(h *Island, target string) string {
	for id, editor := range w.players {
		if editor.island == h.Owner && editor.buildMode && editor.editingBuildID == target && w.canBuildIsland(editor, h) {
			return id
		}
	}
	return ""
}

func buildHasContents(h *Island, o BuildObject) bool {
	if len(h.Storage[o.ID]) > 0 || o.Planted != "" || len(pondAnimals(o)) > 0 {
		return true
	}
	if o.Kind == "chest" && len(h.Chest) > 0 {
		for _, other := range h.Objects {
			if other.Kind == "chest" && other.ID != o.ID {
				return false
			}
		}
		return true
	}
	return false
}

func (w *World) validatePlacement(p *player, h *Island, kind, skip string, a Action) error {
	if !validIslandBuildPosition(h, a.X, a.Y, a.Z, a.Rotation) {
		return errors.New("Keep your build inside this island's building radius and below 12 metres high.")
	}
	if distance(p.x, p.z, a.X, a.Z) > 12 {
		return errors.New("Build within 12 metres of where you are standing.")
	}
	candidate := BuildObject{Kind: kind, X: a.X, Y: a.Y, Z: a.Z, Rotation: normalizeRotation(a.Rotation)}
	for _, o := range h.Objects {
		if o.ID == skip {
			candidate.PondCapacity = o.PondCapacity
			break
		}
	}
	if kind == "pond" && strings.HasPrefix(a.From, "build_pond:capacity") {
		candidate.PondCapacity, _ = strconv.Atoi(strings.TrimPrefix(a.From, "build_pond:capacity"))
	}
	r, _ := buildObjectRecipe(candidate)
	if (kind == "camel_pen" || kind == "pond") && a.Y != 0 {
		return errors.New("Place ponds and camel enclosures directly on the island ground.")
	}
	if overlapsIslandPortal(candidate) {
		return errors.New("Keep the return platform and island upgrade sign clear.")
	}
	if math.Hypot(a.X, a.Z)+math.Hypot(r.Width, r.Depth)/2 > islandBuildRadius(h)+2 {
		return errors.New("Keep the whole object on your island.")
	}
	if r.Category == "appliances" {
		for _, o := range h.Objects {
			other, _ := buildObjectRecipe(o)
			if o.ID != skip && other.Category == "appliances" && rectanglesOverlap(a.X, a.Z, r.Width, r.Depth, normalizeRotation(a.Rotation), o.X, o.Z, other.Width, other.Depth, o.Rotation) {
				return errors.New("Appliances need their own space and cannot be stacked on one another.")
			}
		}
	}
	if owner, ok := w.profileByID(h.Owner); ok {
		for _, camel := range owner.Camels {
			pen, exists := camelHomePen(&owner, camel.HomePenID)
			if !exists || camel.HomePenSlot < 0 || camel.HomePenSlot >= camelPenCapacity {
				continue
			}
			slot := camelPenSlots(pen)[camel.HomePenSlot]
			if objectBlocks(candidate, r, slot.X, pen.Y, slot.Z, .65) {
				return errors.New("Keep assigned camel resting places clear, even while their camels are away.")
			}
		}
	}
	for _, other := range w.players {
		if other.island != p.island {
			continue
		}
		if objectBlocks(candidate, r, other.x, playerY(other), other.z, .42) {
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
	if o.Kind == "flowers" {
		return false
	}
	if o.Kind == "tree" {
		return y < o.Y+3 && y+1.7 > o.Y && distance(x, z, o.X, o.Z) < radius+.3
	}
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
	if o.Kind == "camel_pen" {
		return penBlocks(o, r, x, z, radius)
	}
	// A ground floor is a walkable surface, not an invisible wall.
	if o.Kind == "floor" && o.Y <= y+.45 {
		return false
	}
	lx, lz := objectLocal(o, x, z)
	return circleRect(lx, lz, radius, 0, 0, r.Width, r.Depth)
}

func (w *World) islandBlocked(p *player, x, z float64) bool {
	if isNutIsland(p.island) {
		return w.nutIslandBlocked(p, x, z)
	}
	if isCookieArena(p.island) {
		return cookieArenaBlocked(p, x, z)
	}
	h, ok := w.currentIsland(p)
	if !ok || math.Hypot(x, z) > islandWalkRadius(h) {
		return true
	}
	for _, o := range h.Objects {
		r, _ := buildObjectRecipe(o)
		if objectBlocks(o, r, x, playerY(p), z, .42) {
			return true
		}
	}
	return false
}

func (w *World) islandGround(p *player) float64 {
	if isNutIsland(p.island) {
		return 0
	}
	if p.island == CookiePracticeIsland {
		return cookiePracticeGround(p, p.x, p.z)
	}
	if p.island == CookieRingIsland {
		return 0
	}
	h, ok := w.currentIsland(p)
	if !ok {
		return 0
	}
	height := 0.0
	stepHeight := .4
	if p.jumpOffset > 0 || p.jumpVelocity > 0 {
		stepHeight = 0
	}
	for _, o := range h.Objects {
		if o.Kind == "camel_pen" {
			continue
		}
		r, _ := buildObjectRecipe(o)
		lx, lz := objectLocal(o, p.x, p.z)
		if math.Abs(lx) > r.Width/2 || math.Abs(lz) > r.Depth/2 {
			continue
		}
		top := o.Y + r.Height
		if o.Kind == "stairs" {
			top = o.Y + (lz/r.Depth+.5)*r.Height
			if top <= playerY(p)+stepHeight {
				height = max(height, top)
			}
		} else if top <= playerY(p)+stepHeight {
			height = max(height, top)
		}
	}
	return height
}

func (w *World) chestAction(p *player, h *Island, a Action) error {
	if h.Owner != p.profile.ID && (!h.Visitors || !p.profile.Friends[h.Owner]) {
		return errors.New("Only the owner and invited visitors may use this island's shared chest.")
	}
	near := false
	for _, o := range h.Objects {
		if o.Kind == "chest" && distance(p.x, p.z, o.X, o.Z) <= 5 && math.Abs(playerY(p)-o.Y) <= 3 && w.buildEditor(h, o.ID) == "" {
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
	if a.Action == "chest_withdraw" && h.Owner != p.profile.ID {
		return errors.New("Only the island owner may withdraw materials. Guest builders use shared materials directly when building.")
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

func (w *World) storageAction(p *player, h *Island, a Action) error {
	if h.Owner != p.profile.ID {
		return errors.New("This personal storage chest belongs to the island owner.")
	}
	if !isInventoryItem(a.Item) {
		return errors.New("Choose an inventory item to store.")
	}
	found := false
	for _, o := range h.Objects {
		if o.ID == a.Target && o.Kind == "storage_chest" && distance(p.x, p.z, o.X, o.Z) <= 5 && math.Abs(playerY(p)-o.Y) <= 3 && w.buildEditor(h, o.ID) == "" {
			found = true
			break
		}
	}
	if !found {
		return errors.New("Stand near your personal storage chest and finish editing it first.")
	}
	h = cloneIsland(h)
	if h.Storage == nil {
		h.Storage = map[string]map[string]int{}
	}
	if h.Storage[a.Target] == nil {
		h.Storage[a.Target] = map[string]int{}
	}
	contents := h.Storage[a.Target]
	if a.Action == "storage_deposit" {
		if contents[a.Item] > inventoryLimit-a.Amount {
			return errors.New("This chest cannot hold more of that item.")
		}
		if !take(p, a.Item, a.Amount) {
			return errors.New("You do not have those items.")
		}
		contents[a.Item] += a.Amount
	} else {
		if contents[a.Item] < a.Amount {
			return errors.New("This chest does not contain that many items.")
		}
		if !gain(p, a.Item, a.Amount) {
			return errors.New("Make room in your bag first.")
		}
		contents[a.Item] -= a.Amount
		if contents[a.Item] == 0 {
			delete(contents, a.Item)
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
		if w.buildEditor(h, o.ID) != "" {
			return errors.New("Finish editing this object before using it.")
		}
		switch o.Kind {
		case "chair", "bed":
			if p.ridingCamel {
				return errors.New("Press Q to dismount before sitting or lying down.")
			}
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
				resetMovement(p)
				p.health = min(100, p.health+5)
			}
		case "door":
			if o.Open {
				closed := *o
				closed.Open = false
				r, _ := buildRecipeFor(o.Kind)
				for _, occupant := range w.players {
					if occupant.island == p.island && objectBlocks(closed, r, occupant.x, playerY(occupant), occupant.z, .42) {
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
			w.eventActor("Open this mixer to load ingredients, mix dough, or collect a finished batch.", "notification", p.x, p.z, p.profile.ID)
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
				if err := spendBuildMaterials(p, h, map[string]int{a.Item: 1}); err != nil {
					return err
				}
				o.Planted = a.Item
				o.ReadyAt = float64(w.now().Unix() + 120)
			} else {
				if float64(w.now().Unix()) < o.ReadyAt {
					return errors.New("Your garden is still growing (2 minutes).")
				}
				if h.Owner != p.profile.ID {
					if h.Chest[o.Planted] > inventoryLimit-4 {
						return errors.New("Make room in the shared chest for this harvest.")
					}
					h.Chest[o.Planted] += 4
				} else if !gain(p, o.Planted, 4) {
					return errors.New("Make room for the harvest.")
				}
				o.Planted = ""
				o.ReadyAt = 0
			}
		case "pond":
			if len(pondAnimals(*o)) > 0 {
				w.eventActor("Your rescued animals are safe at home. Expand this pond to shelter up to four.", "notification", p.x, p.z, p.profile.ID)
			} else {
				w.eventActor("Find a wounded fish or turtle in the ocean and choose Adopt to bring it to this pond.", "notification", p.x, p.z, p.profile.ID)
			}
		case "chest", "storage_chest", "camel_pen":
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
		if o.Kind == "pond" && len(pondAnimals(o)) < pondCapacity(o) && w.buildEditor(p.profile.HomeIsland, o.ID) == "" {
			pond = i
			break
		}
	}
	if pond < 0 {
		return errors.New("You need a free space in a home pond. Expand a pond to shelter up to four animals.")
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
		o := &p.profile.HomeIsland.Objects[pond]
		normalizePond(o)
		o.AdoptedAnimals = append(o.AdoptedAnimals, a.Species)
		normalizePond(o)
		a.Health = 0
		a.respawnAt = w.time + 120
		p.profile.Rescues++
		changeSpeciesReputation(p, a.Species, 3)
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
	out.Me.MountedCamelID = p.mountedCamelID
	if h, ok := w.currentIsland(p); !ok || !w.canBuildIsland(p, h) {
		p.buildMode, p.editingBuildID = false, ""
	}
	out.Me.BuildMode, out.Me.EditingBuildID = p.buildMode, p.editingBuildID
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
	out.Nodes = append(out.Nodes, w.camelNodes(p)...)
	out.Nodes = append(out.Nodes, w.dropNodes(p)...)
	if p.island != "" {
		out.Nodes = append(out.Nodes, Node{ID: "island_portal", Kind: "island_portal", Label: "Return platform · Sunbaked clay cave", X: islandPortalX, Z: islandPortalZ, Available: true, Island: p.island})
		out.Me.Area = "home"
		out.Me.Zone = "home island"
		out.Animals = []Animal{}
		out.Projectiles = []Projectile{}
		if h, ok := w.currentIsland(p); ok {
			out.Nodes = append(out.Nodes, Node{ID: "island_upgrade", Kind: "island_upgrade", Label: "Expand your home island", X: islandUpgradeX, Z: islandUpgradeZ, Available: true, Island: p.island})
			out.Island = cloneIsland(h)
			for i := range out.Island.Objects {
				out.Island.Objects[i].EditingBy = w.buildEditor(h, out.Island.Objects[i].ID)
			}
			if h.Owner != p.profile.ID {
				out.Island.Storage = nil
			}
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
