package server

import (
	"cookiekill/internal/game"
	"testing"
	"time"
)

func TestNightCycleAndJohnSpawnMoveDespawnSurviveDelta(t *testing.T) {
	e := &stateEncoder{delta: true}
	r := &testDeltaReceiver{}
	first := game.Snapshot{DayCycle: game.DayCycle{Phase: "day", Elapsed: 719, Remaining: 1, DayDuration: 720, NightDuration: 360}}
	r.apply(t, encodeDeltaTest(t, e, first))
	e.commit(first)
	night := first
	night.DayCycle = game.DayCycle{Phase: "night", Remaining: 360, DayDuration: 720, NightDuration: 360}
	night.JohnPork = &game.JohnPorkView{ID: "john_pork", X: 15, Z: -20, Target: "rich", Island: "", InCave: false}
	assertDeltaSnapshot(t, r.apply(t, encodeDeltaTest(t, e, night)), night)
	e.commit(night)
	moved := night
	john := *night.JohnPork
	john.X = 14.5
	john.Eating = true
	moved.JohnPork = &john
	moved.DayCycle.Elapsed = .1
	moved.DayCycle.Remaining = 359.9
	assertDeltaSnapshot(t, r.apply(t, encodeDeltaTest(t, e, moved)), moved)
	e.commit(moved)
	dawn := first
	dawn.DayCycle = game.DayCycle{DayNumber: 1, Phase: "day", Remaining: 720, DayDuration: 720, NightDuration: 360}
	dawn.JohnPork = &game.JohnPorkView{ID: "john_pork", X: game.JohnBedX, Y: game.JohnBedY, Z: game.JohnBedZ, Yaw: game.JohnBedYaw, Sleeping: true}
	got := r.apply(t, encodeDeltaTest(t, e, dawn))
	assertDeltaSnapshot(t, got, dawn)
	if got.JohnPork == nil || !got.JohnPork.Sleeping || got.JohnPork.Target != "" || got.JohnPork.Eating || got.DayCycle.DayNumber != 1 {
		t.Fatal("dawn did not return John to bed or lost the completed day count")
	}
}

func TestDisconnectedGraceSessionCannotRemainJohnsTarget(t *testing.T) {
	w := game.New(map[string]game.Profile{
		"rich":  {ID: "rich", Coins: 1000, Inventory: map[string]int{"sugar": 30}},
		"other": {ID: "other", Coins: 100, Inventory: map[string]int{"sugar": 30}},
	})
	for i := 0; i < 720; i++ {
		w.Tick(1)
	}
	w.Join("rich", "Rich")
	w.Join("other", "Other")
	for i := 0; i < 4; i++ {
		w.Tick(1)
	}
	if john := w.SnapshotUpdate("rich").JohnPork; john == nil || john.Target != "rich" {
		t.Fatal("highest coin balance was not selected at night")
	}
	p := &peer{out: make(chan outbound, 2), baselineReady: make(chan struct{})}
	c := &coordinator{world: w, peers: map[string]*peer{"rich": p}, departed: map[string]time.Time{}}
	c.handle(detachCommand{id: "rich", peer: p})
	w.Tick(.05)
	if john := w.SnapshotUpdate("other").JohnPork; john == nil || john.Target != "other" {
		t.Fatal("10-second grace period let John keep chasing a disconnected account")
	}
	if len(c.departed) != 1 {
		t.Fatal("test did not exercise an actual grace-period disconnect")
	}
}
