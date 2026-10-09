package server

import (
	"encoding/json"
	"testing"

	"cookiekill/internal/game"
)

func TestRollerCoasterDeltaLifecyclePreservesTrackAndClearsRide(t *testing.T) {
	encoder := &stateEncoder{delta: true}
	receiver := &testDeltaReceiver{}
	before := game.Snapshot{RollerCoaster: game.RollerCoasterView{Station: game.CoasterPoint{X: 165, Z: -161}, Duration: 48, Track: []game.CoasterPoint{{X: 165, Y: 1.1, Z: -165}, {T: 20, X: 200, Y: 24, Z: -190}}, Cars: []game.CoasterCar{}}}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, before)), before)
	encoder.commit(before)
	after := before
	after.RollerCoaster.Cars = []game.CoasterCar{{ID: "coaster_one", PlayerID: "one", CoasterPoint: game.CoasterPoint{X: 166, Y: 2, Z: -165}, Progress: .1, Remaining: 43.2}}
	after.RollerCoaster.Ride = &after.RollerCoaster.Cars[0]
	frame := encodeDeltaTest(t, encoder, after)
	var patch map[string]json.RawMessage
	decodeDeltaTest(t, frame, &patch)
	var coaster map[string]json.RawMessage
	decodeDeltaTest(t, patch["rollerCoaster"], &coaster)
	if _, repeated := coaster["track"]; repeated || coaster["ride"] == nil || coaster["cars"] == nil {
		t.Fatal("coaster delta repeats immutable track or omits boarding state")
	}
	assertDeltaSnapshot(t, receiver.apply(t, frame), after)
	encoder.commit(after)
	returned := before
	frame = encodeDeltaTest(t, encoder, returned)
	decodeDeltaTest(t, frame, &patch)
	decodeDeltaTest(t, patch["rollerCoaster"], &coaster)
	if string(coaster["ride"]) != "null" || string(coaster["cars"]) != "[]" {
		t.Fatal("ride exit or world transition did not explicitly clear rider and cars")
	}
	assertDeltaSnapshot(t, receiver.apply(t, frame), returned)
	encoder.commit(returned)
	changedTrack := returned
	changedTrack.RollerCoaster.Track = []game.CoasterPoint{}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, changedTrack)), changedTrack)
}
