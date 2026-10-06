package game

import "testing"

func TestTreeChoppingYieldsBySizeAndRemovesCollisionUntilRegrowth(t *testing.T) {
	w := New(nil)
	w.Join("lumberjack", "Lumberjack")
	p := w.players["lumberjack"]
	var tree *Node
	for _, n := range w.nodes {
		if n.ID == "starter_0" {
			tree = n
			break
		}
	}
	if tree == nil {
		t.Fatal("missing starter tree")
	}
	p.x, p.z = tree.X+2, tree.Z
	if tree.Amount < 3 || tree.Amount > 6 || !w.staticBlocked(tree.X, tree.Z, .1) {
		t.Fatal("standing tree has incorrect yield or no trunk collision")
	}
	for i := 0; i < tree.ChopTotal; i++ {
		w.time += .71
		if err := w.Act("lumberjack", Action{Action: "interact", Target: tree.ID}); err != nil {
			t.Fatal(err)
		}
		if i < tree.ChopTotal-1 && p.profile.Inventory["wood"] != 0 {
			t.Fatal("wood was gathered before chopping the tree down")
		}
	}
	if tree.Available || p.profile.Inventory["wood"] != tree.Amount || w.staticBlocked(tree.X, tree.Z, .1) {
		t.Fatal("felled tree remained solid or gave wrong amount of wood")
	}
	w.time = tree.readyAt
	w.Tick(.05)
	if !tree.Available || tree.ChopRemaining != tree.ChopTotal || !w.staticBlocked(tree.X, tree.Z, .1) {
		t.Fatal("tree did not regrow with renewed chopping progress")
	}
	var largest, smallest *Node
	for _, n := range w.nodes {
		if n.Kind == "wood" {
			if largest == nil || n.Scale > largest.Scale {
				largest = n
			}
			if smallest == nil || n.Scale < smallest.Scale {
				smallest = n
			}
		}
	}
	if largest.Amount != 6 || smallest.Amount != 3 || largest.ChopTotal <= smallest.ChopTotal {
		t.Fatal("large trees must take more chops and yield more wood")
	}
}

func TestCaveRampForkAndLollipopsAreReachable(t *testing.T) {
	w := New(nil)
	w.Join("explorer", "Explorer")
	p := w.players["explorer"]
	p.x, p.z = 120, -98
	for i := 0; i < 63; i++ {
		w.Input("explorer", Input{Z: -1})
		w.Tick(.05)
	}
	if p.z > -116 || groundY(p) != -4 {
		t.Fatalf("cave ramp is blocked or does not descend: %.2f %.2f", p.z, groundY(p))
	}
	for _, x := range []float64{111, 129} {
		for step := 0; step <= 30; step++ {
			px := 120 + (x-120)*float64(step)/30
			if w.staticBlocked(px, -117, .42) {
				t.Fatalf("fork passage blocked at %.2f", px)
			}
		}
		for z := -117.; z >= -124; z -= .2 {
			if w.staticBlocked(x, z, .42) || caveGroundY(x, z) != -4 {
				t.Fatal("lollipop branch is inaccessible")
			}
		}
	}
	for _, id := range []string{"pink_lollipop", "blue_lollipop", "abu_fanous"} {
		found := false
		for _, n := range w.nodes {
			if n.ID == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s", id)
		}
	}
}
