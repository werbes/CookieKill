package server

import (
	"testing"

	"cookiekill/internal/game"
)

func TestCookieActivitiesAndRingKillsSurviveDeltaReconstruction(t *testing.T) {
	encoder, receiver := &stateEncoder{delta: true}, &testDeltaReceiver{}
	before := game.Snapshot{}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, before)), before)
	encoder.commit(before)
	practice := before
	practice.Me.Island = game.CookiePracticeIsland
	practice.CookieArena = game.CookieArenaView{InArena: true, Mode: "practice", AmmoItem: "sugar", UnlimitedAmmo: true}
	practice.CookiePractice = &game.CookiePracticeView{Hits: 1, Exercise: game.PracticeExerciseView{Station: "practice_bench_press", Progress: 3, Goal: 10, Active: true}}
	practice.Players = []game.PlayerView{{ID: "friend", Grounded: true, Sprinting: true, Island: game.CookiePracticeIsland}}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, practice)), practice)
	encoder.commit(practice)
	ring := practice
	ring.Me.Island, ring.Me.Kills, ring.Me.CookieRingKills = game.CookieRingIsland, 8, 2
	ring.CookiePractice = nil
	ring.CookieArena = game.CookieArenaView{InArena: true, Mode: "ring", Kills: 1, ShieldSeconds: 2.5}
	ring.Players = []game.PlayerView{{ID: "friend", Grounded: false, Sprinting: false, Island: game.CookieRingIsland}}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, ring)), ring)
	encoder.commit(ring)
	back := ring
	back.Me.Island = ""
	back.CookieArena = game.CookieArenaView{}
	back.Players = nil
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, back)), back)
}
