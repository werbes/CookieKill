package game

import "math"

// Furnishings occupy a city plot only while its current baker is connected.
// Rebuild on plot/equipment changes, rather than scanning accounts every step.
func (w *World) rebuildPropertyColliders() {
	w.propertyColliders = nil
	for _, player := range w.players {
		w.propertyColliders = append(w.propertyColliders, w.bakeryFurnitureColliders(player)...)
	}
}
func colliderLocal(c Collider, x, z float64) (float64, float64) {
	a := c.Rotation * math.Pi / 180
	dx, dz := x-c.X, z-c.Z
	return dx*math.Cos(a) - dz*math.Sin(a), dx*math.Sin(a) + dz*math.Cos(a)
}
func colliderOverlaps(c Collider, x, z, r float64) bool {
	if c.Radius > 0 {
		return distance(x, z, c.X, c.Z) < r+c.Radius
	}
	lx, lz := colliderLocal(c, x, z)
	return circleRect(lx, lz, r, 0, 0, c.Width, c.Depth)
}
func colliderSegmentHit(c Collider, x, y, z, nx, ny, nz float64) (float64, bool) {
	lx, lz := colliderLocal(c, x, z)
	lnx, lnz := colliderLocal(c, nx, nz)
	width, depth := c.Width, c.Depth
	if c.Radius > 0 {
		width, depth = c.Radius*2, c.Radius*2
	}
	return slabHit(lx, y, lz, lnx, ny, lnz, -width/2, c.Y, -depth/2, width/2, c.Y+c.Height, depth/2)
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
			if colliderOverlaps(c, x, z, r) {
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
		if t, ok := colliderSegmentHit(c, x, y, z, nx, ny, nz); ok && t < nearest {
			nearest = t
			hit = true
		}
	}
	return nearest, hit
}
