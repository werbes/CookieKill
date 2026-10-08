package game

import (
	"errors"
	"math"
	"strings"
)

const bakeryStationRange = 2.6

// A bakery purchase belongs to the account. Its city plot is occupied only for
// the current connection, so even a full city can welcome returning bakers.
func (w *World) ReleaseBakeryPlot(id string) {
	if p := w.players[id]; p != nil && p.profile.BakeryPlot != "" {
		plotID := p.profile.BakeryPlot
		for _, visitor := range w.players {
			if strings.HasPrefix(visitor.bakerySeatID, plotID+"_seat_") {
				w.releaseBakerySeat(visitor)
			}
		}
		p.profile.BakeryPlot = ""
		p.bakery = nil
		p.bakeryBuildMode, p.editingBakeryFurnitureID = false, ""
		w.rebuildPropertyColliders()
	}
}

func bakeryStationID(plot, station string) string { return plot + "_" + station }

func bakeryStationKind(kind string) bool {
	return validChoice(kind, "oven", "mixer", "display", "cooling_table", "fridge")
}
func bakeryProductionKey(f BakeryFurniture) string {
	if f.ProductionKey != "" {
		return f.ProductionKey
	}
	if validChoice(f.Kind, "oven", "mixer") && f.ID != f.Kind {
		return "bakery:" + f.ID
	}
	return "bakery_" + f.Kind
}
func bakeryFurnitureNodeID(plot string, f BakeryFurniture) string {
	if validChoice(f.Kind, "oven", "mixer") && bakeryProductionKey(f) != "bakery_"+f.Kind {
		return plot + "_station_" + f.ID
	}
	return bakeryStationID(plot, f.Kind)
}
func (w *World) bakeryFurnitureForTarget(p *player, kind, target string) (BakeryFurniture, bool) {
	plot, ok := w.bakeryPlot(p)
	if !ok {
		return BakeryFurniture{}, false
	}
	var chosen BakeryFurniture
	nearest := math.Inf(1)
	for _, f := range p.profile.BakeryFurniture {
		if !bakeryStationKind(f.Kind) || kind != "" && f.Kind != kind {
			continue
		}
		if target != "" {
			if bakeryFurnitureNodeID(plot.ID, f) == target {
				return f, true
			}
			continue
		}
		if d := distance(p.x, p.z, plot.X+f.X, plot.Z+f.Z); d < nearest {
			chosen, nearest = f, d
		}
	}
	return chosen, chosen.ID != ""
}

// A claimed layout can appear beneath visitors. Move only intersecting players
// to a clear point; saved layouts may also furnish the default central aisle.
func (w *World) clearNewBakeryDisplay(plotID string) {
	owner := w.players[w.plotOwner(plotID)]
	if owner == nil {
		return
	}
	for _, plot := range w.layout.Plots {
		if plot.ID != plotID {
			continue
		}
		for _, p := range w.players {
			if p.island != "" || p.inCave {
				continue
			}
			for _, furnishing := range w.bakeryFurnitureColliders(owner) {
				if playerY(p) >= furnishing.Y+furnishing.Height || playerY(p)+1.7 <= furnishing.Y {
					continue
				}
				if !colliderOverlaps(furnishing, p.x, p.z, .42) {
					continue
				}
				candidates := []Point{{plot.X, plot.Z}, {plot.X, plot.Z - 3}, {plot.X, plot.Z + 3}, {plot.X, plot.Z + 9.5}}
				for x := -9.; x <= 9; x++ {
					for z := -7.; z <= 7; z++ {
						candidates = append(candidates, Point{plot.X + x, plot.Z + z})
					}
				}
				for _, safe := range candidates {
					if !w.solidBlockedAt(safe.X, 0, safe.Z, .42) {
						p.x, p.y, p.z = safe.X, 0, safe.Z
						break
					}
				}
				p.posture = ""
				resetMovement(p)
				break
			}
		}
		return
	}
}

func (w *World) seedBakeryStations() {
	for _, plot := range w.layout.Plots {
		for _, station := range []struct {
			kind, label string
			x, z        float64
		}{
			{"oven", "Bakery oven", -5, -5.5},
			{"mixer", "Dough mixer", 5, -5.5},
			{"cooling_table", "Cooling table", -1.8, -5.5},
			{"display", "Cookie display", 5, 3},
			{"fridge", "Kitchen fridge", 8, -5.5},
		} {
			w.nodes = append(w.nodes, &Node{ID: bakeryStationID(plot.ID, station.kind), Kind: "bakery_" + station.kind, PlotID: plot.ID, Label: station.label, X: plot.X + station.x, Z: plot.Z + station.z, Available: true})
		}
	}
}

func (w *World) requireBakeryStation(p *player, station, target string) error {
	if !p.profile.Bakery {
		return errors.New("Buy your first bakery at a free city plot.")
	}
	plot := p.profile.BakeryPlot
	if plot == "" || w.plotOwner(plot) != p.profile.ID {
		return errors.New("Claim a free city plot to reopen your saved bakery at no cost.")
	}
	f, ok := w.bakeryFurnitureForTarget(p, station, target)
	if !ok {
		return errors.New("Use the matching station in your own bakery.")
	}
	location, _ := w.bakeryPlot(p)
	if p.bakeryBuildMode || p.editingBakeryFurnitureID == f.ID {
		return errors.New("Finish editing your bakery before using this station.")
	}
	if p.island != "" || p.inCave || math.Abs(playerY(p)) > 2.5 || distance(p.x, p.z, location.X+f.X, location.Z+f.Z) > bakeryStationRange {
		return errors.New("Move closer to your bakery's " + station + ".")
	}
	return nil
}

// All three base stations are part of the purchased bakery; kitchen equipment
// upgrades improve them without moving their interaction points.
func (w *World) bakeryNodeView(n Node, owners map[string]*player) Node {
	plot := n.PlotID
	if n.Kind == "bakery_plot" {
		plot = n.ID
		n.Label = "Available bakery plot"
	}
	if plot == "" {
		if n.Paint != nil {
			n.Paint = cloneMap(n.Paint)
		}
		if n.Equipment != nil {
			n.Equipment = cloneMap(n.Equipment)
		}
		return n
	}
	p := owners[plot]
	n.Owner, n.CallName, n.Username = "", "", ""
	n.Paint, n.Equipment, n.BakeryLevel = nil, nil, 0
	n.Stock, n.BakeryFurniture, n.Production, n.LinkedTable = nil, nil, nil, ""
	n.FurnitureID, n.ProductionKey = "", ""
	if n.PlotID != "" {
		n.Available = p != nil
	}
	if p != nil {
		n.Owner, n.CallName, n.Username = p.profile.ID, p.profile.CallName, p.profile.Username
		if n.Kind == "bakery_plot" {
			n.Label = p.profile.CallName + "'s bakery"
			if p.profile.BakeryName != "" {
				n.Label = p.profile.BakeryName
			}
		}
		n.Paint, n.Equipment = cloneMap(p.profile.Paint), cloneMap(p.profile.Equipment)
		n.BakeryLevel = p.profile.BakeryLevel
		if n.Kind == "bakery_plot" {
			n.BakeryFurniture = append([]BakeryFurniture{}, p.profile.BakeryFurniture...)
			for i := range n.BakeryFurniture {
				if n.BakeryFurniture[i].ID == p.editingBakeryFurnitureID {
					n.BakeryFurniture[i].EditingBy = p.profile.ID
				}
			}
		} else {
			f, available := w.bakeryFurnitureForTarget(p, strings.TrimPrefix(n.Kind, "bakery_"), n.ID)
			n.Available = available
			if available {
				location, _ := w.bakeryPlot(p)
				n.X, n.Z, n.Yaw = location.X+f.X, location.Z+f.Z, f.Rotation*math.Pi/180
				n.FurnitureID = f.ID
				n.ProductionKey = bakeryProductionKey(f)
			}
			if validChoice(n.Kind, "bakery_mixer", "bakery_oven", "bakery_cooling_table") {
				n.Production = cloneStationProduction(p.profile.Production[n.ProductionKey])
				if n.Kind == "bakery_oven" {
					if _, _, ok := w.bakeryStationPosition(p, "cooling_table"); ok {
						n.LinkedTable = bakeryStationID(plot, "cooling_table")
					}
				}
			}
		}
		if n.Kind == "bakery_display" {
			n.Stock = cloneMap(p.profile.BakeryDisplay)
			n.Amount = bakeryStockCount(p.profile.BakeryDisplay)
		}
	}
	return n
}
