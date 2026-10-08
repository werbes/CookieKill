package game

import (
	"fmt"
	"math"
	"math/rand/v2"
)

const (
	MinX           = -240.0
	MaxX           = 240.0
	MinZ           = -260.0
	MaxZ           = 200.0
	CaveFloorY     = -24.0
	CaveMouthZ     = -75.0
	CaveStairTopZ  = -78.0
	CaveStairCount = 96
	CaveStairRun   = 37.0 / CaveStairCount
	CaveStairRise  = -CaveFloorY / CaveStairCount
)

type Point struct {
	X float64 `json:"x"`
	Z float64 `json:"z"`
}
type MapRect struct {
	ID    string  `json:"id"`
	X     float64 `json:"x"`
	Z     float64 `json:"z"`
	Width float64 `json:"width"`
	Depth float64 `json:"depth"`
}
type BuildZone struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Area  string  `json:"area"`
	X     float64 `json:"x"`
	Z     float64 `json:"z"`
	Width float64 `json:"width"`
	Depth float64 `json:"depth"`
}
type BakeryPlot struct {
	ID    string  `json:"id"`
	X     float64 `json:"x"`
	Z     float64 `json:"z"`
	Width float64 `json:"width"`
	Depth float64 `json:"depth"`
}
type WorldProp struct {
	ID      string  `json:"id"`
	Kind    string  `json:"kind"`
	X       float64 `json:"x"`
	Z       float64 `json:"z"`
	Width   float64 `json:"width"`
	Depth   float64 `json:"depth"`
	Height  float64 `json:"height"`
	Scale   float64 `json:"scale"`
	Label   string  `json:"label,omitempty"`
	Color   string  `json:"color,omitempty"`
	Variant int     `json:"variant,omitempty"`
	Y       float64 `json:"y,omitempty"`
}
type Layout struct {
	Version int          `json:"version"`
	MinX    float64      `json:"minX"`
	MaxX    float64      `json:"maxX"`
	MinZ    float64      `json:"minZ"`
	MaxZ    float64      `json:"maxZ"`
	Coast   []Point      `json:"coast"`
	Props   []WorldProp  `json:"props"`
	Paths   []MapRect    `json:"paths"`
	Zones   []BuildZone  `json:"zones"`
	Plots   []BakeryPlot `json:"plots"`
}

// Collider uses the same physical dimensions as the props sent to the browser.
type Collider struct {
	ID                                 string
	X, Z, Width, Depth, Height, Radius float64
	Y                                  float64
	Rotation                           float64
	resource                           *Node
}

func coastZ(x float64) float64 { return 32 + 11*math.Sin((x+240)*.021) + 5*math.Sin(x*.057) }

// Keep approaches and interiors clear before adding randomized resources and
// scenery; roof colliders alone cannot reserve a walkable ground-level room.
func specialPlaceReserved(x, z, radius float64) bool {
	radius = max(radius, .001)
	return circleRect(x, z, radius, -225.5, 17.5, 29, 29) ||
		circleRect(x, z, radius, TreeHutX, TreeHutZ, 22, 28) ||
		circleRect(x, z, radius, 222, -236.5, 10, 17) ||
		circleRect(x, z, radius, 163.5, 156, 89, 42)
}
func waterAt(x, z float64) bool { return x < 0 && z > coastZ(x) }
func area(x, z float64) string {
	if x < 0 {
		if waterAt(x, z) {
			if z-coastZ(x) < 14 {
				return "shallows"
			}
			return "deep sea"
		}
		if z > coastZ(x)-18 {
			return "beach"
		}
		return "forest"
	}
	if z < 0 {
		return "desert"
	}
	return "city"
}
func inWater(p *player) bool {
	return p.island == "" && waterAt(p.x, p.z) || p.island == CookiePracticeIsland && cookiePracticeWater(p)
}
func groundY(p *player) float64 {
	if p.island != "" {
		return p.y
	}
	if inWater(p) {
		return -.6
	}
	if p.inCave {
		return caveGroundY(p.x, p.z)
	}
	return p.y
}

// Construction now belongs to private home islands; city bakeries remain businesses.
var buildingZones = []BuildZone{}

func caveGroundY(x, z float64) float64 {
	if clubFootprint(x, z) {
		return clubGroundY(x, z)
	}
	if x >= 117 && x <= 123 && z <= CaveMouthZ && z >= -115 {
		return -CaveStairRise * math.Ceil(max(0, (CaveStairTopZ-z)/CaveStairRun-1e-9))
	}
	if x >= 107 && x <= 133 && z <= -113 && z >= -129 {
		return CaveFloorY
	}
	return 0
}

func caveFootprint(x, z float64) bool {
	return clubFootprint(x, z) || math.Abs(x-120) <= caveHalfWidth(z) && z <= CaveMouthZ && z >= -115 ||
		x >= 107 && x <= 133 && z <= -113 && z >= -129
}

func caveHalfWidth(z float64) float64 { return 3 + 1.4*max(0, min(1, (z-CaveStairTopZ)/3)) }

func caveMouth(x, z float64) bool {
	return clubMouth(x, z) || math.Abs(x-120) <= caveHalfWidth(z) && z < CaveMouthZ && z > -80
}

func sameNodeLayer(p *player, n *Node) bool {
	underground := n.InCave || n.Kind == "pink_lollipop" || n.Kind == "blue_lollipop"
	return p.island == n.Island && p.inCave == underground
}

func caveReserved(x, z float64) bool {
	return x > 103 && x < 137 && z > -134 && z < -68 ||
		x > -223 && x < -181 && z > -255 && z < -199 ||
		x > -207 && x < -198 && z >= -204 && z < -186 ||
		x >= -207 && x <= -30 && z > -197 && z < -187
}

func buildZone(x, z float64) string {
	for _, b := range buildingZones {
		if math.Abs(x-b.X) < b.Width/2 && math.Abs(z-b.Z) < b.Depth/2 {
			return b.ID
		}
	}
	return ""
}
func zoneAt(x, z float64) string {
	for _, b := range buildingZones {
		if math.Abs(x-b.X) < b.Width/2 && math.Abs(z-b.Z) < b.Depth/2 {
			return b.Name
		}
	}
	switch area(x, z) {
	case "forest":
		return "Wildwood wilderness"
	case "desert":
		if distance(x, z, 32, -30) < 19 || distance(x, z, 150, -55) < 22 || distance(x, z, 55, -195) < 20 {
			return "Sunbaked barter village"
		}
		return "Sunbaked desert wilderness"
	case "beach":
		return "Peace's winding coast"
	case "shallows":
		return "Turquoise shallows"
	case "deep sea":
		return "The open blue"
	default:
		return "Crumb City streets"
	}
}

func (w *World) StaticLayout() Layout {
	l := w.layout
	l.Coast = append([]Point{}, l.Coast...)
	l.Props = append([]WorldProp{}, l.Props...)
	l.Paths = append([]MapRect{}, l.Paths...)
	l.Zones = append([]BuildZone{}, l.Zones...)
	l.Plots = append([]BakeryPlot{}, l.Plots...)
	return l
}
func (w *World) seedMap() {
	w.layout = Layout{Version: 12, MinX: MinX, MaxX: MaxX, MinZ: MinZ, MaxZ: MaxZ, Zones: append([]BuildZone{}, buildingZones...), Props: []WorldProp{}, Plots: []BakeryPlot{}}
	for x := MinX; x <= 0; x += 3 {
		w.layout.Coast = append(w.layout.Coast, Point{x, coastZ(x)})
	}
	w.layout.Paths = []MapRect{{"forest_lane", -35, -121, 6, 246}, {"crossing", 0, -9, 480, 6}, {"village_lane", 90, -20, 185, 5}, {"main_street", 122, 34, 232, 9}, {"baker_street", 120, 84, 232, 8}, {"market_street", 120, 126, 232, 8}, {"city_lane", 8, 95, 7, 198}, {"east_lane", 222, 96, 7, 198}, {"city_cross", 110, 90, 7, 195}}
	addNode := func(id, kind, label string, x, z float64, amount int, respawn float64) {
		w.nodes = append(w.nodes, &Node{ID: id, Kind: kind, Label: label, X: x, Z: z, Available: true, Amount: amount, respawn: respawn})
		if kind == "peace" {
			w.colliders = append(w.colliders, Collider{ID: id + "_counter", X: x, Z: z - 1.8, Width: 3.5, Depth: 1.1, Height: 1})
		}
	}
	building := func(id, kind, label string, x, z, width, depth, height float64) {
		p := WorldProp{ID: id, Kind: kind, Label: label, X: x, Z: z, Width: width, Depth: depth, Height: height, Scale: 1}
		w.layout.Props = append(w.layout.Props, p)
		w.buildingColliders(p)
	}
	addNode("peace", "peace", "Peace's beach stand", -30, 8, 0, 0)
	addNode("desert_trader", "desert_trader", "Saffron's barter kitchen", 32, -30, 0, 0)
	addNode("desert_oven", "fire", "Village clay oven", 29, -30, 0, 0)
	addNode("desert_trader_2", "desert_trader", "Juniper's barter house", 150, -55, 0, 0)
	addNode("desert_trader_3", "desert_trader", "Clove's barter house", 55, -195, 0, 0)
	addNode("pink_lollipop", "pink_lollipop", "Pink lollipop · your first land", 111, -124, 0, 0)
	addNode("blue_lollipop", "blue_lollipop", "Blue lollipop · visit friends", 129, -124, 0, 0)
	addNode("abu_fanous", "abu_fanous", "Abu Fanous · camel keeper", 226, -126, 0, 0)
	w.seedCave()
	w.seedClub()
	w.seedRace()
	w.seedCamelTraining()
	w.seedTreeHut()
	w.seedJohnHut()
	building("saffron_house", "desert_building", "SAFFRON • BARTER", 32, -30, 8, 7, 4)
	building("juniper_house", "desert_building", "JUNIPER • BARTER", 150, -55, 9, 8, 4)
	building("clove_house", "desert_building", "CLOVE • BARTER", 55, -195, 9, 8, 4)
	for i, p := range []Point{{51, -48}, {15, -52}, {146, -91}, {175, -68}, {77, -210}, {36, -220}} {
		building(fmt.Sprintf("village_house_%d", i), "desert_building", "", p.X, p.Z, 8, 7, 4)
	}
	addNode("kitchen_shop", "kitchen_shop", "The Kitchen Workshop", 80, 22, 0, 0)
	building("kitchen_store", "kitchen_shop", "KITCHEN WORKSHOP", 80, 22, 12, 9, 5)
	addNode("ingredient_shop", "ingredient_shop", "Baker's Pantry", 145, 22, 0, 0)
	building("ingredient_store", "ingredient_shop", "BAKER'S PANTRY", 145, 22, 12, 9, 5)
	building("paint_store", "paint_shop", "PIGMENT & COLOUR", 178, 22, 12, 9, 5)
	addNode("paint_shop", "paint_shop", "Pigment & Colour", 178, 22, 0, 0)
	building("outfit_store", "outfit_shop", "THREAD & THIMBLE", 210, 22, 11, 9, 5)
	addNode("outfit_shop", "outfit_shop", "Thread & Thimble", 210, 22, 0, 0)
	for row := 0; row < 2; row++ {
		for col := 0; col < 6; col++ {
			id := fmt.Sprintf("bakery_plot_%d", row*6+col)
			x := 24 + float64(col)*34
			z := 72 + float64(row)*42
			// Leave the north/south city cross street clear by routing around its block.
			if col >= 3 {
				x += 5
			}
			w.layout.Plots = append(w.layout.Plots, BakeryPlot{id, x, z, 20, 16})
			addNode(id, "bakery_plot", "Bakery plot • 250 coins", x, z, 0, 0)
			building(id, "bakery", "BAKERY • AVAILABLE", x, z, 20, 16, 5.8)
		}
	}
	addNode("dummy", "dummy", "Practice dummy", -35, -47, 0, 0)
	w.seedTown()
	labels := map[string]string{"wood": "Tree · chop for wood", "stick": "Dry sticks", "stone": "Loose stones", "berry": "Wild berries", "nut": "Forest nuts", "cactus": "Cactus fruit", "sea_salt": "Sea salt", "trash": "Beach litter • kilograms", "shell": "Washed-up seashell", "chest": "Forgotten chest"}
	starters := []struct {
		kind  string
		x, z  float64
		count int
	}{{"wood", -39, -36, 2}, {"stick", -40, -31, 2}, {"stone", -31, -31, 3}, {"chest", -29, -39, 1}, {"berry", -41, -40, 3}, {"nut", -31, -44, 3}, {"chest", -43, -29, 1}}
	for i, n := range starters {
		addNode(fmt.Sprintf("starter_%d", i), n.kind, labels[n.kind], n.x, n.z, n.count, 25)
		if n.kind == "wood" {
			w.makeWoodTree(w.nodes[len(w.nodes)-1], .8, 0)
		}
	}
	rng := rand.New(rand.NewPCG(2323, 7819))
	for i := 0; i < 130; i++ {
		kind := []string{"wood", "stick", "stone", "chest", "berry", "nut", "chest"}[i%7]
		x := -12 - rng.Float64()*220
		z := -15 - rng.Float64()*233
		if w.staticBlocked(x, z, 2) || buildZone(x, z) != "" || caveReserved(x, z) || specialPlaceReserved(x, z, 2) {
			continue
		}
		addNode(fmt.Sprintf("forest_%d", i), kind, labels[kind], x, z, 2, 35)
		if kind == "wood" {
			w.makeWoodTree(w.nodes[len(w.nodes)-1], .65+rng.Float64(), rng.IntN(5))
		}
	}
	for i := 0; i < 80; i++ {
		kind := []string{"cactus", "chest", "stone", "chest"}[i%4]
		x := 12 + rng.Float64()*218
		z := -16 - rng.Float64()*232
		if w.staticBlocked(x, z, 3) || buildZone(x, z) != "" || caveReserved(x, z) || specialPlaceReserved(x, z, 3) || distance(x, z, 226, -126) < 8 {
			continue
		}
		addNode(fmt.Sprintf("desert_%d", i), kind, labels[kind], x, z, 2, 40)
		if kind == "cactus" {
			n := w.nodes[len(w.nodes)-1]
			n.Scale, n.Variant, n.Color = .65+rng.Float64()*.9, rng.IntN(4), cactusColors[rng.IntN(len(cactusColors))]
			w.makeCactus(n, n.Scale, n.Variant)
		}
	}
	for i := 0; i < 75; i++ {
		kind := []string{"trash", "trash", "shell", "stone", "sea_salt", "chest", "chest"}[i%7]
		x := -8 - rng.Float64()*225
		z := coastZ(x) - 3 - rng.Float64()*12
		if specialPlaceReserved(x, z, 2) {
			continue
		}
		amount := 1
		if kind == "trash" {
			amount = 2
		}
		addNode(fmt.Sprintf("beach_%d", i), kind, labels[kind], x, z, amount, 35)
	}
	// Small cactus gardens grow outside the village walls. These use the same
	// resources as wilderness cacti, so every visible cactus can be chopped.
	for _, house := range append([]WorldProp{}, w.layout.Props...) {
		if house.Kind != "desert_building" {
			continue
		}
		for i, side := range []float64{-1, 1} {
			x, z := house.X+side*(house.Width/2+2), house.Z-1
			if w.staticBlocked(x, z, 1) {
				continue
			}
			n := &Node{ID: fmt.Sprintf("%s_cactus_%d", house.ID, i), Kind: "cactus", X: x, Z: z, Available: true}
			w.nodes = append(w.nodes, n)
			w.makeCactus(n, .65+float64(i)*.15, i)
		}
	}
	for i, point := range []Point{{112, -120}, {128, -116}, {125, -128}} {
		n := &Node{ID: fmt.Sprintf("cave_roof_cactus_%d", i), Kind: "cactus", X: point.X, Z: point.Z, Available: true}
		w.nodes = append(w.nodes, n)
		w.makeCactus(n, []float64{.7, .85, .65}[i], i)
	}
	w.seedDecor(rng)
}

func (w *World) buildingColliders(p WorldProp) {
	add := func(id string, x, z, width, depth float64) {
		w.colliders = append(w.colliders, Collider{ID: p.ID + id, X: x, Z: z, Width: width, Depth: depth, Height: p.Height})
	}
	add("_back", p.X, p.Z-p.Depth/2, p.Width, .24)
	add("_left", p.X-p.Width/2, p.Z, .24, p.Depth)
	add("_right", p.X+p.Width/2, p.Z, .24, p.Depth)
	wing := (p.Width - 3.2) / 2
	for _, side := range []float64{-1, 1} {
		add(fmt.Sprintf("_front%v", side), p.X+side*(1.6+wing/2), p.Z+p.Depth/2, wing, .24)
	}
	furniture := func(id string, x, z, width, depth, height float64) {
		w.colliders = append(w.colliders, Collider{ID: p.ID + id, X: p.X + x, Z: p.Z + z, Width: width, Depth: depth, Height: height})
	}
	switch p.Kind {
	case "bakery":
		for _, side := range []float64{-1, 1} {
			add(fmt.Sprintf("_partition%v", side), p.X+side*(1.6+wing/2), p.Z-1, wing, .24)
		}
		for _, z := range []float64{p.Z - 1, p.Z + p.Depth/2} {
			w.colliders = append(w.colliders, Collider{ID: fmt.Sprintf("%s_lintel_%v", p.ID, z), X: p.X, Z: z, Width: 3.2, Depth: .24, Y: 3.5, Height: p.Height - 3.5})
		}
		w.colliders = append(w.colliders, Collider{ID: p.ID + "_ceiling", X: p.X, Z: p.Z, Width: p.Width, Depth: p.Depth, Y: p.Height, Height: .36})
	case "gym":
		furniture("_treadmill_left", -3, -2, 1.2, 2.7, 1.4)
		furniture("_treadmill_right", 3, -2, 1.2, 2.7, 1.4)
		furniture("_bench", 0, -2.8, 4, .85, .65)
	case "desert_building":
		for _, side := range []float64{-1, 1} {
			w.colliders = append(w.colliders, Collider{ID: fmt.Sprintf("%s_awning_%v", p.ID, side), X: p.X + side*1.9, Z: p.Z + p.Depth/2 + 2.2, Radius: .07, Height: 3})
		}
		w.colliders = append(w.colliders, Collider{ID: p.ID + "_pot", X: p.X - 3, Z: p.Z + p.Depth/2 + .5, Radius: .5, Height: 1.2})
	case "kitchen_shop", "paint_shop", "outfit_shop", "ingredient_shop":
		for _, side := range []float64{-1, 1} {
			furniture(fmt.Sprintf("_window_display_%v", side), side*(p.Width/2-1.6), p.Depth/2+.5, 2, .8, 1.1)
		}
	}
}
func circleRect(x, z, r, cx, cz, width, depth float64) bool {
	dx := math.Max(math.Abs(x-cx)-width/2, 0)
	dz := math.Max(math.Abs(z-cz)-depth/2, 0)
	return dx*dx+dz*dz < r*r
}
func (w *World) staticBlocked(x, z, r float64) bool {
	if !finite(x) || !finite(z) || x-r < MinX || x+r > MaxX || z-r < MinZ || z+r > MaxZ {
		return true
	}
	for _, c := range w.colliders {
		if c.resource != nil && !c.resource.Available {
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
	return false
}
func (w *World) safeSpace(x, z, r float64) bool {
	zone := buildZone(x, z)
	if zone == "" || w.staticBlocked(x, z, r) {
		return false
	}
	for _, b := range buildingZones {
		if b.ID == zone && (math.Abs(x-b.X)+r > b.Width/2 || math.Abs(z-b.Z)+r > b.Depth/2) {
			return false
		}
	}
	for _, p := range w.layout.Paths {
		if circleRect(x, z, r, p.X, p.Z, p.Width, p.Depth) {
			return false
		}
	}
	return true
}
func (w *World) seedDecor(rng *rand.Rand) {
	clear := func(x, z, r float64) bool {
		if distance(x, z, -35, -35) < 15 || buildZone(x, z) != "" || w.staticBlocked(x, z, r+1.3) || caveReserved(x, z) || specialPlaceReserved(x, z, r) || distance(x, z, 226, -126) < 8 {
			return false
		}
		for _, b := range buildingZones {
			if math.Abs(x-b.X) < b.Width/2+2 && math.Abs(z-b.Z) < b.Depth/2+2 {
				return false
			}
		}
		for _, p := range w.layout.Paths {
			if circleRect(x, z, r+1, p.X, p.Z, p.Width, p.Depth) {
				return false
			}
		}
		for _, n := range w.nodes {
			if distance(x, z, n.X, n.Z) < r+2.2 {
				return false
			}
		}
		for _, p := range w.layout.Props {
			if p.Kind == "desert_building" || p.Kind == "gym" || p.Kind == "bakery" || p.Kind == "kitchen_shop" || p.Kind == "paint_shop" || p.Kind == "outfit_shop" || p.Kind == "ingredient_shop" {
				if circleRect(x, z, r+3, p.X, p.Z, p.Width, p.Depth) {
					return false
				}
			}
		}
		return true
	}
	add := func(kind string, x, z, scale float64) {
		r, h := .45*scale, 7*scale
		switch kind {
		case "bush":
			r = .85 * scale
			h = 1.3 * scale
		case "rock", "beach_rock":
			r = scale
			h = 1.3 * scale
		case "cactus":
			r = .85 * scale
			h = 3.3 * scale
		case "palm":
			r = .38 * scale
			h = 5.5 * scale
		}
		if !clear(x, z, r) {
			return
		}
		id := fmt.Sprintf("prop_%d", len(w.layout.Props))
		if kind == "tree" || kind == "pine" {
			n := &Node{ID: id, Kind: "wood", Label: "Tree · chop for wood", X: x, Z: z, Available: true}
			w.nodes = append(w.nodes, n)
			w.makeWoodTree(n, scale, rng.IntN(5))
			w.layout.Props[len(w.layout.Props)-1].Kind = kind
			return
		}
		if kind == "cactus" {
			n := &Node{ID: id, Kind: kind, X: x, Z: z, Available: true}
			w.nodes = append(w.nodes, n)
			w.makeCactus(n, scale, rng.IntN(5))
			return
		}
		prop := WorldProp{ID: id, Kind: kind, X: x, Z: z, Width: 2 * r, Depth: 2 * r, Height: h, Scale: scale, Variant: rng.IntN(5)}
		if kind == "cactus" {
			prop.Color = cactusColors[rng.IntN(len(cactusColors))]
		}
		w.layout.Props = append(w.layout.Props, prop)
		w.colliders = append(w.colliders, Collider{ID: id, X: x, Z: z, Radius: r, Height: h})
	}
	for i := 0; i < 470; i++ {
		x := -7 - rng.Float64()*224
		z := -12 - rng.Float64()*239
		if i < 100 {
			x = -10 - rng.Float64()*83
			z = -10 - rng.Float64()*85
		}
		kind := "tree"
		if i%4 == 0 {
			kind = "pine"
		}
		add(kind, x, z, .65+rng.Float64())
	}
	for i := 0; i < 120; i++ {
		kind := "bush"
		if i%4 == 0 {
			kind = "rock"
		}
		add(kind, -8-rng.Float64()*222, -10-rng.Float64()*240, .55+rng.Float64()*.5)
	}
	for i := 0; i < 95; i++ {
		kind := "cactus"
		if i%4 == 0 {
			kind = "rock"
		}
		add(kind, 9+rng.Float64()*220, -12-rng.Float64()*237, .55+rng.Float64()*.8)
	}
	for i := 0; i < 66; i++ {
		x := -9 - rng.Float64()*219
		kind := "beach_rock"
		scale := .23 + rng.Float64()*.27
		if i%5 == 0 {
			kind = "palm"
			scale = .75 + rng.Float64()*.3
		}
		add(kind, x, coastZ(x)-4-rng.Float64()*11, scale)
	}
	// Planted courtyards and street-side greenery soften the city blocks.
	for i := 0; i < 100; i++ {
		x, z := 14+rng.Float64()*216, 40+rng.Float64()*150
		kind := "bush"
		if i%4 == 0 {
			kind = "palm"
		}
		add(kind, x, z, .55+rng.Float64()*.4)
	}
	// Trees flank the compact settlement clearings without filling them.
	for _, b := range buildingZones {
		for i := 0; i < 8; i++ {
			x := b.X - b.Width/2 - 4 + float64(i)*(b.Width+8)/7
			kind := "tree"
			if b.Area == "desert" {
				kind = "cactus"
			}
			add(kind, x, b.Z-b.Depth/2-4, .85)
		}
	}
}

// slabHit returns the first contact along a segment, including wall occlusion.
func slabHit(x, y, z, nx, ny, nz, minX, minY, minZ, maxX, maxY, maxZ float64) (float64, bool) {
	lo, hi := 0.0, 1.0
	for _, axis := range [][4]float64{{x, nx - x, minX, maxX}, {y, ny - y, minY, maxY}, {z, nz - z, minZ, maxZ}} {
		if math.Abs(axis[1]) < 1e-9 {
			if axis[0] < axis[2] || axis[0] > axis[3] {
				return 0, false
			}
			continue
		}
		a, b := (axis[2]-axis[0])/axis[1], (axis[3]-axis[0])/axis[1]
		if a > b {
			a, b = b, a
		}
		lo = math.Max(lo, a)
		hi = math.Min(hi, b)
		if lo > hi {
			return 0, false
		}
	}
	return lo, true
}
func (w *World) staticShotHit(x, y, z, nx, ny, nz float64) (float64, bool) {
	nearest := 2.0
	for _, c := range w.colliders {
		if c.resource != nil && !c.resource.Available {
			continue
		}
		t, hit := colliderSegmentHit(c, x, y, z, nx, ny, nz)
		if hit && t < nearest {
			nearest = t
		}
	}
	return nearest, nearest <= 1
}
