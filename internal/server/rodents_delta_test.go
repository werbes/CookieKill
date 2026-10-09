package server

import (
	"bytes"
	"testing"

	"cookiekill/internal/game"
)

func TestNutIslandAndRodentDeltaLifecycle(t *testing.T) {
	w := game.New(map[string]game.Profile{"keeper": {ID: "keeper", Name: "Keeper", Coins: 500, Inventory: map[string]int{"pet_wood_mouse": 1}, NutIsland: &game.NutIsland{Owner: "keeper"}}})
	w.Join("keeper", "Keeper")
	encoder, receiver := &stateEncoder{delta: true}, &testDeltaReceiver{}
	transfer := func() game.Snapshot {
		s := w.Snapshot("keeper")
		assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, s)), s)
		encoder.commit(s)
		return s
	}
	initial := transfer()
	if len(initial.Rodents) == 0 || len(initial.RodentCatalog) != 12 {
		t.Fatal("missing initial wildlife or species catalog")
	}
	act := func(action, item string) {
		t.Helper()
		if err := w.Act("keeper", game.Action{Action: action, Item: item}); err != nil {
			t.Fatal(err)
		}
	}
	act("teleport_nut_island", "")
	arrival := transfer()
	if arrival.NutIsland == nil || len(arrival.Rodents) != 0 || arrival.Island != nil {
		t.Fatal("nut island did not replace mainland wildlife and home scenery")
	}
	act("rodent_release", "pet_wood_mouse")
	released := transfer()
	if len(released.NutIsland.Pets) != 1 || released.Me.Inventory["pet_wood_mouse"] != 0 {
		t.Fatal("release did not transfer exactly one inventory pet")
	}
	retained := marshalDeltaTest(t, released)
	baseline := marshalDeltaTest(t, *encoder.previous)
	for i := 0; i < 10; i++ {
		w.Tick(.1)
	}
	if !bytes.Equal(retained, marshalDeltaTest(t, released)) || !bytes.Equal(baseline, marshalDeltaTest(t, *encoder.previous)) {
		t.Fatal("pet movement mutated a prior snapshot or delta baseline")
	}
	transfer()
	act("nut_island_leave", "")
	returned := transfer()
	if returned.NutIsland != nil || returned.Me.NutIsland == nil || len(returned.Me.NutIsland.Pets) != 1 {
		t.Fatal("return failed to clear island view or erased permanent companions")
	}
}

func TestRodentRainbowDeltaExpiresAndEntitiesDisappear(t *testing.T) {
	encoder, receiver := &stateEncoder{delta: true}, &testDeltaReceiver{}
	s := game.Snapshot{Rodents: []game.RodentView{{ID: "otter", Species: "otter", State: "fleeing"}}, RodentCatalog: []game.RodentSpecies{{ID: "otter", Rarity: "mythic"}}}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, s)), s)
	encoder.commit(s)
	s.Rodents = []game.RodentView{{ID: "otter", Species: "otter", State: "rainbow", Y: 8}}
	s.RodentWorld = game.RodentWorldView{RainbowUntil: 60, OtterNextDay: 4}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, s)), s)
	encoder.commit(s)
	s.Time, s.Rodents = 61, []game.RodentView{}
	s.RodentWorld.RainbowUntil = 0
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, s)), s)
}
