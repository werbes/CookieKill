package game

import "fmt"

// The outcrop is solid on every side except its horizontal, camel-height mouth.
func (w *World) seedCaveRocks() {
	for i, p := range [][6]float64{
		{110.8, -76.25, 9.6, 3.5, 0, 7.8}, {129.2, -76.25, 9.6, 3.5, 0, 7.8},
		{111.4, -85.25, 10.8, 14.5, 0, 7.8}, {128.6, -85.25, 10.8, 14.5, 0, 7.8},
		{120, -76.25, 8.8, 3.5, 3.8, 4.7}, {120, -85.25, 6.4, 14.5, 4.1, 4.4},
		{120, -92, 28, 1, 0, 6.7},
	} {
		id := fmt.Sprintf("cave_outcrop_%d", i)
		w.layout.Props = append(w.layout.Props, WorldProp{ID: id, Kind: "cave_outcrop", X: p[0], Z: p[1], Width: p[2], Depth: p[3], Y: p[4], Height: p[5], Scale: 1})
		w.colliders = append(w.colliders, Collider{ID: id, X: p[0], Z: p[1], Width: p[2], Depth: p[3], Y: p[4], Height: p[5]})
	}
	for i, p := range [][4]float64{
		{104.5, -78, 1.4, 1.7}, {135.5, -79.5, 1.7, 2}, {107, -72.5, 1.1, 1.3}, {132, -72, 1.15, 1.4},
		{103.8, -88, 1.7, 2.1}, {136.2, -90, 1.5, 1.6}, {109, -94.5, 1.25, 1.4}, {130, -95, 1.5, 1.5},
		{105, -119, 1.1, 1.3}, {135, -125, 1.1, 1.3}, {110, -131, 1.2, 1.2}, {128, -132, 1.3, 1.5},
		// Fallen slabs cluster at the foot of the rock without narrowing the
		// broad entrance or the central approach used by returning players.
		{109.9, -74.5, 2.3, 2.8}, {113.2, -72.7, 1.35, 1.6},
		{127.1, -73, 1.4, 1.7}, {131, -74, 2.2, 2.6},
		{104.8, -82, 2.1, 2.7}, {136.3, -84.4, 2, 2.6},
		{107.7, -91.8, 2.1, 2.4}, {132.4, -93, 2, 2.4},
	} {
		id := fmt.Sprintf("cave_surface_rock_%d", i)
		w.layout.Props = append(w.layout.Props, WorldProp{ID: id, Kind: "cave_rock", X: p[0], Z: p[1], Width: p[2] * 2, Depth: p[2] * 2, Height: p[3], Variant: i % 4, Scale: 1})
		w.colliders = append(w.colliders, Collider{ID: id, X: p[0], Z: p[1], Radius: p[2], Height: p[3]})
	}
	// The central dripstone column and larger stalagmites have matching collision.
	for i, p := range [][4]float64{
		{120, -124, 1.15, 9}, {108, -116, .5, 2.3}, {132, -116.5, .55, 2.7}, {108, -126.8, .6, 3.2},
		{114.5, -128, .5, 2.4}, {125, -128, .6, 3}, {132, -128, .5, 2.2},
	} {
		id := fmt.Sprintf("cave_stalagmite_%d", i)
		w.layout.Props = append(w.layout.Props, WorldProp{ID: id, Kind: "cave_stalagmite", X: p[0], Z: p[1], Width: p[2] * 2, Depth: p[2] * 2, Y: CaveFloorY, Height: p[3], Variant: i, Scale: 1})
		w.colliders = append(w.colliders, Collider{ID: id, X: p[0], Z: p[1], Radius: p[2], Y: CaveFloorY, Height: p[3]})
	}
}
