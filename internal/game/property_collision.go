package game

// Furnishings occupy a city plot only while its current baker is connected.
// Rebuild on plot/equipment changes, rather than scanning accounts every step.
func (w *World) rebuildPropertyColliders() {
	w.propertyColliders = nil
	for id, player := range w.players {
		p := player.profile
		if !p.Bakery || p.BakeryPlot == "" {
			continue
		}
		for _, plot := range w.layout.Plots {
			if plot.ID == p.BakeryPlot {
				w.propertyColliders = append(w.propertyColliders, Collider{ID: "display_" + id, X: plot.X + 2.4, Z: plot.Z + 2, Width: 2.8, Depth: .8, Height: 1.65})
				break
			}
		}
	}
}
func (w *World) solidBlocked(x, z, r float64) bool {
	return w.solidBlockedAt(x, 0, z, r)
}

// Underground walls and ceilings end at the desert surface. Walking across
// their roof must not collide with a wall several metres below the player.
func (w *World) solidBlockedAt(x, y, z, r float64) bool {
	if !finite(x) || !finite(y) || !finite(z) || x-r < MinX || x+r > MaxX || z-r < MinZ || z+r > MaxZ {
		return true
	}
	for _, colliders := range [][]Collider{w.colliders, w.propertyColliders} {
		for _, c := range colliders {
			if c.resource != nil && !c.resource.Available || y >= c.Y+c.Height-.001 || y+1.7 <= c.Y+.001 {
				continue
			}
			if c.Radius > 0 {
				if distance(x, z, c.X, c.Z) < r+c.Radius {
					return true
				}
			} else if circleRect(x, z, r, c.X, c.Z, c.Width, c.Depth) {
				return true
			}
		}
	}
	return false
}
func (w *World) solidShotHit(x, y, z, nx, ny, nz float64) (float64, bool) {
	nearest, hit := w.staticShotHit(x, y, z, nx, ny, nz)
	if !hit {
		nearest = 2
	}
	for _, c := range w.propertyColliders {
		if t, ok := slabHit(x, y, z, nx, ny, nz, c.X-c.Width/2, 0, c.Z-c.Depth/2, c.X+c.Width/2, c.Height, c.Z+c.Depth/2); ok && t < nearest {
			nearest = t
			hit = true
		}
	}
	return nearest, hit
}
