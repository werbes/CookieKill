package game

// Equipment is persistent even while its owner is offline. Rebuild only when
// ownership/equipment changes, rather than scanning accounts on every step.
func (w *World) rebuildPropertyColliders() {
	w.propertyColliders = nil
	for id, p := range w.profileViews() {
		if !p.Bakery || !p.Equipment["display"] {
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
	if w.staticBlocked(x, z, r) {
		return true
	}
	for _, c := range w.propertyColliders {
		if circleRect(x, z, r, c.X, c.Z, c.Width, c.Depth) {
			return true
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
