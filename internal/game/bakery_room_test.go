package game

import (
	"math"
	"testing"
)

func TestBakeryRoomsKeepPlotsAndCityRoadsClear(t *testing.T) {
	w, _ := testWorld()
	if len(w.layout.Plots) != 12 {
		t.Fatal("city lost bakery plots")
	}
	for i, plot := range w.layout.Plots {
		wantX := 24 + float64(i%6)*34
		if i%6 >= 3 {
			wantX += 5
		}
		if plot.X != wantX || plot.Z != 72+float64(i/6)*42 || plot.Width != 20 || plot.Depth != 16 {
			t.Fatalf("plot moved or room dimensions differ: %+v", plot)
		}
		for _, prop := range w.layout.Props {
			if prop.ID == plot.ID && (prop.Width != 20 || prop.Depth != 16 || prop.Height != 5.8) {
				t.Fatalf("visible shell mismatches collider dimensions: %+v", prop)
			}
		}
		for _, local := range []Point{{0, 8}, {0, -1}, {0, -6}, {-5, -5.5}, {5, -5.5}} {
			if w.solidBlockedAt(plot.X+local.X, 0, plot.Z+local.Z, .42) {
				t.Fatalf("empty room or doorway obstructed at %+v in %s", local, plot.ID)
			}
		}
		for _, x := range []float64{-6, 6} {
			if !w.solidBlockedAt(plot.X+x, 0, plot.Z-1, .42) {
				t.Fatal("kitchen partition is not solid")
			}
		}
		if w.solidBlockedAt(plot.X, 0, plot.Z-1, .42) || !w.solidBlockedAt(plot.X, 2, plot.Z-1, .42) {
			t.Fatal("partition doorway lintel does not match 3.5m headroom")
		}
		if w.solidBlockedAt(plot.X, 0, plot.Z+9.5, .35) {
			t.Fatal("customer sidewalk arrival point is blocked")
		}
	}
}

func TestBakeryPlayerWalksThroughBothRoomsButCannotCrossPartitionSides(t *testing.T) {
	w, p := testWorld()
	plot := w.layout.Plots[0]
	p.x, p.z = plot.X, plot.Z+9.5
	for i := 0; i < 52; i++ {
		w.Input("one", Input{Z: -1})
		w.Tick(.05)
	}
	if p.z >= plot.Z-5 || math.Abs(p.x-plot.X) > .01 {
		t.Fatalf("central route through cafe and kitchen blocked: %v,%v", p.x, p.z)
	}
	p.x, p.z = plot.X+5, plot.Z-4
	for i := 0; i < 30; i++ {
		w.Input("one", Input{Z: 1})
		w.Tick(.05)
	}
	if p.z > plot.Z-1.5 {
		t.Fatalf("walked through the kitchen partition: %v", p.z)
	}
}
