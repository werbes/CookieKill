package game

import "strings"

const bakeryStaffLimit = 2

type BakeryWorkerView struct {
	ID string `json:"id"`
	Role string `json:"role"`
	Label string `json:"label"`
	OnBreak bool `json:"onBreak"`
	Status string `json:"status"`
}

func bakeryStaffRole(key string) string {
	role := strings.TrimSuffix(key, "_2")
	if validChoice(role, "chef", "server", "cashier") { return role }
	return ""
}

func bakeryWorkerIDs(p *Profile, role string) []string {
	out := []string{}
	if bakeryStaffRole(role) != role { return out }
	for _, id := range []string{role, role+"_2"} {
		if p.BakeryStaff[id] { out = append(out, id) }
	}
	return out
}

func bakeryWorkerOnBreak(p *Profile, id string) bool { return p.BakeryStaffBreaks[id] }

func bakeryWorkerLabel(id string) string {
	role := bakeryStaffRole(id)
	label := strings.ToUpper(role[:1])+role[1:]+" 1"
	if strings.HasSuffix(id, "_2") { label = strings.ToUpper(role[:1])+role[1:]+" 2" }
	return label
}

// The first tray keeps its original save field; additional trays never mirror it.
func bakeryServerTray(p *Profile, id string) map[string]int {
	if id == "server" {
		if p.BakeryCarry == nil { p.BakeryCarry = map[string]int{} }
		return p.BakeryCarry
	}
	if p.BakeryServerCarry == nil { p.BakeryServerCarry = map[string]map[string]int{} }
	if p.BakeryServerCarry[id] == nil { p.BakeryServerCarry[id] = map[string]int{} }
	return p.BakeryServerCarry[id]
}

func normalizeBakeryWorkforce(p *Profile) {
	if p.BakeryStaffBreaks == nil { p.BakeryStaffBreaks = map[string]bool{} }
	for id, hired := range p.BakeryStaff {
		if !hired || bakeryStaffRole(id) == "" { delete(p.BakeryStaff,id) }
	}
	for id, paused := range p.BakeryStaffBreaks {
		if !paused || !p.BakeryStaff[id] { delete(p.BakeryStaffBreaks,id) }
	}
	if p.BakeryServerCarry == nil { p.BakeryServerCarry = map[string]map[string]int{} }
	for id := range p.BakeryServerCarry { if id != "server_2" { delete(p.BakeryServerCarry,id) } }
	for _, id := range []string{"server", "server_2"} {
		tray := bakeryServerTray(p,id)
		remaining := bakeryServerTrayCapacity
		for item, count := range tray { if _,ok := recipeFor(item); !ok || count <= 0 { delete(tray,item) } }
		for _, r := range recipes {
			count := min(remaining,tray[r.ID])
			if count > 0 { tray[r.ID]=count; remaining-=count } else { delete(tray,r.ID) }
		}
	}
}

func bakeryDisplays(p *Profile) []BakeryFurniture {
	out := []BakeryFurniture{}
	for _, f := range p.BakeryFurniture { if f.Kind=="display" { out=append(out,f) } }
	return out
}

func bakeryTotalDisplayCapacity(p *Profile) int { return bakeryDisplayCapacity*len(bakeryDisplays(p)) }

// A shared stock pool is divided evenly in stable recipe order. The two visual
// counters therefore never advertise duplicate cookies or more than 300 each.
func bakeryDisplayStock(p *Profile, id string) map[string]int {
	displays := bakeryDisplays(p)
	index := -1
	for i,f := range displays { if f.ID==id { index=i } }
	out := map[string]int{}
	if index < 0 { return out }
	offset := 0
	for _,r := range recipes {
		n := p.BakeryDisplay[r.ID]
		first := (index-offset%len(displays)+len(displays))%len(displays)
		if n > first { out[r.ID]=(n-1-first)/len(displays)+1 }
		offset+=n
	}
	return out
}

func bakeryFurnitureByID(p *Profile, id string) (BakeryFurniture,bool) {
	for _, f := range p.BakeryFurniture { if f.ID==id { return f,true } }
	return BakeryFurniture{},false
}

func bakeryWorkerStation(p *player, id, kind string) (BakeryFurniture,bool) {
	stations := []BakeryFurniture{}
	for _,f := range p.profile.BakeryFurniture {
		if f.Kind==kind && f.ID != p.editingBakeryFurnitureID { stations=append(stations,f) }
	}
	if len(stations)==0 { return BakeryFurniture{},false }
	index:=0
	if strings.HasSuffix(id,"_2") { index=1%len(stations) }
	return stations[index],true
}
