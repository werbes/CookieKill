package game

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const nutIslandPrice = 1000
const nutCagePrice = 150
const nutCageCapacity = 4
const nutIslandWalkRadius = 36.0
const nutIslandMaxPets = 100

type NutIsland struct {
	Owner  string    `json:"owner"`
	NextID int       `json:"nextId"`
	Pets   []NutPet  `json:"pets"`
	Cages  []NutCage `json:"cages"`
}

type NutPet struct {
	ID      string  `json:"id"`
	Species string  `json:"species"`
	CageID  string  `json:"cageId"`
	X       float64 `json:"x"`
	Z       float64 `json:"z"`
	Yaw     float64 `json:"yaw"`
}

type NutCage struct {
	ID       string  `json:"id"`
	X        float64 `json:"x"`
	Z        float64 `json:"z"`
	Capacity int     `json:"capacity"`
}

type NutPetView struct {
	NutPet
	Y float64 `json:"y"`
}

type NutIslandView struct {
	Owner      string       `json:"owner"`
	WalkRadius float64      `json:"walkRadius"`
	Pets       []NutPetView `json:"pets"`
	Cages      []NutCage    `json:"cages"`
	CagePrice  int          `json:"cagePrice"`
	MaxCages   int          `json:"maxCages"`
	MaxPets    int          `json:"maxPets"`
}

var nutCagePlots = []Point{{16, 18}, {-16, 18}, {16, 6}, {-16, 6}, {16, -6}, {-16, -6}, {16, -18}, {-16, -18}}
var nutGroveTrees = []Point{{-27, -11}, {-28, 10}, {27, -11}, {28, 10}, {-7, -25}, {0, -25}, {7, -25}, {-25, 23}, {25, 23}}

func nutIslandID(owner string) string { return "nut:" + owner }
func isNutIsland(id string) bool      { return strings.HasPrefix(id, "nut:") }

func cloneNutIsland(h *NutIsland) *NutIsland {
	if h == nil {
		return nil
	}
	clone := *h
	clone.Pets = append([]NutPet{}, h.Pets...)
	clone.Cages = append([]NutCage{}, h.Cages...)
	return &clone
}

func nextNutID(h *NutIsland, kind string) string {
	for {
		h.NextID++
		id := "nut_" + kind + "_" + strconv.Itoa(h.NextID)
		used := false
		for _, pet := range h.Pets {
			used = used || pet.ID == id
		}
		for _, cage := range h.Cages {
			used = used || cage.ID == id
		}
		if !used {
			return id
		}
	}
}

func nutCage(h *NutIsland, id string) *NutCage {
	for i := range h.Cages {
		if h.Cages[i].ID == id {
			return &h.Cages[i]
		}
	}
	return nil
}

func nutCageCount(h *NutIsland, id string) int {
	count := 0
	for _, pet := range h.Pets {
		if pet.CageID == id {
			count++
		}
	}
	return count
}

func nutPointBlocked(h *NutIsland, x, z, radius float64) bool {
	if !finite(x) || !finite(z) || math.Hypot(x, z) > nutIslandWalkRadius-radius {
		return true
	}
	for _, trunk := range nutGroveTrees {
		if distance(x, z, trunk.X, trunk.Z) < .39+radius {
			return true
		}
	}
	for _, cage := range h.Cages {
		if circleRect(x, z, radius, cage.X, cage.Z, 7, 7) {
			return true
		}
	}
	return false
}

func nutFreePoint(h *NutIsland, x, z float64) Point {
	for ring := 0; ring < 20; ring++ {
		for i := 0; i < 16; i++ {
			a := float64(i) * math.Pi / 8
			point := Point{x + float64(ring)*.8*math.Cos(a), z + float64(ring)*.8*math.Sin(a)}
			if !nutPointBlocked(h, point.X, point.Z, .6) {
				return point
			}
		}
	}
	return Point{0, 0} // The center lane is always outside fixed cage plots.
}

func normalizeNutIsland(p *Profile) {
	h := p.NutIsland
	if h == nil {
		return
	}
	h.Owner = p.ID
	h.NextID = max(0, min(1000000000, h.NextID))
	if len(h.Cages) > len(nutCagePlots) {
		h.Cages = h.Cages[:len(nutCagePlots)]
	}
	seen := map[string]bool{}
	for i := range h.Cages {
		cage := &h.Cages[i]
		if cage.ID == "" || seen[cage.ID] {
			cage.ID = nextNutID(h, "cage")
		}
		seen[cage.ID] = true
		cage.X, cage.Z, cage.Capacity = nutCagePlots[i].X, nutCagePlots[i].Z, nutCageCapacity
	}
	kept := make([]NutPet, 0, len(h.Pets))
	occupancy := map[string]int{}
	for _, pet := range h.Pets {
		if _, valid := rodentSpeciesFor(pet.Species); !valid {
			continue
		}
		if pet.ID == "" || seen[pet.ID] {
			pet.ID = nextNutID(h, "pet")
		}
		seen[pet.ID] = true
		if !finite(pet.X) || !finite(pet.Z) || !finite(pet.Yaw) {
			pet.X, pet.Z, pet.Yaw = 0, 0, 0
		}
		cage := nutCage(h, pet.CageID)
		if cage == nil || occupancy[pet.CageID] >= nutCageCapacity {
			pet.CageID = ""
			point := nutFreePoint(h, pet.X, pet.Z)
			pet.X, pet.Z = point.X, point.Z
		} else {
			occupancy[cage.ID]++
			pet.X = max(cage.X-2.4, min(cage.X+2.4, pet.X))
			pet.Z = max(cage.Z-2.4, min(cage.Z+2.4, pet.Z))
		}
		kept = append(kept, pet)
	}
	h.Pets = kept
}

func (w *World) seedNutIsland() {
	w.nodes = append(w.nodes, &Node{ID: "green_lollipop", Kind: "green_lollipop", Label: "Green lollipop - Nut Island for 1000 coins", X: 120, Y: CaveFloorY, Z: -126, InCave: true, Available: true})
}

func ownNutIsland(p *player) (*NutIsland, error) {
	h := p.profile.NutIsland
	if h == nil || p.island != nutIslandID(p.profile.ID) || h.Owner != p.profile.ID || p.inCave {
		return nil, errors.New("Visit your own Nut Island to release or care for captured animals.")
	}
	if !grounded(p) || math.Abs(playerY(p)) > 2.5 {
		return nil, errors.New("Stand on the island ground to care for your animals.")
	}
	return h, nil
}

func (w *World) nutIslandAction(p *player, a Action) error {
	switch a.Action {
	case "nut_island_unlock":
		if p.profile.NutIsland != nil {
			return errors.New("You already own Nut Island. Use Teleport to visit it.")
		}
		if a.Target != "green_lollipop" || p.island != "" || !p.inCave || !grounded(p) || math.Abs(playerY(p)-CaveFloorY) > 2.5 || distance(p.x, p.z, 120, -126) > 4 {
			return errors.New("Stand beside the green lollipop at the end of the desert cave.")
		}
		if p.profile.Coins < nutIslandPrice {
			return errors.New("Nut Island costs 1000 coins.")
		}
		p.profile.Coins -= nutIslandPrice
		p.profile.NutIsland = &NutIsland{Owner: p.profile.ID, Pets: []NutPet{}, Cages: []NutCage{}}
		w.eventActor("Nut Island is yours! Visit it through Teleport and release captured animals from your inventory.", "notification", p.x, p.z, p.profile.ID)
		return nil
	case "teleport_nut_island":
		if p.profile.NutIsland == nil {
			return errors.New("Unlock Nut Island for 1000 coins at the green lollipop in the desert cave.")
		}
		w.setIsland(p, nutIslandID(p.profile.ID))
		return nil
	case "nut_island_leave":
		if !isNutIsland(p.island) {
			return errors.New("You are already outside Nut Island.")
		}
		w.setIsland(p, "")
		p.x, p.y, p.z, p.inCave = 120, CaveFloorY, -117, true
		syncMountedCamel(p)
		return nil
	}
	h, err := ownNutIsland(p)
	if err != nil {
		return err
	}
	if a.Action == "nut_cage_buy" {
		if distance(p.x, p.z, 4, 22) > 5 {
			return errors.New("Visit the cage workshop by the return platform.")
		}
		if len(h.Cages) >= len(nutCagePlots) {
			return errors.New("All eight animal cages are already built.")
		}
		if p.profile.Coins < nutCagePrice {
			return errors.New("A roomy four-animal cage costs 150 coins.")
		}
		plot := nutCagePlots[len(h.Cages)]
		for _, camel := range p.profile.Camels {
			if camel.Island == p.island && circleRect(camel.X, camel.Z, .8, plot.X, plot.Z, 7, 7) {
				return errors.New("Move your camel away from the next cage plot first.")
			}
		}
		for _, pet := range h.Pets {
			if circleRect(pet.X, pet.Z, .6, plot.X, plot.Z, 7, 7) {
				return errors.New("Pack or move the animal on the next cage plot before building there.")
			}
		}
		p.profile.Coins -= nutCagePrice
		h.Cages = append(h.Cages, NutCage{ID: nextNutID(h, "cage"), X: plot.X, Z: plot.Z, Capacity: nutCageCapacity})
		return nil
	}
	if a.Action == "rodent_release" {
		species, valid := capturedRodentItem(a.Item)
		if !valid || p.profile.Inventory[a.Item] < 1 {
			return errors.New("Choose a captured animal from your inventory.")
		}
		if len(h.Pets) >= nutIslandMaxPets {
			return errors.New("Nut Island currently has room for 100 released animals. Pack one to make space.")
		}
		point := nutFreePoint(h, p.x+2*math.Cos(p.yaw), p.z+2*math.Sin(p.yaw))
		if a.Target != "" {
			cage := nutCage(h, a.Target)
			if cage == nil || distance(p.x, p.z, cage.X, cage.Z) > 5 {
				return errors.New("Stand beside one of your cages to release an animal into it.")
			}
			if nutCageCount(h, cage.ID) >= nutCageCapacity {
				return errors.New("That cage already shelters four animals.")
			}
			point = Point{cage.X, cage.Z}
		}
		if !take(p, a.Item, 1) {
			return errors.New("That captured animal is no longer in your inventory.")
		}
		h.Pets = append(h.Pets, NutPet{ID: nextNutID(h, "pet"), Species: species, CageID: a.Target, X: point.X, Z: point.Z})
		return nil
	}
	index := -1
	for i := range h.Pets {
		if h.Pets[i].ID == a.Target {
			index = i
			break
		}
	}
	if index < 0 {
		return errors.New("Choose one of your released animals.")
	}
	reach := 4.0
	if h.Pets[index].CageID != "" {
		reach = 5
	}
	if distance(p.x, p.z, h.Pets[index].X, h.Pets[index].Z) > reach {
		return errors.New("Walk closer to this animal or its cage to care for it.")
	}
	pet := &h.Pets[index]
	switch a.Action {
	case "nut_pet_pack":
		if !gain(p, "pet_"+pet.Species, 1) {
			return errors.New("Make room in your bag before picking up this animal.")
		}
		h.Pets = append(h.Pets[:index], h.Pets[index+1:]...)
	case "nut_pet_cage":
		cage := nutCage(h, a.Item)
		if cage == nil {
			return errors.New("Choose one of your island's animal cages.")
		}
		if pet.CageID == cage.ID {
			return errors.New("This animal is already in that cage.")
		}
		if nutCageCount(h, cage.ID) >= nutCageCapacity {
			return errors.New("That cage already shelters four animals.")
		}
		pet.CageID, pet.X, pet.Z = cage.ID, cage.X, cage.Z
	case "nut_pet_free":
		if pet.CageID == "" {
			return errors.New("This animal is already roaming freely.")
		}
		point := nutFreePoint(h, p.x, p.z)
		pet.CageID, pet.X, pet.Z = "", point.X, point.Z
	default:
		return errors.New("Choose Release, Cage, Free, or Pick up for your animal.")
	}
	return nil
}

func nutPetPhase(id string) float64 {
	value := uint32(2166136261)
	for _, c := range id {
		value = (value ^ uint32(c)) * 16777619
	}
	return float64(value%6283) / 1000
}

func (w *World) tickNutIslandPets(dt float64) {
	if !finite(dt) || dt <= 0 {
		return
	}
	for _, p := range w.players {
		h := p.profile.NutIsland
		if h == nil || p.disconnected {
			continue
		}
		for i := range h.Pets {
			pet := &h.Pets[i]
			phase := nutPetPhase(pet.ID)
			t := w.time/18 + phase
			goal, speed := Point{28 * math.Cos(t), 28 * math.Sin(t*.87+phase*.3)}, 2.2
			cage := nutCage(h, pet.CageID)
			if cage != nil {
				goal, speed = Point{cage.X + 2.2*math.Cos(t*2), cage.Z + 2.2*math.Sin(t*1.7+phase)}, .8
			}
			dx, dz := goal.X-pet.X, goal.Z-pet.Z
			d := math.Hypot(dx, dz)
			if d < .01 {
				continue
			}
			step := min(speed*min(dt, .1), d) / d
			dx, dz = dx*step, dz*step
			if cage != nil {
				pet.X, pet.Z = max(cage.X-2.4, min(cage.X+2.4, pet.X+dx)), max(cage.Z-2.4, min(cage.Z+2.4, pet.Z+dz))
			} else {
				if !nutPointBlocked(h, pet.X+dx, pet.Z, .35) {
					pet.X += dx
				}
				if !nutPointBlocked(h, pet.X, pet.Z+dz, .35) {
					pet.Z += dz
				}
			}
			pet.Yaw = math.Atan2(-dx, -dz)
		}
	}
}

func (w *World) nutIslandBlocked(p *player, x, z float64) bool {
	h := p.profile.NutIsland
	return h == nil || p.island != nutIslandID(p.profile.ID) || nutPointBlocked(h, x, z, .65)
}

func (w *World) nutIslandSnapshot(p *player, out *Snapshot) {
	if p == nil || !isNutIsland(p.island) {
		return
	}
	h := p.profile.NutIsland
	if h == nil || p.island != nutIslandID(p.profile.ID) {
		return
	}
	view := &NutIslandView{Owner: h.Owner, WalkRadius: nutIslandWalkRadius, Pets: []NutPetView{}, Cages: append([]NutCage{}, h.Cages...), CagePrice: nutCagePrice, MaxCages: len(nutCagePlots), MaxPets: nutIslandMaxPets}
	out.NutIsland = view
	out.Island = nil
	out.Me.Area, out.Me.Zone, out.Me.CanBuildHome = "nut island", "Nut Island animal haven", false
	kept := out.Nodes[:0]
	for _, node := range out.Nodes {
		if node.Kind != "island_portal" && node.Kind != "island_upgrade" {
			kept = append(kept, node)
		}
	}
	out.Nodes = kept
	out.Nodes = append(out.Nodes, Node{ID: "nut_island_portal", Kind: "nut_return", Label: "Return to the desert cave", X: 0, Z: 28, Available: true, Island: p.island}, Node{ID: "nut_cage_shop", Kind: "nut_cage_shop", Label: "Cage workshop - 150 coins for four animals", X: 4, Z: 22, Available: true, Island: p.island})
	for _, cage := range h.Cages {
		out.Nodes = append(out.Nodes, Node{ID: cage.ID, Kind: "nut_cage", Label: fmt.Sprintf("Animal cage - %d / 4 residents", nutCageCount(h, cage.ID)), X: cage.X, Z: cage.Z, Available: true, Island: p.island})
	}
	for _, pet := range h.Pets {
		view.Pets = append(view.Pets, NutPetView{NutPet: pet})
		species, _ := rodentSpeciesFor(pet.Species)
		out.Nodes = append(out.Nodes, Node{ID: pet.ID, Kind: "nut_pet", Label: species.Name, X: pet.X, Z: pet.Z, Available: true, Island: p.island})
	}
	for i := range out.Players {
		out.Players[i].Area = "nut island"
	}
}
