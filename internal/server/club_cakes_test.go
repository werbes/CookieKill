package server

import (
	"testing"

	"cookiekill/internal/game"
)

func TestClubCakeCatalogSellerAndEffectSurviveDeltaReconstruction(t *testing.T) {
	encoder := &stateEncoder{delta: true}
	receiver := &testDeltaReceiver{}
	day := game.Snapshot{
		ClubCakes: []game.ClubCake{{ID: "funny_cake", Name: "Funny cake", Price: 15, Duration: 45}},
		Nodes:     []game.Node{{ID: "club_cake_seller", Kind: "club_cake_seller", Available: false, InCave: true}},
	}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, day)), day)
	encoder.commit(day)
	night := day
	night.Nodes = []game.Node{{ID: "club_cake_seller", Kind: "club_cake_seller", Available: true, InCave: true}}
	night.Me.Buffs = map[string]float64{"funny_cake": 45}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, night)), night)
	encoder.commit(night)
	dawn := day
	dawn.ClubCakes = []game.ClubCake{{ID: "cosmic_cake", Name: "Cosmic cake", Price: 25, Duration: 75}}
	dawn.Me.Buffs = map[string]float64{}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, dawn)), dawn)
	encoder.commit(dawn)
	dawn.ClubCakes = []game.ClubCake{}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, dawn)), dawn)
}
