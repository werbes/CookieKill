package game

import (
	"errors"
	"fmt"
	"math"
)

var treeColors = []string{"#387a48", "#698c48", "#32746a", "#8f954a", "#9c6748"}
var cactusColors = []string{"#538d68", "#72994d", "#439381", "#7c9774", "#859644"}

func (w *World) makeWoodTree(n *Node, scale float64, variant int) {
	n.Scale, n.Variant, n.Color = scale, variant, treeColors[variant%len(treeColors)]
	n.Amount = max(3, min(6, 3+int(math.Round((scale-.65)*3))))
	n.ChopTotal, n.ChopRemaining = n.Amount, n.Amount
	n.Label = fmt.Sprintf("Tree · %d wood · %d chops", n.Amount, n.ChopTotal)
	n.respawn = 180
	w.layout.Props = append(w.layout.Props, WorldProp{ID: n.ID, Kind: "tree", X: n.X, Z: n.Z, Width: .9 * scale, Depth: .9 * scale, Height: 7 * scale, Scale: scale, Color: n.Color, Variant: variant})
	w.colliders = append(w.colliders, Collider{ID: n.ID, X: n.X, Z: n.Z, Radius: .45 * scale, Height: 7 * scale, resource: n})
}

func (w *World) chopTree(p *player, n *Node) error {
	if p.island != "" || !n.Available || distance(p.x, p.z, n.X, n.Z) > 5 {
		return errors.New("Move close to a standing tree to chop it.")
	}
	if n.ChopTotal < 3 {
		n.ChopTotal = max(3, min(6, n.Amount))
		n.ChopRemaining = n.ChopTotal
		n.Amount = n.ChopTotal
	}
	if n.ChopRemaining > 1 {
		n.ChopRemaining--
		p.cooldowns["interact"] = w.time + .7
		w.eventActor(fmt.Sprintf("Chop! %d more chops to fell this tree.", n.ChopRemaining), "chop", n.X, n.Z, p.profile.ID)
		return nil
	}
	if !gain(p, "wood", n.Amount) {
		return errors.New("Make room for the wood before felling this tree.")
	}
	n.ChopRemaining, n.Available, n.readyAt = 0, false, w.time+n.respawn
	p.cooldowns["interact"] = w.time + .7
	w.eventActor(fmt.Sprintf("Tree felled: gathered %d wood.", n.Amount), "gather", n.X, n.Z, p.profile.ID)
	return nil
}

func (w *World) seedCave() {
	w.layout.Props = append(w.layout.Props, WorldProp{ID: "sunbaked_cave", Kind: "cave_entrance", Label: "Sunbaked clay caves", X: 120, Z: -98, Width: 12, Depth: 8, Height: 5, Scale: 1})
	for i, p := range []Point{{116, -96}, {124, -96}, {115, -100}, {125, -100}} {
		w.layout.Props = append(w.layout.Props, WorldProp{ID: fmt.Sprintf("cave_plant_%d", i), Kind: "cave_plant", X: p.X, Z: p.Z, Scale: .7 + float64(i%2)*.25, Height: 1.2, Color: cactusColors[i]})
	}
	wall := func(id string, x, z, width, depth, y, height float64) {
		w.layout.Props = append(w.layout.Props, WorldProp{ID: id, Kind: "cave_wall", X: x, Z: z, Width: width, Depth: depth, Y: y, Height: height, Scale: 1, Color: "#a86e52"})
		w.colliders = append(w.colliders, Collider{ID: id, X: x, Z: z, Width: width, Depth: depth, Y: y, Height: height})
	}
	// Follow the descending clay ramp, keeping the chambers beneath the desert
	// instead of raising their walls into a second building on the surface.
	const segments = 8
	for i := 0; i < segments; i++ {
		z := -98 - (float64(i)+.5)*17/segments
		y := CaveFloorY * float64(i+1) / segments
		for side, x := range []float64{116.6, 123.4} {
			wall(fmt.Sprintf("cave_ramp_wall_%d_%d", side, i), x, z, .8, 17.0/segments+.02, y, 5-CaveFloorY/segments)
		}
	}
	for i, dimensions := range [][4]float64{{106.6, -121, .8, 16.8}, {133.4, -121, .8, 16.8}, {120, -129.4, 27.6, .8}, {111.75, -113, 10.5, .8}, {128.25, -113, 10.5, .8}, {120, -124, 3, 8}} {
		wall(fmt.Sprintf("cave_wall_%d", i), dimensions[0], dimensions[1], dimensions[2], dimensions[3], CaveFloorY, 5)
	}
}
