package game

import (
	"errors"
	"fmt"
	"math"
)

const islandMaxUpgrades = 5
const islandUpgradeX, islandUpgradeZ = -5.0, 24.0
const camelPenCapacity = 4

var islandUpgradeCosts = [...]int{150, 300, 500, 750, 1000}
var pondUpgradeCosts = [...]int{75, 125, 200}

func islandBuildRadius(h *Island) float64 {
	if h == nil {
		return 32
	}
	return 32 + 5*float64(max(0, min(islandMaxUpgrades, h.UpgradeLevel)))
}
func islandWalkRadius(h *Island) float64 { return islandBuildRadius(h) + 8 }
func normalizeIslandUpgrade(h *Island) {
	h.UpgradeLevel = max(0, min(islandMaxUpgrades, h.UpgradeLevel))
	h.BuildRadius, h.WalkRadius, h.MaxUpgrades = islandBuildRadius(h), islandWalkRadius(h), islandMaxUpgrades
	h.NextUpgradeCost = 0
	if h.UpgradeLevel < islandMaxUpgrades {
		h.NextUpgradeCost = islandUpgradeCosts[h.UpgradeLevel]
	}
}
func validIslandBuildPosition(h *Island, x, y, z, r float64) bool {
	return finite(x) && finite(y) && finite(z) && finite(r) && math.Hypot(x, z) <= islandBuildRadius(h) && y >= 0 && y <= 12
}
func pondCapacity(o BuildObject) int { return max(1, min(4, o.PondCapacity)) }
func pondAnimals(o BuildObject) []string {
	if len(o.AdoptedAnimals) > 0 {
		return o.AdoptedAnimals
	}
	if validChoice(o.Adopted, "fish", "turtle") {
		return []string{o.Adopted}
	}
	return nil
}
func normalizePond(o *BuildObject) {
	if o.Kind != "pond" {
		return
	}
	animals := []string{}
	for _, species := range pondAnimals(*o) {
		if validChoice(species, "fish", "turtle") && len(animals) < 4 {
			animals = append(animals, species)
		}
	}
	o.AdoptedAnimals = animals
	o.PondCapacity = max(pondCapacity(*o), len(animals))
	o.Adopted = ""
	if len(animals) > 0 {
		o.Adopted = animals[0]
	}
	o.PondUpgradeCost = 0
	if o.PondCapacity < 4 {
		o.PondUpgradeCost = pondUpgradeCosts[o.PondCapacity-1]
	}
}

// A pond's footprint grows with its saved capacity. All placement, collision,
// packing and rendering use this same per-object size.
func buildObjectRecipe(o BuildObject) (BuildRecipe, bool) {
	r, ok := buildRecipeFor(o.Kind)
	if ok && o.Kind == "pond" {
		growth := float64(pondCapacity(o) - 1)
		r.Width += growth
		r.Depth += growth
	}
	return r, ok
}
func camelPenSlots(o BuildObject) []Point {
	a := o.Rotation * math.Pi / 180
	slots := make([]Point, 0, camelPenCapacity)
	for _, z := range []float64{-1.6, 1.6} {
		for _, x := range []float64{-1.8, 1.8} {
			slots = append(slots, Point{X: o.X + x*math.Cos(a) + z*math.Sin(a), Z: o.Z - x*math.Sin(a) + z*math.Cos(a)})
		}
	}
	return slots
}
func penBlocks(o BuildObject, r BuildRecipe, x, z, radius float64) bool {
	lx, lz := objectLocal(o, x, z)
	for _, side := range []float64{-1, 1} {
		if circleRect(lx, lz, radius, side*(r.Width/2-.07), 0, .14, r.Depth) {
			return true
		}
	}
	if circleRect(lx, lz, radius, 0, -r.Depth/2+.07, r.Width, .14) {
		return true
	}
	wing := (r.Width - 3.2) / 2
	for _, side := range []float64{-1, 1} {
		if circleRect(lx, lz, radius, side*(1.6+wing/2), r.Depth/2-.07, wing, .14) {
			return true
		}
	}
	return false
}

func (w *World) homeUpgradeAction(p *player, a Action) error {
	h, ok := w.currentIsland(p)
	if !ok || p.island != p.profile.ID || h.Owner != p.profile.ID {
		return errors.New("Only the owner may upgrade their home island or ponds while at home.")
	}
	if a.Action == "upgrade_island" {
		if a.Target != "island_upgrade" || distance(p.x, p.z, islandUpgradeX, islandUpgradeZ) > 3.5 || math.Abs(playerY(p)) > 2 {
			return errors.New("Visit the island upgrade sign beside your return platform.")
		}
		if h.UpgradeLevel >= islandMaxUpgrades {
			return errors.New("Your island already has all five expansions.")
		}
		price := islandUpgradeCosts[max(0, h.UpgradeLevel)]
		if p.profile.Coins < price {
			return errors.New("Save the listed coins before expanding your island.")
		}
		h = cloneIsland(h)
		h.UpgradeLevel++
		normalizeIslandUpgrade(h)
		p.profile.Coins -= price
		w.saveIsland(h)
		w.eventActor(fmt.Sprintf("Island expansion %d of 5: your building radius is now %.0f metres.", h.UpgradeLevel, h.BuildRadius), "notification", p.x, p.z, p.profile.ID)
		return nil
	}
	if a.Action != "upgrade_pond" {
		return errors.New("Choose an island or pond expansion.")
	}
	h = cloneIsland(h)
	for i := range h.Objects {
		o := &h.Objects[i]
		if o.ID != a.Target || o.Kind != "pond" {
			continue
		}
		if distance(p.x, p.z, o.X, o.Z) > 5 || math.Abs(playerY(p)-o.Y) > 3 {
			return errors.New("Stand within five metres of your pond to expand it.")
		}
		if w.buildEditor(h, o.ID) != "" {
			return errors.New("Finish editing the pond before expanding it.")
		}
		normalizePond(o)
		if o.PondCapacity >= 4 {
			return errors.New("This pond already has room for four rescued animals.")
		}
		price := o.PondUpgradeCost
		if p.profile.Coins < price {
			return errors.New("Save the listed coins before expanding this pond.")
		}
		o.PondCapacity++
		normalizePond(o)
		a := Action{X: o.X, Y: o.Y, Z: o.Z, Rotation: o.Rotation}
		if err := w.validatePlacement(p, h, "pond", o.ID, a); err != nil {
			return err
		}
		p.profile.Coins -= price
		w.saveIsland(h)
		w.eventActor(fmt.Sprintf("Your pond now shelters %d rescued animals.", o.PondCapacity), "notification", p.x, p.z, p.profile.ID)
		return nil
	}
	return errors.New("Choose a pond on your own island.")
}
