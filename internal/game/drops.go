package game

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

const groundItemTTL = 30 * time.Minute

// Bound transient world entities before allocating or removing any inventory.
const maxGroundBundles = 20000

type groundDrop struct {
	node    Node
	expires time.Time
}

func dropBundleCounts(count int) []int {
	bundles := make([]int, 0, count/10+5)
	for count >= 10 {
		bundles = append(bundles, 10)
		count -= 10
	}
	if count >= 5 {
		bundles = append(bundles, 5)
		count -= 5
	}
	for ; count > 0; count-- {
		bundles = append(bundles, 1)
	}
	return bundles
}

func (w *World) pruneDrops() {
	now := w.now()
	for id, drop := range w.drops {
		if !now.Before(drop.expires) {
			delete(w.drops, id)
		}
	}
}

// Validate the short approach, including its endpoint, so pickups cannot move
// items through walls or between cave/upper-floor layers.
func (w *World) reachableDrop(p *player, x, y, z float64) bool {
	if distance(p.x, p.z, x, z) > 5 || math.Abs(groundY(p)-y) > 2 {
		return false
	}
	candidate := *p
	candidate.jumpOffset, candidate.jumpVelocity = 0, 0
	steps := max(1, int(math.Ceil(distance(p.x, p.z, x, z)/.25)))
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		nx, nz := p.x+(x-p.x)*t, p.z+(z-p.z)*t
		if p.island != "" {
			if w.islandBlocked(&candidate, nx, nz) {
				return false
			}
			candidate.x, candidate.z = nx, nz
			candidate.y = w.islandGround(&candidate)
		} else {
			if !w.moveMainland(&candidate, nx, nz, groundY(&candidate)) || candidate.inCave != p.inCave {
				return false
			}
			candidate.x, candidate.z = nx, nz
		}
	}
	return math.Abs(groundY(&candidate)-y) <= .5
}

func (w *World) dropPosition(p *player, index int) (float64, float64, float64) {
	// Scatter singles and bundles around the player without crossing walls.
	// Reuse the small ring for unusually large stacks to keep every bundle near.
	angle := p.yaw + float64(index%16)*2.4
	radius := .65 + float64(index%3)*.4
	x, z := p.x+math.Cos(angle)*radius, p.z+math.Sin(angle)*radius
	candidate := *p
	candidate.x, candidate.z = x, z
	if p.island != "" {
		candidate.y = w.islandGround(&candidate)
	}
	y := groundY(&candidate)
	if w.reachableDrop(p, x, y, z) {
		return x, y, z
	}
	return p.x, groundY(p), p.z
}

func (w *World) dropItem(p *player, a Action) error {
	slots := inventorySlots(&p.profile, a.From)
	if a.FromSlot < 0 || a.FromSlot >= len(slots) {
		return errors.New("Choose an inventory or hotbar slot to drop.")
	}
	s := slots[a.FromSlot]
	if s.Count <= 0 || !isInventoryItem(s.Item) || a.Item != "" && a.Item != s.Item {
		return errors.New("That inventory item is no longer in this slot.")
	}
	if a.Amount != 0 && a.Amount != 1 && a.Amount != 5 && a.Amount != 10 {
		return errors.New("Choose Drop 1, 5, 10, or all.")
	}
	count := a.Amount
	if count == 0 {
		count = s.Count
	}
	if count > s.Count {
		return errors.New("There are not enough items in that stack.")
	}
	w.pruneDrops()
	bundleCount := count/10 + (count%10)/5 + count%5
	if bundleCount > maxGroundBundles-len(w.drops) {
		return errors.New("There are too many items on the ground. Pick some up or drop fewer items.")
	}
	if w.drops == nil {
		w.drops = make(map[string]groundDrop)
	}
	expires := w.now().Add(groundItemTTL)
	positions := make([][3]float64, min(bundleCount, 48))
	for i := range positions {
		x, y, z := w.dropPosition(p, i)
		positions[i] = [3]float64{x, y, z}
	}
	for i, amount := range dropBundleCounts(count) {
		position := positions[i%len(positions)]
		x, y, z := position[0], position[1], position[2]
		w.dropSequence++
		id := fmt.Sprintf("item_drop_%d", w.dropSequence)
		node := Node{ID: id, Kind: "item_drop", Item: s.Item, Amount: amount, X: x, Y: y, Z: z, Island: p.island, InCave: p.inCave, Available: true}
		w.drops[id] = groundDrop{node: node, expires: expires}
	}
	slots[a.FromSlot].Count -= count
	if slots[a.FromSlot].Count == 0 {
		slots[a.FromSlot] = Stack{}
	}
	syncInventory(&p.profile)
	return nil
}

func (w *World) pickupItem(p *player, id string) error {
	drop, ok := w.drops[id]
	if !ok || !w.now().Before(drop.expires) {
		delete(w.drops, id)
		return errors.New("That dropped item is no longer here.")
	}
	n := drop.node
	if n.Island != p.island || n.InCave != p.inCave || !w.reachableDrop(p, n.X, n.Y, n.Z) {
		return errors.New("Move close to the dropped item to pick it up.")
	}
	if !gain(p, n.Item, n.Amount) {
		return errors.New("Make room in your inventory before picking this up.")
	}
	// A later drop creates a fresh timer; held items never expire.
	delete(w.drops, id)
	return nil
}

func (w *World) dropNodes(viewer *player) []Node {
	nodes := []Node{}
	now := w.now()
	for _, drop := range w.drops {
		if drop.node.Island == viewer.island && drop.node.InCave == viewer.inCave && now.Before(drop.expires) {
			nodes = append(nodes, drop.node)
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return nodes
}
