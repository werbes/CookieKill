package game

import (
	"errors"
	"sort"
)

func syncInventory(p *Profile) {
	p.Inventory = map[string]int{}
	for _, slots := range [][]Stack{p.SafeSlots[:], p.BagSlots[:], p.Hotbar[:]} {
		for _, s := range slots {
			if s.Count > 0 {
				p.Inventory[s.Item] += s.Count
			}
		}
	}
	p.Protected = []string{}
	for _, s := range p.SafeSlots {
		if s.Count > 0 {
			p.Protected = append(p.Protected, s.Item)
		}
	}
	if p.SelectedSlot == 5 && p.HomeIsland != nil {
		p.Selected = "build_menu"
		return
	}
	p.SelectedSlot = max(0, min(4, p.SelectedSlot))
	p.Selected = p.Hotbar[p.SelectedSlot].Item
}

func normalizeInventory(p *Profile) {
	if p.Recovery == nil {
		p.Recovery = map[string]int{}
	}
	if p.InventoryVersion < 2 {
		old := cloneMap(p.Inventory)
		safeIndex := 0
		for _, key := range p.Protected {
			if safeIndex >= 3 {
				break
			}
			if itemSet[key] && old[key] > 0 {
				p.SafeSlots[safeIndex] = Stack{key, min(inventoryLimit, old[key])}
				safeIndex++
				delete(old, key)
			}
		}
		if old[p.Selected] > 0 && itemSet[p.Selected] {
			p.Hotbar[0] = Stack{p.Selected, min(inventoryLimit, old[p.Selected])}
			delete(old, p.Selected)
		}
		keys := []string{}
		for key := range old {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if !itemSet[key] || old[key] <= 0 {
				continue
			}
			n := min(inventoryLimit, old[key])
			if !putEmpty(p.BagSlots[:], key, n) && !putEmpty(p.Hotbar[:], key, n) && !putEmpty(p.SafeSlots[:], key, n) {
				p.Recovery[key] += n
			}
		}
		p.InventoryVersion = 2
	}
	for _, slots := range [][]Stack{p.SafeSlots[:], p.BagSlots[:], p.Hotbar[:]} {
		for i, s := range slots {
			if !itemSet[s.Item] || s.Count <= 0 {
				slots[i] = Stack{}
			} else {
				slots[i].Count = min(inventoryLimit, s.Count)
			}
		}
	}
	for key, n := range p.Recovery {
		if !itemSet[key] || n <= 0 {
			delete(p.Recovery, key)
		} else {
			p.Recovery[key] = min(inventoryLimit, n)
		}
	}
	syncInventory(p)
}

func putEmpty(slots []Stack, item string, count int) bool {
	for i, s := range slots {
		if s.Count == 0 {
			slots[i] = Stack{item, count}
			return true
		}
	}
	return false
}

// New items fill existing stacks, then the bag. A full inventory is an atomic
// refusal; recovery is only for legacy saves, never extra gameplay storage.
func gain(p *player, item string, count int) bool {
	if count <= 0 || !itemSet[item] {
		return false
	}
	next := p.profile
	for _, slots := range [][]Stack{next.BagSlots[:], next.Hotbar[:], next.SafeSlots[:]} {
		for i, s := range slots {
			if s.Item == item && s.Count > 0 {
				n := min(count, inventoryLimit-s.Count)
				slots[i].Count += n
				count -= n
			}
		}
	}
	for count > 0 {
		n := min(count, inventoryLimit)
		if !putEmpty(next.BagSlots[:], item, n) {
			break
		}
		count -= n
	}
	if count > 0 {
		return false
	}
	syncInventory(&next)
	p.profile = next
	return true
}

func take(p *player, item string, count int) bool {
	if count < 0 || p.profile.Inventory[item] < count {
		return false
	}
	for _, slots := range [][]Stack{p.profile.BagSlots[:], p.profile.Hotbar[:], p.profile.SafeSlots[:]} {
		for i, s := range slots {
			if s.Item == item {
				n := min(count, s.Count)
				slots[i].Count -= n
				count -= n
				if slots[i].Count == 0 {
					slots[i] = Stack{}
				}
			}
		}
	}
	syncInventory(&p.profile)
	return true
}
func takeHotbar(p *player, item string) bool {
	// Split stacks can share a cookie type: use the selected stack first.
	for offset := 0; offset < len(p.profile.Hotbar); offset++ {
		i := (p.profile.SelectedSlot + offset) % len(p.profile.Hotbar)
		s := p.profile.Hotbar[i]
		if s.Item == item && s.Count > 0 {
			p.profile.Hotbar[i].Count--
			if s.Count == 1 {
				p.profile.Hotbar[i] = Stack{}
			}
			syncInventory(&p.profile)
			return true
		}
	}
	return false
}
func hasHotbar(p *player, item string) bool {
	for _, s := range p.profile.Hotbar {
		if s.Item == item && s.Count > 0 {
			return true
		}
	}
	return false
}

func inventorySlots(p *Profile, kind string) []Stack {
	switch kind {
	case "safe":
		return p.SafeSlots[:]
	case "bag":
		return p.BagSlots[:]
	case "hotbar":
		return p.Hotbar[:]
	}
	return nil
}
func moveItem(p *player, a Action) error {
	if a.Amount < 0 {
		return errors.New("Choose a positive stack amount, or move the whole stack.")
	}
	from, to := inventorySlots(&p.profile, a.From), inventorySlots(&p.profile, a.To)
	if a.FromSlot < 0 || a.FromSlot >= len(from) || a.ToSlot < 0 || a.ToSlot >= len(to) {
		return errors.New("Choose valid inventory slots.")
	}
	if a.From == a.To && a.FromSlot == a.ToSlot {
		return nil
	}
	src, dst := from[a.FromSlot], to[a.ToSlot]
	if src.Count <= 0 {
		return errors.New("That inventory slot is empty.")
	}
	count := src.Count
	if a.Amount > 0 {
		count = min(count, a.Amount)
	}
	if dst.Count > 0 && dst.Item != src.Item {
		if count != src.Count {
			return errors.New("Move the whole stack to swap different items.")
		}
		from[a.FromSlot], to[a.ToSlot] = dst, src
	} else {
		count = min(count, inventoryLimit-dst.Count)
		to[a.ToSlot] = Stack{src.Item, dst.Count + count}
		from[a.FromSlot].Count -= count
		if from[a.FromSlot].Count == 0 {
			from[a.FromSlot] = Stack{}
		}
	}
	syncInventory(&p.profile)
	return nil
}
func claimRecovery(p *player, item string) error {
	count := p.profile.Recovery[item]
	if count <= 0 {
		return errors.New("There are no saved overflow items of this kind.")
	}
	for i, s := range p.profile.BagSlots {
		if count == 0 {
			break
		}
		if s.Count == 0 || s.Item == item {
			n := min(count, inventoryLimit-s.Count)
			p.profile.BagSlots[i] = Stack{item, s.Count + n}
			count -= n
		}
	}
	if count == p.profile.Recovery[item] {
		return errors.New("Make room in your inventory first.")
	}
	p.profile.Recovery[item] = count
	if count == 0 {
		delete(p.profile.Recovery, item)
	}
	syncInventory(&p.profile)
	return nil
}
