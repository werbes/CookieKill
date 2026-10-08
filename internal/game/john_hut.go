package game

import "math"

// The far northeastern cabin has a level, open south doorway. Keep these
// dimensions in sync with web/john-hut-scenery.js.
const (
	JohnHutX   = 222.0
	JohnHutZ   = -239.0
	JohnBedX   = JohnHutX - 1.8
	JohnBedY   = 1.0
	JohnBedZ   = JohnHutZ + .2
	JohnBedYaw = math.Pi
)

func (w *World) seedJohnHut() {
	w.layout.Props = append(w.layout.Props, WorldProp{ID: "john_pork_hut", Kind: "john_pork_hut", Label: "John Pork's hut", X: JohnHutX, Z: JohnHutZ, Width: 8, Depth: 8, Height: 4.5, Scale: 1})
	w.layout.Paths = append(w.layout.Paths, MapRect{ID: "john_hut_approach", X: JohnHutX, Z: JohnHutZ + 7, Width: 2.4, Depth: 7})
	wall := func(id string, x, z, width, depth, y, height float64) {
		w.colliders = append(w.colliders, Collider{ID: "john_hut_" + id, X: JohnHutX + x, Z: JohnHutZ + z, Width: width, Depth: depth, Y: y, Height: height})
	}
	wall("west", -4, 0, .3, 8.3, 0, 3)
	wall("east", 4, 0, .3, 8.3, 0, 3)
	wall("north", 0, -4, 8.3, .3, 0, 3)
	wall("south_left", -2.6, 4, 2.8, .3, 0, 3)
	wall("south_right", 2.6, 4, 2.8, .3, 0, 3)
	wall("door_lintel", 0, 4, 2.4, .3, 2.5, .5)
	wall("ceiling", 0, 0, 8.3, 8.3, 3, .2)
	wall("bed", -1.8, -.9, 1.55, 2.7, 0, .7)
	wall("nightstand", -.25, -1.8, .7, .7, 0, .68)
}
