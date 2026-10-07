package game

import (
	"strings"
	"testing"
)

func TestEverySceneryCactusIsAChoppableResource(t *testing.T) {
	w, p := testWorld()
	nodes := map[string]*Node{}
	for _, n := range w.nodes {
		if n.Kind == "cactus" {
			nodes[n.ID] = n
		}
	}
	props, gardens, roof := 0, 0, 0
	for _, prop := range w.layout.Props {
		if prop.Kind != "cactus" {
			continue
		}
		props++
		if strings.Contains(prop.ID, "house_cactus_") || strings.Contains(prop.ID, "village_house_") {
			gardens++
		}
		if strings.HasPrefix(prop.ID, "cave_roof_cactus_") {
			roof++
		}
		n := nodes[prop.ID]
		if n == nil || n.X != prop.X || n.Z != prop.Z || n.Scale != prop.Scale || n.Variant != prop.Variant || n.Color != prop.Color || n.ChopTotal < 2 || n.Amount < 2 {
			t.Fatalf("visible cactus %s has no matching harvestable resource", prop.ID)
		}
		linked := false
		for _, c := range w.colliders {
			if c.ID == n.ID && c.resource == n {
				linked = true
			}
		}
		if !linked {
			t.Fatalf("cactus %s collider cannot be removed after chopping", n.ID)
		}
		if !w.solidBlockedAt(n.X, 0, n.Z, .1) {
			t.Fatalf("standing cactus %s is not solid", n.ID)
		}
		p.x, p.z = n.X+2, n.Z
		before := p.profile.Inventory["cactus"]
		for i := 0; i < n.ChopTotal; i++ {
			w.time += .71
			mustAct(t, w, Action{Action: "interact", Target: n.ID})
			if i < n.ChopTotal-1 && p.profile.Inventory["cactus"] != before {
				t.Fatal("cactus yielded ingredients before it was chopped down")
			}
		}
		if n.Available || n.ChopRemaining != 0 || p.profile.Inventory["cactus"] != before+n.Amount {
			t.Fatalf("cactus %s did not disappear and yield usable ingredients", n.ID)
		}
		if w.solidBlockedAt(n.X, 0, n.Z, .1) {
			t.Fatalf("felled cactus %s left an invisible movement collider", n.ID)
		}
		if _, hit := w.staticShotHit(n.X, 1, n.Z, n.X+.05, 1, n.Z); hit {
			t.Fatalf("felled cactus %s still blocks thrown cookies", n.ID)
		}
	}
	if props != len(nodes) || props < 40 || gardens < 6 || roof != 3 {
		t.Fatalf("incomplete cactus coverage: %d props, %d nodes, %d gardens, %d roof", props, len(nodes), gardens, roof)
	}
	// The yielded item is the actual ingredient consumed by existing recipes.
	nodePosition(w, p, "desert_oven")
	setItem(p, "dough", 2)
	before := p.profile.Inventory["cactus"]
	mustAct(t, w, Action{Action: "craft", Item: "cactus_cookie"})
	if p.profile.Inventory["cactus"] != before-2 || p.profile.Inventory["cactus_cookie"] != 1 {
		t.Fatal("harvested cactus was not usable for baking")
	}
}

func TestCactusRegrowthWaitsForClearSurface(t *testing.T) {
	w, p := testWorld()
	var cactus *Node
	for _, n := range w.nodes {
		if n.ID == "cave_roof_cactus_0" {
			cactus = n
		}
	}
	if cactus == nil {
		t.Fatal("missing cave roof cactus")
	}
	cactus.Available, cactus.ChopRemaining, cactus.readyAt = false, 0, 0
	p.x, p.z = cactus.X, cactus.Z
	w.Tick(.05)
	if cactus.Available {
		t.Fatal("cactus regrew around a surface player")
	}
	p.inCave = true
	w.time = cactus.readyAt
	w.Tick(.05)
	if !cactus.Available || cactus.ChopRemaining != cactus.ChopTotal {
		t.Fatal("underground player prevented safe surface cactus regrowth")
	}
	w.time += 1
	if err := w.Act("one", Action{Action: "interact", Target: cactus.ID}); err == nil {
		t.Fatal("player chopped roof cactus through the cave ceiling")
	}
}
