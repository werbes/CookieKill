package game

import "errors"

const bakeryStationRange = 2.6

// A bakery purchase belongs to the account. Its city plot is occupied only for
// the current connection, so even a full city can welcome returning bakers.
func (w *World) ReleaseBakeryPlot(id string) {
	if p := w.players[id]; p != nil && p.profile.BakeryPlot != "" {
		p.profile.BakeryPlot = ""
		w.rebuildPropertyColliders()
	}
}

func bakeryStationID(plot, station string) string { return plot + "_" + station }

// A newly claimed display can appear where someone was standing while the
// bakery was empty. Only those intersecting that new furnishing move to the
// clear central aisle; everyone else's position and pending input stay intact.
func (w *World) clearNewBakeryDisplay(plotID string) {
	owner := w.plotOwner(plotID)
	for _, plot := range w.layout.Plots {
		if plot.ID != plotID {
			continue
		}
		for _, display := range w.propertyColliders {
			if display.ID != "display_"+owner {
				continue
			}
			for _, p := range w.players {
				if p.island != "" || p.inCave || playerY(p) >= display.Y+display.Height || playerY(p)+1.7 <= display.Y {
					continue
				}
				if !circleRect(p.x, p.z, .42, display.X, display.Z, display.Width, display.Depth) {
					continue
				}
				p.x, p.y, p.z = plot.X, 0, plot.Z
				p.posture = ""
				resetMovement(p)
			}
			return
		}
	}
}

func (w *World) seedBakeryStations() {
	for _, plot := range w.layout.Plots {
		for _, station := range []struct {
			kind, label string
			x, z        float64
		}{
			{"oven", "Bakery oven", -2.6, -2.5},
			{"mixer", "Dough mixer", 2.5, -2.5},
			{"display", "Cookie display", 2.4, 2},
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
	stationID := bakeryStationID(plot, station)
	if target != "" && target != stationID {
		return errors.New("Use the matching station in your own bakery.")
	}
	if !w.near(p, stationID, bakeryStationRange) {
		return errors.New("Move closer to your bakery's " + station + ".")
	}
	return nil
}

// All three base stations are part of the purchased bakery; kitchen equipment
// upgrades improve them without moving their interaction points.
func bakeryNodeView(n Node, owners map[string]*player) Node {
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
	}
	return n
}
