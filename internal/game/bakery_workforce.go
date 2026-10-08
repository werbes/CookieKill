package game

import (
	"fmt"
	"strings"
)

const bakeryStaffLimit = 2

type BakeryWorkerView struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Label   string `json:"label"`
	OnBreak bool   `json:"onBreak"`
	Status  string `json:"status"`
}

func bakeryStaffRole(key string) string {
	role := strings.TrimSuffix(key, "_2")
	if validChoice(role, "chef", "server", "cashier") {
		return role
	}
	return ""
}

func bakeryWorkerIDs(p *Profile, role string) []string {
	out := []string{}
	if bakeryStaffRole(role) != role {
		return out
	}
	for _, id := range []string{role, role + "_2"} {
		if p.BakeryStaff[id] {
			out = append(out, id)
		}
	}
	return out
}

func bakeryWorkerOnBreak(p *Profile, id string) bool { return p.BakeryStaffBreaks[id] }

func bakeryWorkerLabel(id string) string {
	role := bakeryStaffRole(id)
	label := strings.ToUpper(role[:1]) + role[1:] + " 1"
	if strings.HasSuffix(id, "_2") {
		label = strings.ToUpper(role[:1]) + role[1:] + " 2"
	}
	return label
}

// The first tray keeps its original save field; additional trays never mirror it.
func bakeryServerTray(p *Profile, id string) map[string]int {
	if id == "server" {
		if p.BakeryCarry == nil {
			p.BakeryCarry = map[string]int{}
		}
		return p.BakeryCarry
	}
	if p.BakeryServerCarry == nil {
		p.BakeryServerCarry = map[string]map[string]int{}
	}
	if p.BakeryServerCarry[id] == nil {
		p.BakeryServerCarry[id] = map[string]int{}
	}
	return p.BakeryServerCarry[id]
}

func normalizeBakeryWorkforce(p *Profile) {
	if p.BakeryStaffBreaks == nil {
		p.BakeryStaffBreaks = map[string]bool{}
	}
	for id, hired := range p.BakeryStaff {
		if !hired || bakeryStaffRole(id) == "" {
			delete(p.BakeryStaff, id)
		}
	}
	for id, paused := range p.BakeryStaffBreaks {
		if !paused || !p.BakeryStaff[id] {
			delete(p.BakeryStaffBreaks, id)
		}
	}
	if p.BakeryServerCarry == nil {
		p.BakeryServerCarry = map[string]map[string]int{}
	}
	for id := range p.BakeryServerCarry {
		if id != "server_2" {
			delete(p.BakeryServerCarry, id)
		}
	}
	for _, id := range []string{"server", "server_2"} {
		tray := bakeryServerTray(p, id)
		remaining := bakeryServerTrayCapacity
		for item, count := range tray {
			if _, ok := recipeFor(item); !ok || count <= 0 {
				delete(tray, item)
			}
		}
		for _, r := range recipes {
			count := min(remaining, tray[r.ID])
			if count > 0 {
				tray[r.ID] = count
				remaining -= count
			} else {
				delete(tray, r.ID)
			}
		}
	}
}

func bakeryDisplays(p *Profile) []BakeryFurniture {
	out := []BakeryFurniture{}
	for _, f := range p.BakeryFurniture {
		if f.Kind == "display" {
			out = append(out, f)
		}
	}
	return out
}

func bakeryTotalDisplayCapacity(p *Profile) int {
	return bakeryDisplayCapacity * len(bakeryDisplays(p))
}

// A shared stock pool is divided evenly in stable recipe order. The two visual
// counters therefore never advertise duplicate cookies or more than 300 each.
func bakeryDisplayStock(p *Profile, id string) map[string]int {
	displays := bakeryDisplays(p)
	index := -1
	for i, f := range displays {
		if f.ID == id {
			index = i
		}
	}
	out := map[string]int{}
	if index < 0 {
		return out
	}
	offset := 0
	remaining := bakeryDisplayCapacity
	for _, r := range recipes {
		n := p.BakeryDisplay[r.ID]
		first := (index - offset%len(displays) + len(displays)) % len(displays)
		if n > first {
			count := min(remaining, (n-1-first)/len(displays)+1)
			if count > 0 {
				out[r.ID] = count
				remaining -= count
			}
		}
		offset += n
	}
	return out
}

func bakeryFurnitureByID(p *Profile, id string) (BakeryFurniture, bool) {
	for _, f := range p.BakeryFurniture {
		if f.ID == id {
			return f, true
		}
	}
	return BakeryFurniture{}, false
}

func bakeryWorkerStation(p *player, id, kind string) (BakeryFurniture, bool) {
	var first, second BakeryFurniture
	for _, f := range p.profile.BakeryFurniture {
		if f.Kind == kind && f.ID != p.editingBakeryFurnitureID {
			if first.ID == "" {
				first = f
			} else {
				second = f
				break
			}
		}
	}
	if first.ID == "" {
		return BakeryFurniture{}, false
	}
	if strings.HasSuffix(id, "_2") && second.ID != "" {
		return second, true
	}
	return first, true
}

func (w *World) admitBakeryCustomer(p *player) {
	b := p.bakery
	displays := bakeryDisplays(&p.profile)
	if len(displays) == 0 || bakeryStockCount(p.profile.BakeryDisplay) == 0 {
		return
	}
	plot, _ := w.bakeryPlot(p)
	choices := []string{}
	for _, r := range recipes {
		if p.profile.BakeryDisplay[r.ID] > 0 {
			choices = append(choices, r.ID)
		}
	}
	for i := 0; i < len(displays); i++ {
		index := (b.customerDisplayCursor + i) % len(displays)
		f := displays[index]
		if f.ID == p.editingBakeryFurnitureID {
			continue
		}
		used := [3]bool{}
		for _, c := range b.customers {
			if c.DisplayID == f.ID && c.slot >= 0 && c.slot < len(used) {
				used[c.slot] = true
			}
		}
		slot := 0
		for slot < len(used) && used[slot] {
			slot++
		}
		if slot == len(used) {
			continue
		}
		w.sequence++
		item := choices[int(w.sequence%uint64(len(choices)))]
		c := &bakeryCustomer{BakeryCustomerView: BakeryCustomerView{ID: fmt.Sprintf("customer_%d", w.sequence), Item: item, Amount: min(5, p.profile.BakeryDisplay[item]), Phase: "arriving", DisplayID: f.ID}, x: plot.X, z: plot.Z + 9.5, slot: slot}
		w.routeBakeryCustomer(p, c)
		if c.hasRoute && c.Phase == "arriving" {
			b.customers = append(b.customers, c)
			b.customerDisplayCursor = (index + 1) % len(displays)
			return
		}
	}
}

func (w *World) workBakeryCashiers(p *player) {
	for _, id := range bakeryWorkerIDs(&p.profile, "cashier") {
		worker := p.bakery.workers[id]
		if !w.bakeryWorkerReady(p, id) || w.time < worker.readyAt {
			continue
		}
		worker.readyAt = w.time + 3
		for _, c := range p.bakery.customers {
			if c.DisplayID == worker.displayID && c.Phase == "waiting" && p.profile.BakeryDisplay[c.Item] >= c.Amount {
				if w.serveBakeryCustomer(p, c) == nil {
					worker.status = "Served a customer"
					break
				}
			}
		}
	}
}

func bakeryWorkerViews(p *player) []BakeryWorkerView {
	out := []BakeryWorkerView{}
	for _, role := range []string{"chef", "server", "cashier"} {
		for _, id := range bakeryWorkerIDs(&p.profile, role) {
			v := BakeryWorkerView{ID: id, Role: role, Label: bakeryWorkerLabel(id), OnBreak: bakeryWorkerOnBreak(&p.profile, id), Status: "Starting shift"}
			if p.bakery != nil {
				if worker := p.bakery.workers[id]; worker != nil {
					v.Status = worker.status
				}
			}
			if v.OnBreak {
				v.Status = "On break"
			}
			out = append(out, v)
		}
	}
	return out
}
