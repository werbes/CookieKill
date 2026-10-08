package server

import (
	"cookiekill/internal/game"
	"testing"
)

func TestTownAndRaceChangesSurviveDeltaReconstruction(t *testing.T) {
	encoder := &stateEncoder{delta: true}
	receiver := &testDeltaReceiver{}
	before := game.Snapshot{CamelRace: game.CamelRaceView{Phase: "registration"}, Town: game.TownView{Leaderboards: []game.TownLeaderboard{{ID: "kills", Entries: []game.TownRank{}}}}}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, before)), before)
	encoder.commit(before)
	after := before
	after.CamelRace = game.CamelRaceView{InArena: true, Phase: "gathering", Entrants: []game.CamelRaceEntryView{{ID: "one", Name: "Rider", Status: "racing"}}}
	after.Town = game.TownView{Leaderboards: []game.TownLeaderboard{{ID: "camel_coins", Entries: []game.TownRank{{ID: "one", Rank: 1, Value: 50}}}}}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, after)), after)
	encoder.commit(after)
	after.CamelRace = game.CamelRaceView{Phase: "registration"}
	after.Town.Leaderboards = []game.TownLeaderboard{}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, after)), after)
}
