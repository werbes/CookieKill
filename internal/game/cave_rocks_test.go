package game

import "testing"

func TestCaveOutcropIsSolidAndHorizontalMouthHasCamelHeadroom(t *testing.T) {
	w := New(nil)
	boulders, outcrop := 0, 0
	for _, prop := range w.layout.Props {
		switch prop.Kind {
		case "cave_rock":
			boulders++
			if !w.solidBlockedAt(prop.X, 0, prop.Z, .42) {
				t.Fatalf("visible boulder %s has no surface collision", prop.ID)
			}
		case "cave_outcrop":
			outcrop++
			if !w.solidBlockedAt(prop.X, prop.Y, prop.Z, .42) {
				t.Fatalf("outcrop mass %s has no collision", prop.ID)
			}
		case "cave_stalagmite":
			if !w.solidBlockedAt(prop.X, CaveFloorY, prop.Z, .42) {
				t.Fatalf("dripstone %s has no collision", prop.ID)
			}
		}
	}
	if boulders < 10 || outcrop != 7 {
		t.Fatalf("incomplete rocky outcrop: %d boulders, %d masses", boulders, outcrop)
	}
	for z := -68.; z >= -115; z -= .125 {
		y := 0.
		if z < CaveMouthZ {
			y = caveGroundY(120, z)
		}
		if w.solidBlockedAt(120, y, z, .42) || w.solidBlockedAt(120, y+1.4, z, .42) {
			t.Fatalf("entry/stairs lack camel headroom at z=%v y=%v", z, y)
		}
	}
	for _, point := range []Point{{110, -80}, {130, -80}, {120, -92}} {
		if !w.solidBlockedAt(point.X, 0, point.Z, .42) {
			t.Fatalf("surface player can walk through outcrop at %+v", point)
		}
	}
	for _, x := range []float64{117.5, 120, 122.5} {
		for z := -68.; z > CaveStairTopZ; z -= .25 {
			if w.solidBlockedAt(x, 0, z, .42) {
				t.Fatalf("fallen rocks narrowed the clear horizontal approach at (%v,%v)", x, z)
			}
		}
	}
}

func TestMountedCamelCanDescendAndClimbTheDeepCave(t *testing.T) {
	w, p := testWorld()
	p.x, p.z = 120, -70
	p.profile.Camels = []Camel{{ID: "one_camel_1", X: 120, Z: -70}}
	camelCompatibility(&p.profile)
	if err := w.mountCamel(p, "one_camel_1"); err != nil {
		t.Fatal(err)
	}
	for _, target := range []float64{-117, -70} {
		for step := 0; step < 500; step++ {
			dz := target - p.z
			if dz > -.2 && dz < .2 {
				break
			}
			direction := 1.
			if dz < 0 {
				direction = -1
			}
			w.Input("one", Input{Z: direction * .4})
			w.Tick(.05)
		}
		if d := p.z - target; d > .3 || d < -.3 {
			t.Fatalf("mounted route blocked at z=%v on way to %v", p.z, target)
		}
		if !p.ridingCamel {
			t.Fatal("cave travel lost mounted camel")
		}
		if target < -115 && (groundY(p) != CaveFloorY || !p.inCave) {
			t.Fatal("mounted descent did not reach the24metre chamber")
		}
		if target > CaveMouthZ && (groundY(p) != 0 || p.inCave) {
			t.Fatal("mounted climb did not return to surface")
		}
	}
}
