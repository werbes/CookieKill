package game

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

type BakeryFurniture struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	X         float64 `json:"x"`
	Z         float64 `json:"z"`
	Rotation  float64 `json:"rotation"`
	Color     string  `json:"color,omitempty"`
	EditingBy string  `json:"editingBy,omitempty"`
}
type BakeryFurnitureRecipe struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Width  float64 `json:"width"`
	Depth  float64 `json:"depth"`
	Height float64 `json:"height"`
	Y      float64 `json:"y"`
	Cost   int     `json:"cost"`
	Room   string  `json:"room"`
	Core   bool    `json:"core"`
	Solid  bool    `json:"solid"`
}

var bakeryFurnitureCatalog = []BakeryFurnitureRecipe{
	{"oven", "Bakery oven", 2.2, 1.4, 1.8, 0, 120, "kitchen", true, true},
	{"mixer", "Dough mixer", 1.6, 1.6, 1.8, 0, 90, "kitchen", true, true},
	{"cooling_table", "Cooling table", 2.4, 1.2, .9, 0, 40, "kitchen", true, true},
	{"display", "Cookie display", 2.8, .8, 1.65, 0, 100, "cafe", true, true},
	{"chair", "Cafe chair", .9, .9, 1.4, 0, 12, "cafe", false, true},
	{"table", "Cafe table", 1.8, 1.4, .85, 0, 30, "cafe", false, true},
	{"counter", "Kitchen counter", 3, .9, 1.1, 0, 45, "any", false, true},
	{"bread_rack", "Bread rack", 2, .7, 2.4, 0, 40, "cafe", false, true},
	{"plant", "Potted plant", .8, .8, 1.5, 0, 15, "any", false, true},
	{"pendant", "Pendant light", .8, .8, 1.1, 4.4, 25, "any", false, false},
	{"rug", "Cafe rug", 3, 2, .04, 0, 15, "any", false, false},
	{"menu_board", "Menu board", 1.3, .3, 1.8, 0, 20, "cafe", false, true},
}

func bakeryFurnitureRecipeFor(kind string) (BakeryFurnitureRecipe, bool) {
	for _, r := range bakeryFurnitureCatalog {
		if r.ID == kind {
			return r, true
		}
	}
	return BakeryFurnitureRecipe{}, false
}
func defaultBakeryFurniture() []BakeryFurniture {
	return []BakeryFurniture{{ID: "oven", Kind: "oven", X: -5, Z: -5.5}, {ID: "mixer", Kind: "mixer", X: 5, Z: -5.5}, {ID: "display", Kind: "display", X: 5, Z: 3}, {ID: "table_1", Kind: "table", X: -5, Z: 3}, {ID: "table_2", Kind: "table", X: -5, Z: 6}, {ID: "chair_1", Kind: "chair", X: -6.5, Z: 3, Rotation: 270}, {ID: "chair_2", Kind: "chair", X: -3.5, Z: 3, Rotation: 90}, {ID: "chair_3", Kind: "chair", X: -6.5, Z: 6, Rotation: 270}, {ID: "chair_4", Kind: "chair", X: -3.5, Z: 6, Rotation: 90}}
}

// Migration happens before an account claims a city plot. Check its local
// layout with the same navigation used by live customers and staff.
func bakeryLayoutReachable(objects []BakeryFurniture) bool {
	w := World{layout: Layout{Plots: []BakeryPlot{{ID: "migration"}}}}
	p := player{profile: Profile{Bakery: true, BakeryPlot: "migration", BakeryFurniture: objects}}
	entry := Point{0, 9.5}
	if _, ok := w.bakeryRoute(&p, entry, Point{0, -3}); !ok {
		return false
	}
	for _, kind := range []string{"oven", "mixer", "display", "cooling_table"} {
		for _, back := range []bool{false, true} {
			if back && kind != "display" {
				continue
			}
			if point, present := w.bakeryStationServicePoint(&p, kind, back); present {
				if _, ok := w.bakeryRoute(&p, entry, point); !ok {
					return false
				}
			}
		}
	}
	return true
}
func normalizeBakeryFurniture(p *Profile) {
	if p.Bakery && p.BakeryFurnitureVersion < 1 {
		p.BakeryFurniture = defaultBakeryFurniture()
		for i := range p.BakeryFurniture {
			if color := p.Paint[p.BakeryFurniture[i].Kind]; validColor(color) {
				p.BakeryFurniture[i].Color = color
			}
		}
		p.BakeryFurnitureVersion = 1
	}
	if p.Bakery && p.BakeryFurnitureVersion < 2 {
		found := false
		for _, f := range p.BakeryFurniture {
			if f.Kind == "cooling_table" {
				found = true
				break
			}
		}
		if !found {
			positions := []Point{{-1.8, -5.5}}
			for z := -6.; z <= -3; z += 1.5 {
				for x := -8.; x <= 8; x += 2 {
					positions = append(positions, Point{x, z})
				}
			}
			for _, point := range positions {
				candidate := BakeryFurniture{ID: "cooling_table", Kind: "cooling_table", X: point.X, Z: point.Z}
				recipe, _ := bakeryFurnitureRecipeFor(candidate.Kind)
				clear := bakeryFurnitureInRoom(candidate, recipe) && !bakeryFurnitureBlocks(candidate, 0, -3, .35)
				for _, existing := range p.BakeryFurniture {
					if existing.ID == candidate.ID {
						candidate.ID = "starter_cooling_table"
					}
					if bakeryFurnitureOverlaps(candidate, existing) {
						clear = false
						break
					}
					if r, _ := bakeryFurnitureRecipeFor(existing.Kind); r.Core {
						a := existing.Rotation * math.Pi / 180
						if bakeryFurnitureBlocks(candidate, existing.X+math.Sin(a)*1.8, existing.Z+math.Cos(a)*1.8, .35) {
							clear = false
						}
					}
				}
				if clear && len(p.BakeryFurniture) < 120 && bakeryLayoutReachable(append(append([]BakeryFurniture{}, p.BakeryFurniture...), candidate)) {
					p.BakeryFurniture = append(p.BakeryFurniture, candidate)
					found = true
					break
				}
			}
			if !found {
				if p.Recovery == nil {
					p.Recovery = map[string]int{}
				}
				p.Recovery["bakery_cooling_table"]++
			}
		}
		p.BakeryFurnitureVersion = 2
	}
	kept := make([]BakeryFurniture, 0, len(p.BakeryFurniture))
	ids := map[string]bool{}
	core := map[string]bool{}
	for _, f := range p.BakeryFurniture {
		r, ok := bakeryFurnitureRecipeFor(f.Kind)
		if !ok || f.ID == "" || len(f.ID) > 100 || ids[f.ID] || r.Core && core[f.Kind] || !finite(f.Rotation) || !finite(f.X) || !finite(f.Z) || math.Abs(f.X) > 10 || math.Abs(f.Z) > 8 || len(kept) >= 120 {
			continue
		}
		f.Rotation = math.Mod(math.Round(f.Rotation/45)*45+360, 360)
		if f.Rotation < 0 {
			f.Rotation += 360
		}
		if f.Color != "" && !validColor(f.Color) {
			f.Color = ""
		}
		f.EditingBy = ""
		if !bakeryFurnitureInRoom(f, r) {
			continue
		}
		ids[f.ID] = true
		core[f.Kind] = r.Core
		kept = append(kept, f)
	}
	p.BakeryFurniture = kept
}
func packedBakeryItem(item string) (string, string, bool) {
	if !strings.HasPrefix(item, "bakery_") {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(item, "bakery_"), ":")
	if len(parts) > 2 {
		return "", "", false
	}
	if _, ok := bakeryFurnitureRecipeFor(parts[0]); !ok {
		return "", "", false
	}
	color := ""
	if len(parts) == 2 {
		color = parts[1]
		if !validColor(color) {
			return "", "", false
		}
	}
	return parts[0], color, true
}
func packedBakeryName(f BakeryFurniture) string {
	item := "bakery_" + f.Kind
	if f.Color != "" {
		item += ":" + f.Color
	}
	return item
}
func bakeryFurnitureLocal(f BakeryFurniture, x, z float64) (float64, float64) {
	a := f.Rotation * math.Pi / 180
	dx, dz := x-f.X, z-f.Z
	return dx*math.Cos(a) - dz*math.Sin(a), dx*math.Sin(a) + dz*math.Cos(a)
}
func bakeryFurnitureBlocks(f BakeryFurniture, x, z, radius float64) bool {
	r, ok := bakeryFurnitureRecipeFor(f.Kind)
	if !ok || !r.Solid {
		return false
	}
	lx, lz := bakeryFurnitureLocal(f, x, z)
	return circleRect(lx, lz, radius, 0, 0, r.Width, r.Depth)
}
func bakeryFurnitureExtents(f BakeryFurniture, r BakeryFurnitureRecipe) (float64, float64) {
	a := f.Rotation * math.Pi / 180
	return (math.Abs(math.Cos(a))*r.Width + math.Abs(math.Sin(a))*r.Depth) / 2, (math.Abs(math.Sin(a))*r.Width + math.Abs(math.Cos(a))*r.Depth) / 2
}
func bakeryFurnitureInRoom(f BakeryFurniture, r BakeryFurnitureRecipe) bool {
	x, z := bakeryFurnitureExtents(f, r)
	if math.Abs(f.X)+x > 9.5 || math.Abs(f.Z)+z > 7.5 || r.Room == "kitchen" && f.Z+z > -1.35 || r.Room == "cafe" && f.Z-z < -.65 {
		return false
	}
	// Solid furnishings belong wholly to one side of the partition. Its
	// doorway must also stay clear, so no solid object can straddle this band.
	return !r.Solid || f.Z+z <= -1.12 || f.Z-z >= -.88
}
func bakeryFurnitureOverlaps(a, b BakeryFurniture) bool {
	ar, _ := bakeryFurnitureRecipeFor(a.Kind)
	br, _ := bakeryFurnitureRecipeFor(b.Kind)
	if !ar.Solid || !br.Solid {
		return false
	}
	for _, rotation := range []float64{a.Rotation, b.Rotation} {
		for _, turn := range []float64{0, 90} {
			angle := (rotation + turn) * math.Pi / 180
			ux, uz := math.Cos(angle), -math.Sin(angle)
			project := func(f BakeryFurniture, r BakeryFurnitureRecipe) float64 {
				radians := f.Rotation * math.Pi / 180
				return math.Abs(ux*math.Cos(radians)-uz*math.Sin(radians))*r.Width/2 + math.Abs(ux*math.Sin(radians)+uz*math.Cos(radians))*r.Depth/2
			}
			if math.Abs((a.X-b.X)*ux+(a.Z-b.Z)*uz) >= project(a, ar)+project(b, br)+.05 {
				return false
			}
		}
	}
	return true
}

func (w *World) bakeryStationPosition(p *player, kind string) (Point, float64, bool) {
	plot, ok := w.bakeryPlot(p)
	if !ok {
		return Point{}, 0, false
	}
	for _, f := range p.profile.BakeryFurniture {
		if f.Kind == kind {
			return Point{plot.X + f.X, plot.Z + f.Z}, f.Rotation, true
		}
	}
	return Point{}, 0, false
}
func (w *World) bakeryStationServicePoint(p *player, kind string, back bool) (Point, bool) {
	point, rotation, ok := w.bakeryStationPosition(p, kind)
	if !ok {
		return Point{}, false
	}
	d := 1.8
	if back {
		d = -d
	}
	a := rotation * math.Pi / 180
	return Point{point.X + math.Sin(a)*d, point.Z + math.Cos(a)*d}, true
}

type bakeryObstacle struct {
	x, z, halfWidth, halfDepth, cos, sin float64
}

type bakeryNavigation struct {
	origin Point
	solids []bakeryObstacle
}

// A route can test thousands of points. Resolve recipes, room origin and
// rotations once; decorative rugs and lamps never enter the collision loop.
func (w *World) bakeryNavigation(p *player) (bakeryNavigation, bool) {
	plot, ok := w.bakeryPlot(p)
	if !ok {
		return bakeryNavigation{}, false
	}
	nav := bakeryNavigation{origin: Point{plot.X, plot.Z}}
	for _, f := range p.profile.BakeryFurniture {
		r, valid := bakeryFurnitureRecipeFor(f.Kind)
		if !valid || !r.Solid {
			continue
		}
		a := f.Rotation * math.Pi / 180
		nav.solids = append(nav.solids, bakeryObstacle{f.X, f.Z, r.Width / 2, r.Depth / 2, math.Cos(a), math.Sin(a)})
	}
	return nav, true
}

func (nav bakeryNavigation) walkable(point Point, radius float64) bool {
	x, z := point.X-nav.origin.X, point.Z-nav.origin.Z
	if math.Abs(x)+radius > 9.88 || z-radius < -7.88 || z > 10.1 {
		return false
	}
	if z+radius > 7.88 && math.Abs(x)+radius > 1.6 {
		return false
	}
	if math.Abs(z+1) < radius+.12 && math.Abs(x)+radius > 1.6 {
		return false
	}
	for _, solid := range nav.solids {
		dx, dz := x-solid.x, z-solid.z
		lx := max(0, math.Abs(dx*solid.cos-dz*solid.sin)-solid.halfWidth)
		lz := max(0, math.Abs(dx*solid.sin+dz*solid.cos)-solid.halfDepth)
		if lx*lx+lz*lz < radius*radius {
			return false
		}
	}
	return true
}
func (nav bakeryNavigation) segment(a, b Point, radius float64) bool {
	steps := max(1, int(math.Ceil(distance(a.X, a.Z, b.X, b.Z)/.15)))
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		if !nav.walkable(Point{a.X + (b.X-a.X)*t, a.Z + (b.Z-a.Z)*t}, radius) {
			return false
		}
	}
	return true
}

func (w *World) bakeryWalkable(p *player, point Point, radius float64) bool {
	nav, ok := w.bakeryNavigation(p)
	return ok && nav.walkable(point, radius)
}

func (w *World) bakeryWalkSegment(p *player, a, b Point, radius float64) bool {
	nav, ok := w.bakeryNavigation(p)
	return ok && nav.segment(a, b, radius)
}

// Small deterministic grid searches keep both rooms and station service points
// reachable. Routes use absolute points and omit the start, retaining the exact
// destination; the fixed doorway and rotated furniture use the same clearance.
func (w *World) bakeryRoute(p *player, from, to Point) ([]Point, bool) {
	const radius = .35
	nav, ok := w.bakeryNavigation(p)
	if !ok || !nav.walkable(from, radius) || !nav.walkable(to, radius) {
		return nil, false
	}
	if nav.segment(from, to, radius) {
		return []Point{to}, true
	}
	const cols = 37
	const rows = 35
	const total = cols * rows
	point := func(i int) Point {
		return Point{nav.origin.X - 9 + float64(i%cols)*.5, nav.origin.Z - 7 + float64(i/cols)*.5}
	}
	start, goal := -1, -1
	ds, dg := math.Inf(1), math.Inf(1)
	nearest := func(p Point) int {
		x := min(cols-1, max(0, int(math.Round((p.X-nav.origin.X+9)*2))))
		z := min(rows-1, max(0, int(math.Round((p.Z-nav.origin.Z+7)*2))))
		return z*cols + x
	}
	// Seed the closest grid cell first so the scan does not repeatedly test
	// long segments from successively nearer cells across the entire room.
	if i := nearest(from); nav.segment(from, point(i), radius) {
		start, ds = i, distance(from.X, from.Z, point(i).X, point(i).Z)
	}
	if i := nearest(to); nav.segment(point(i), to, radius) {
		goal, dg = i, distance(to.X, to.Z, point(i).X, point(i).Z)
	}
	var walk [total]bool
	for i := 0; i < total; i++ {
		q := point(i)
		walk[i] = nav.walkable(q, radius)
		if !walk[i] {
			continue
		}
		a := distance(q.X, q.Z, from.X, from.Z)
		b := distance(q.X, q.Z, to.X, to.Z)
		if a < ds && nav.segment(from, q, radius) {
			start, ds = i, a
		}
		if b < dg && nav.segment(q, to, radius) {
			goal, dg = i, b
		}
	}
	if start < 0 || goal < 0 {
		return nil, false
	}
	var previous [total]int
	for i := range previous {
		previous[i] = -2
	}
	previous[start] = -1
	var queue [total]int
	queue[0] = start
	queued := 1
	for head := 0; head < queued && previous[goal] == -2; head++ {
		i := queue[head]
		for _, offset := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			x, z := i%cols+offset[0], i/cols+offset[1]
			if x < 0 || x >= cols || z < 0 || z >= rows {
				continue
			}
			j := z*cols + x
			if !walk[j] || previous[j] != -2 || !nav.segment(point(i), point(j), radius) {
				continue
			}
			previous[j] = i
			queue[queued] = j
			queued++
		}
	}
	if previous[goal] == -2 {
		return nil, false
	}
	reverse := []Point{to}
	for i := goal; i >= 0; i = previous[i] {
		reverse = append(reverse, point(i))
	}
	route := make([]Point, 0, len(reverse))
	anchor := from
	for i := len(reverse) - 1; i >= 0; {
		j := i
		for j > 0 && nav.segment(anchor, reverse[j-1], radius) {
			j--
		}
		route = append(route, reverse[j])
		anchor = reverse[j]
		i = j - 1
	}
	return route, true
}

func (w *World) bakeryFurnitureColliders(p *player) []Collider {
	plot, ok := w.bakeryPlot(p)
	if !ok {
		return nil
	}
	out := []Collider{}
	for _, f := range p.profile.BakeryFurniture {
		r, _ := bakeryFurnitureRecipeFor(f.Kind)
		if !r.Solid {
			continue
		}
		id := "bakery_furniture_" + p.profile.ID + "_" + f.ID
		if f.Kind == "display" {
			id = "display_" + p.profile.ID
		}
		out = append(out, Collider{ID: id, X: plot.X + f.X, Z: plot.Z + f.Z, Width: r.Width, Depth: r.Depth, Y: r.Y, Height: r.Height, Rotation: f.Rotation})
	}
	return out
}

func (w *World) bakeryFurnitureVisible(p *player, f BakeryFurniture, plot BakeryPlot) bool {
	r, _ := bakeryFurnitureRecipeFor(f.Kind)
	owner := w.plotOwner(plot.ID)
	targetID := "bakery_furniture_" + owner + "_" + f.ID
	if f.Kind == "display" {
		targetID = "display_" + owner
	}
	for _, colliders := range [][]Collider{w.colliders, w.propertyColliders} {
		for _, c := range colliders {
			if c.ID == targetID || c.resource != nil && !c.resource.Available {
				continue
			}
			if t, hit := colliderSegmentHit(c, p.x, playerY(p)+1.6, p.z, plot.X+f.X, r.Y+max(.3, r.Height*.65), plot.Z+f.Z); hit && t < .995 {
				return false
			}
		}
	}
	return true
}

// Chairs have real collision. Standing places feet beside the chair before
// movement resumes, instead of leaving a player trapped inside its footprint.
func (w *World) releaseBakerySeat(p *player) {
	if p.bakerySeatID == "" {
		return
	}
	seatCollider := ""
	for _, owner := range w.players {
		prefix := owner.profile.BakeryPlot + "_seat_"
		if owner.profile.BakeryPlot != "" && strings.HasPrefix(p.bakerySeatID, prefix) {
			seatCollider = "bakery_furniture_" + owner.profile.ID + "_" + strings.TrimPrefix(p.bakerySeatID, prefix)
			break
		}
	}
	pathClear := func(x, z float64) bool {
		steps := int(math.Ceil(distance(p.x, p.z, x, z) / .15))
		for i := 1; i <= steps; i++ {
			t := float64(i) / float64(steps)
			for _, colliders := range [][]Collider{w.colliders, w.propertyColliders} {
				for _, c := range colliders {
					if c.ID == seatCollider || c.resource != nil && !c.resource.Available || c.Y >= 1.7 || c.Y+c.Height <= 0 {
						continue
					}
					if colliderOverlaps(c, p.x+(x-p.x)*t, p.z+(z-p.z)*t, .42) {
						return false
					}
				}
			}
		}
		return true
	}
	for _, radius := range []float64{1.05, 1.45, 1.9, 2.5} {
		for _, turn := range []float64{0, math.Pi / 2, -math.Pi / 2, math.Pi, math.Pi / 4, -math.Pi / 4, math.Pi * .75, -math.Pi * .75} {
			x, z := p.x-math.Sin(p.yaw+turn)*radius, p.z-math.Cos(p.yaw+turn)*radius
			if !w.solidBlockedAt(x, groundY(p), z, .42) && pathClear(x, z) {
				p.x, p.z = x, z
				p.bakerySeatID, p.posture = "", ""
				return
			}
		}
	}
	p.bakerySeatID, p.posture = "", ""
}
func (w *World) requireBakeryBuild(p *player) error {
	plot, ok := w.bakeryPlot(p)
	if !ok || w.plotOwner(plot.ID) != p.profile.ID {
		return errors.New("Claim your bakery before redecorating it.")
	}
	if p.island != "" || p.inCave || math.Abs(p.x-plot.X) > 9.7 || math.Abs(p.z-plot.Z) > 7.7 || math.Abs(playerY(p)) > 2 {
		return errors.New("Go inside your own bakery to redecorate.")
	}
	return nil
}
func (w *World) validateBakeryFurniture(p *player, candidate []BakeryFurniture, changed BakeryFurniture) error {
	r, ok := bakeryFurnitureRecipeFor(changed.Kind)
	if !ok || !finite(changed.X) || !finite(changed.Z) || !finite(changed.Rotation) || math.Abs(changed.Rotation) > 36000 || math.Abs(changed.Rotation/45-math.Round(changed.Rotation/45)) > .001 || !bakeryFurnitureInRoom(changed, r) {
		return errors.New("Place this furnishing inside its room, within the bakery walls.")
	}
	if r.Solid {
		// Keep the two door thresholds clear, including rotating corners.
		for _, doorZ := range []float64{-1, 8} {
			for x := -1.6; x <= 1.6; x += .4 {
				for z := doorZ - .5; z <= doorZ+.5; z += .25 {
					if bakeryFurnitureBlocks(changed, x, z, .35) {
						return errors.New("Keep the entrance and kitchen doorway clear.")
					}
				}
			}
		}
	}
	core := map[string]bool{}
	for _, f := range candidate {
		fr, _ := bakeryFurnitureRecipeFor(f.Kind)
		if fr.Core && core[f.Kind] {
			return errors.New("A bakery can have one oven, one mixer, and one display.")
		}
		core[f.Kind] = fr.Core || core[f.Kind]
		if f.ID != changed.ID && bakeryFurnitureOverlaps(changed, f) {
			return errors.New("Leave space between furnishings.")
		}
	}
	plot, _ := w.bakeryPlot(p)
	if r.Solid {
		for _, other := range w.players {
			if other.island == "" && !other.inCave && playerY(other) < r.Y+r.Height && bakeryFurnitureBlocks(changed, other.x-plot.X, other.z-plot.Z, .42) {
				return errors.New("Someone is standing there. Place the furnishing in a clear spot.")
			}
		}
		for _, npc := range w.bakeryNPCPositions(p) {
			if bakeryFurnitureBlocks(changed, npc.X-plot.X, npc.Z-plot.Z, .35) {
				return errors.New("A customer or member of staff is standing there.")
			}
		}
	}
	preview := *p
	preview.profile = p.profile
	preview.profile.BakeryFurniture = candidate
	entry := Point{plot.X, plot.Z + 9.5}
	for _, npc := range w.bakeryNPCPositions(p) {
		if _, ok := w.bakeryRoute(&preview, entry, npc); !ok {
			return errors.New("Leave customers and staff a clear path to the entrance.")
		}
	}
	if _, ok := w.bakeryRoute(&preview, entry, Point{plot.X, plot.Z - 3}); !ok {
		return errors.New("Keep a walkable route between the cafe and kitchen.")
	}
	for _, kind := range []string{"oven", "mixer", "display", "cooling_table"} {
		for _, back := range []bool{false, true} {
			if back && kind != "display" {
				continue
			}
			if service, ok := w.bakeryStationServicePoint(&preview, kind, back); ok {
				if _, reachable := w.bakeryRoute(&preview, entry, service); !reachable {
					return errors.New("Leave a clear path to the front of each station and behind the display.")
				}
			}
		}
	}
	return nil
}
func (w *World) bakeryFurnitureAction(p *player, a Action) error {
	if a.Action == "bakery_cancel_edit" || a.Action == "bakery_build_mode" && !a.Enabled {
		p.editingBakeryFurnitureID = ""
		if a.Action == "bakery_build_mode" {
			p.bakeryBuildMode = false
		}
		return nil
	}
	if err := w.requireBakeryBuild(p); err != nil {
		return err
	}
	if a.Action == "bakery_build_mode" {
		p.buildMode = false
		p.editingBuildID = ""
		p.bakeryBuildMode = true
		return nil
	}
	if !p.bakeryBuildMode {
		return errors.New("Enter bakery build mode before editing furnishings.")
	}
	objects := append([]BakeryFurniture{}, p.profile.BakeryFurniture...)
	if a.Action == "bakery_place" {
		if p.editingBakeryFurnitureID != "" {
			return errors.New("Place or cancel the furnishing you are already editing.")
		}
		if len(objects) >= 120 {
			return errors.New("This bakery has room for at most 120 furnishings.")
		}
		r, ok := bakeryFurnitureRecipeFor(a.Item)
		if !ok {
			return errors.New("Choose a bakery furnishing.")
		}
		f := BakeryFurniture{Kind: a.Item, X: a.X, Z: a.Z, Rotation: a.Rotation}
		if r.Core && validColor(p.profile.Paint[f.Kind]) {
			f.Color = p.profile.Paint[f.Kind]
		}
		for {
			w.sequence++
			f.ID = fmt.Sprintf("furnishing_%d", w.sequence)
			exists := false
			for _, o := range objects {
				if o.ID == f.ID {
					exists = true
				}
			}
			if !exists {
				break
			}
		}
		if a.From != "" {
			kind, color, ok := packedBakeryItem(a.From)
			if !ok || kind != f.Kind {
				return errors.New("Choose a matching packed bakery furnishing.")
			}
			f.Color = color
		}
		objects = append(objects, f)
		if err := w.validateBakeryFurniture(p, objects, f); err != nil {
			return err
		}
		if a.From != "" {
			if !take(p, a.From, 1) {
				return errors.New("You do not have that packed furnishing.")
			}
		} else {
			if p.profile.Coins < r.Cost {
				return errors.New("You do not have enough coins for this furnishing.")
			}
			p.profile.Coins -= r.Cost
		}
	} else {
		index := -1
		for i, f := range objects {
			if f.ID == a.Target {
				index = i
				break
			}
		}
		if index < 0 {
			return errors.New("That furnishing is no longer in your bakery.")
		}
		f := objects[index]
		plot, _ := w.bakeryPlot(p)
		if distance(p.x, p.z, plot.X+f.X, plot.Z+f.Z) > 12 {
			return errors.New("Move closer to edit that furnishing.")
		}
		for _, other := range w.players {
			if other.bakerySeatID == plot.ID+"_seat_"+f.ID && other.posture == "sitting" {
				return errors.New("Let the person in this chair stand up before editing it.")
			}
		}
		if a.Action == "bakery_edit" {
			if !w.bakeryFurnitureVisible(p, f, plot) {
				return errors.New("Look at this furnishing from a clear spot in the same room.")
			}
			if p.editingBakeryFurnitureID != "" && p.editingBakeryFurnitureID != f.ID {
				return errors.New("Finish or cancel your current edit first.")
			}
			p.editingBakeryFurnitureID = f.ID
			return nil
		}
		if p.editingBakeryFurnitureID != f.ID {
			return errors.New("Look at the furnishing and press E to take edit control first.")
		}
		switch a.Action {
		case "bakery_move":
			f.X, f.Z, f.Rotation = a.X, a.Z, a.Rotation
			objects[index] = f
			if err := w.validateBakeryFurniture(p, objects, f); err != nil {
				return err
			}
		case "bakery_pack":
			if productionRemovalBlocked(&p.profile, "bakery_"+f.Kind) {
				return errors.New("Finish cooking and collect this station's output before packing it.")
			}
			if !gain(p, packedBakeryName(f), 1) {
				return errors.New("Make room in your inventory before packing this furnishing.")
			}
			objects = append(objects[:index], objects[index+1:]...)
			delete(p.profile.Production, "bakery_"+f.Kind)
		case "bakery_destroy":
			if productionRemovalBlocked(&p.profile, "bakery_"+f.Kind) {
				return errors.New("Finish cooking and collect this station's output before removing it.")
			}
			r, _ := bakeryFurnitureRecipeFor(f.Kind)
			refund := r.Cost / 2
			if refund > coinLimit-p.profile.Coins {
				return errors.New("Spend some coins before collecting this furnishing's refund.")
			}
			p.profile.Coins += refund
			objects = append(objects[:index], objects[index+1:]...)
			delete(p.profile.Production, "bakery_"+f.Kind)
		case "bakery_paint":
			if !validColor(a.Item) {
				return errors.New("Choose a valid paint color.")
			}
			if f.Color == a.Item {
				return errors.New("This furnishing already has that color.")
			}
			if !take(p, "paint_"+a.Item, 1) {
				return errors.New("You need one matching paint tin.")
			}
			objects[index].Color = a.Item
			if r, _ := bakeryFurnitureRecipeFor(f.Kind); r.Core {
				if p.profile.Paint == nil {
					p.profile.Paint = map[string]string{}
				}
				p.profile.Paint[f.Kind] = a.Item
			}
			p.profile.BakeryFurniture = objects
			return nil
		default:
			return errors.New("Unknown bakery furnishing action.")
		}
	}
	p.profile.BakeryFurniture = objects
	for _, f := range objects {
		if r, _ := bakeryFurnitureRecipeFor(f.Kind); r.Core {
			if p.profile.Paint == nil {
				p.profile.Paint = map[string]string{}
			}
			p.profile.Paint[f.Kind] = f.Color
		}
	}
	p.editingBakeryFurnitureID = ""
	w.rebuildPropertyColliders()
	w.bakeryFurnitureChanged(p)
	return nil
}

func (w *World) sitBakery(p *player, target string) error {
	if p.island != "" || p.inCave || p.ridingCamel || math.Abs(playerY(p)) > 2 {
		return errors.New("Walk to a bakery chair before sitting down.")
	}
	for _, owner := range w.players {
		plot, ok := w.bakeryPlot(owner)
		if !ok {
			continue
		}
		for _, f := range owner.profile.BakeryFurniture {
			if f.Kind != "chair" || plot.ID+"_seat_"+f.ID != target {
				continue
			}
			if distance(p.x, p.z, plot.X+f.X, plot.Z+f.Z) > 2.5 {
				return errors.New("Move closer to the chair.")
			}
			if p.bakerySeatID != target && !w.bakeryFurnitureVisible(p, f, plot) {
				return errors.New("Walk to the open side of this chair.")
			}
			if owner.editingBakeryFurnitureID == f.ID {
				return errors.New("This chair is being moved.")
			}
			if p.bakerySeatID == target && p.posture == "sitting" {
				w.releaseBakerySeat(p)
				return nil
			}
			for _, other := range w.players {
				if other != p && other.bakerySeatID == target && other.posture == "sitting" {
					return errors.New("Someone is already sitting in this chair.")
				}
			}
			p.x, p.z, p.yaw = plot.X+f.X, plot.Z+f.Z, f.Rotation*math.Pi/180
			resetMovement(p)
			p.bakerySeatID, p.posture = target, "sitting"
			return nil
		}
	}
	return errors.New("That bakery chair is no longer available.")
}
func (w *World) bakeryFurnitureSnapshot(viewer *player, out *Snapshot) {
	out.BakeryFurnitureCatalog = append([]BakeryFurnitureRecipe{}, bakeryFurnitureCatalog...)
	if viewer != nil {
		if w.requireBakeryBuild(viewer) == nil {
			out.Me.BakeryBuildMode = viewer.bakeryBuildMode
			out.Me.EditingBakeryFurnitureID = viewer.editingBakeryFurnitureID
		}
		if viewer.posture == "sitting" {
			out.Me.BakerySeatID = viewer.bakerySeatID
		}
	}
	if viewer != nil && viewer.island != "" {
		return
	}
	owners := make([]string, 0, len(w.players))
	for id := range w.players {
		owners = append(owners, id)
	}
	sort.Strings(owners)
	for _, id := range owners {
		owner := w.players[id]
		plot, ok := w.bakeryPlot(owner)
		if !ok {
			continue
		}
		for _, f := range owner.profile.BakeryFurniture {
			if f.Kind != "chair" {
				continue
			}
			id := plot.ID + "_seat_" + f.ID
			node := Node{ID: id, Kind: "bakery_chair", Item: f.ID, Owner: owner.profile.ID, PlotID: plot.ID, Label: "Cafe chair", X: plot.X + f.X, Z: plot.Z + f.Z, Yaw: f.Rotation * math.Pi / 180, Available: owner.editingBakeryFurnitureID != f.ID}
			for _, other := range w.players {
				if other.bakerySeatID == id && other.posture == "sitting" {
					node.OccupiedBy = other.profile.ID
					break
				}
			}
			out.Nodes = append(out.Nodes, node)
		}
	}
}
