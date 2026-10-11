package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"cookiekill/internal/game"
	"github.com/coder/websocket"
)

func animalPathFixture() game.Animal {
	return game.Animal{ID: "swimmer", Species: "dolphin", Health: 108, MaxHealth: 108, Scale: 1, GroupID: "pod", Disposition: "calm", Motion: &game.AnimalMotion{
		Kind: "orbit", Epoch: 10, Transition: 3, FromX: -65, FromZ: 100, Heading: 1,
		CenterX: -70, CenterZ: 100, Radius: 10, Phase: .4, AngularSpeed: .3, WanderPhase: .6, JumpPhase: 1, Speed: 3, Age: 25,
	}}
}

func animalPathSnapshot(a game.Animal, now float64) game.Snapshot {
	if a.Motion != nil {
		p := a.Motion.Sample(a.Species, a.Egg, now)
		a.X, a.Y, a.Z, a.Yaw, a.Heading, a.Speed, a.Age, a.HatchIn = p.X, p.Y, p.Z, p.Yaw, p.Heading, p.Speed, p.Age, p.HatchIn
	}
	return game.Snapshot{Time: now, ServerTime: 2_000_000_000, Animals: []game.Animal{a}, Layout: &game.Layout{Version: 2}}
}

func animalChanges(t *testing.T, data []byte) ([]map[string]json.RawMessage, []string) {
	t.Helper()
	var frame map[string]json.RawMessage
	decodeDeltaTest(t, data, &frame)
	if frame["animals"] == nil {
		return nil, nil
	}
	var changes struct {
		Upsert []map[string]json.RawMessage `json:"upsert"`
		Remove []string                     `json:"remove"`
	}
	decodeDeltaTest(t, frame["animals"], &changes)
	return changes.Upsert, changes.Remove
}

func TestAnimalPathsSuppressQuietPoseAndClockUpdates(t *testing.T) {
	for _, kind := range []string{"orbit", "trapped", "egg"} {
		t.Run(kind, func(t *testing.T) {
			a := animalPathFixture()
			if kind != "orbit" {
				a.Motion.Kind = "still"
				a.Need = "trapped"
			}
			if kind == "egg" {
				a.Egg, a.Need, a.Motion.Age, a.Motion.HatchAt = true, "", 0, 130
			}
			first := animalPathSnapshot(a, 10)
			encoder, receiver := stateEncoder{delta: true, animalPaths: true}, testDeltaReceiver{}
			initial := receiver.apply(t, encodeDeltaTest(t, &encoder, first))
			encoder.commit(first)
			for _, now := range []float64{10.1, 11, 16.2, 40} {
				next := animalPathSnapshot(a, now)
				data := encodeDeltaTest(t, &encoder, next)
				upsert, remove := animalChanges(t, data)
				if len(upsert) != 0 || len(remove) != 0 {
					t.Fatalf("quiet %s produced animal updates at %g: %s", kind, now, data)
				}
				got := receiver.apply(t, data)
				if !reflect.DeepEqual(got.Animals, initial.Animals) || got.Time != now {
					t.Fatal("receiver lost route anchor or advancing simulation clock")
				}
				encoder.commit(next)
			}
		})
	}
}

func TestAnimalPathsDeliverSemanticChangesRemovalAndReappearance(t *testing.T) {
	a := animalPathFixture()
	encoder, receiver := stateEncoder{delta: true, animalPaths: true}, testDeltaReceiver{}
	first := animalPathSnapshot(a, 10)
	receiver.apply(t, encodeDeltaTest(t, &encoder, first))
	encoder.commit(first)
	for _, health := range []float64{60, 108} {
		a.Health, a.Need = health, "wounded"
		if health == a.MaxHealth {
			a.Need = ""
		}
		next := animalPathSnapshot(a, 12)
		data := encodeDeltaTest(t, &encoder, next)
		upserts, _ := animalChanges(t, data)
		if len(upserts) != 1 || upserts[0]["health"] == nil || upserts[0]["need"] == nil || upserts[0]["motion"] != nil {
			t.Fatalf("semantic patch lost changes or repeated unchanged route: %s", data)
		}
		got := receiver.apply(t, data)
		if got.Animals[0].Health != health || got.Animals[0].Need != a.Need {
			t.Fatal("damage/help was not reconstructed")
		}
		encoder.commit(next)
	}
	dead := game.Snapshot{Time: 13, ServerTime: first.ServerTime, Animals: []game.Animal{}}
	data := encodeDeltaTest(t, &encoder, dead)
	_, removed := animalChanges(t, data)
	if !reflect.DeepEqual(removed, []string{a.ID}) || len(receiver.apply(t, data).Animals) != 0 {
		t.Fatal("dead animal remained on the receiver")
	}
	encoder.commit(dead)
	a.Egg, a.Health, a.Motion = true, 1, &game.AnimalMotion{Kind: "still", Epoch: 43, FromX: -50, FromZ: 100, HatchAt: 163}
	egg := animalPathSnapshot(a, 43)
	got := receiver.apply(t, encodeDeltaTest(t, &encoder, egg))
	if len(got.Animals) != 1 || !got.Animals[0].Egg || got.Animals[0].Motion == nil || got.Animals[0].Motion.HatchAt != 163 || got.Animals[0].HatchIn != 120 {
		t.Fatal("reappearing egg omitted its complete lifecycle state")
	}
}

func TestAnimalPathsSwitchBetweenRoutesAndLiveCoordinates(t *testing.T) {
	a := animalPathFixture()
	encoder, receiver := stateEncoder{delta: true, animalPaths: true}, testDeltaReceiver{}
	first := animalPathSnapshot(a, 10)
	receiver.apply(t, encodeDeltaTest(t, &encoder, first))
	encoder.commit(first)
	a.Motion, a.X, a.Y, a.Z, a.Yaw, a.Heading, a.Speed, a.Age = nil, 0, 2, 90, 0, 0, 0, 27
	live := animalPathSnapshot(a, 12)
	data := encodeDeltaTest(t, &encoder, live)
	upserts, _ := animalChanges(t, data)
	if len(upserts) != 1 || string(upserts[0]["motion"]) != "null" || string(upserts[0]["x"]) != "0" || string(upserts[0]["heading"]) != "0" {
		t.Fatalf("entering live motion did not clear route and send zero-valued coordinates: %s", data)
	}
	got := receiver.apply(t, data)
	if !reflect.DeepEqual(got.Animals, live.Animals) {
		t.Fatal("live coordinates were not fully reconstructed")
	}
	encoder.commit(live)
	a.Motion = &game.AnimalMotion{Kind: "orbit", Epoch: 14, Transition: 5, FromX: 0, FromZ: 90, CenterX: -50, CenterZ: 100, Radius: 9, AngularSpeed: .4, Speed: 4, Age: 29}
	resumed := animalPathSnapshot(a, 14)
	data = encodeDeltaTest(t, &encoder, resumed)
	upserts, _ = animalChanges(t, data)
	if len(upserts) != 1 || upserts[0]["motion"] == nil || string(upserts[0]["motion"]) == "null" {
		t.Fatal("leaving live motion omitted the new route")
	}
	got = receiver.apply(t, data)
	if !reflect.DeepEqual(got.Animals, resumed.Animals) {
		t.Fatal("resumed route did not reconstruct its anchor")
	}
	encoder.commit(resumed)
	upserts, removed := animalChanges(t, encodeDeltaTest(t, &encoder, animalPathSnapshot(a, 15)))
	if len(upserts) != 0 || len(removed) != 0 {
		t.Fatal("resumed route kept streaming poses")
	}
}

func TestAnimalPathsUnsentFramesDoNotAdvanceBaseline(t *testing.T) {
	a := animalPathFixture()
	encoder, receiver := stateEncoder{delta: true, animalPaths: true}, testDeltaReceiver{}
	first := animalPathSnapshot(a, 10)
	receiver.apply(t, encodeDeltaTest(t, &encoder, first))
	encoder.commit(first)
	a.Motion, a.X, a.Z = nil, -50, 90
	live := animalPathSnapshot(a, 12)
	discarded := encodeDeltaTest(t, &encoder, live)
	if retry := encodeDeltaTest(t, &encoder, live); !bytes.Equal(discarded, retry) {
		t.Fatal("encoding an unsent motion transition changed its retry")
	}
	a.Motion = &game.AnimalMotion{Kind: "orbit", Epoch: 14, Transition: 5, FromX: -50, FromZ: 90, CenterX: -50, CenterZ: 100, Radius: 9, AngularSpeed: .4, Speed: 4, Age: 29}
	resumed := animalPathSnapshot(a, 14)
	got := receiver.apply(t, encodeDeltaTest(t, &encoder, resumed))
	if !reflect.DeepEqual(got.Animals, resumed.Animals) || receiver.seq != 2 {
		t.Fatal("skipping live transition broke reconstruction from last written route")
	}
	encoder.commit(resumed)
	// An unsent route change must also not hide a subsequent live transition.
	a.Motion = animalPathFixture().Motion
	encodeDeltaTest(t, &encoder, animalPathSnapshot(a, 15))
	got = receiver.apply(t, encodeDeltaTest(t, &encoder, live))
	if !reflect.DeepEqual(got.Animals, live.Animals) || receiver.seq != 3 {
		t.Fatal("skipping route change broke subsequent live coordinates")
	}
}

func TestAnimalPathsPreserveInputSnapshotsAndLegacyPoses(t *testing.T) {
	first, next := animalPathSnapshot(animalPathFixture(), 12), animalPathSnapshot(animalPathFixture(), 13)
	before := marshalDeltaTest(t, next)
	encoder := stateEncoder{delta: true, animalPaths: true}
	encodeDeltaTest(t, &encoder, next)
	encoder.commit(next)
	if !bytes.Equal(before, marshalDeltaTest(t, next)) || next.Animals[0].Motion.Age != 25 {
		t.Fatal("path projection mutated caller's snapshot or shared descriptor")
	}
	legacy := stateEncoder{delta: true}
	receiver := testDeltaReceiver{}
	receiver.apply(t, encodeDeltaTest(t, &legacy, first))
	legacy.commit(first)
	data := encodeDeltaTest(t, &legacy, next)
	upserts, _ := animalChanges(t, data)
	if len(upserts) != 1 || upserts[0]["x"] == nil || upserts[0]["age"] == nil {
		t.Fatal("legacy delta client lost current coordinates or age")
	}
	if got := receiver.apply(t, data); !reflect.DeepEqual(got.Animals, next.Animals) {
		t.Fatal("legacy client did not reconstruct authoritative poses")
	}
	reconnected, fresh := stateEncoder{delta: true, animalPaths: true}, testDeltaReceiver{}
	got := fresh.apply(t, encodeDeltaTest(t, &reconnected, next))
	if got.Layout == nil || got.Animals[0].Motion == nil || !reflect.DeepEqual(got.Animals[0].Motion, next.Animals[0].Motion) || fresh.seq != 1 {
		t.Fatal("reconnect did not receive a fresh route descriptor and world baseline")
	}
	full := stateEncoder{}
	full.commit(first)
	var snapshot game.Snapshot
	decodeDeltaTest(t, encodeDeltaTest(t, &full, next), &snapshot)
	if !reflect.DeepEqual(snapshot.Animals, next.Animals) {
		t.Fatal("legacy full-snapshot client lost authoritative animal coordinates")
	}
}

func TestAnimalPathsWebsocketOptIn(t *testing.T) {
	for _, mode := range []struct {
		name, query string
		paths       bool
	}{{"paths", "?updates=delta-v1&animals=paths-v1", true}, {"legacy", "?updates=delta-v1", false}, {"unknown", "?updates=delta-v1&animals=unknown", false}} {
		t.Run(mode.name, func(t *testing.T) {
			_, srv := testServer(t)
			client := signIn(t, srv.URL, "animal-paths@example.com")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws"+mode.query, &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": {srv.URL}}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { conn.CloseNow() })
			conn.SetReadLimit(2 << 20)
			receiver := testDeltaReceiver{}
			first := receiver.apply(t, readDeltaFrame(t, conn))
			if len(first.Animals) == 0 || first.Layout == nil {
				t.Fatal("initial websocket snapshot omitted world or animals")
			}
			// The first movement tick establishes routes for a freshly seeded world.
			receiver.apply(t, readDeltaFrame(t, conn))
			for n := 0; n < 3; n++ {
				data := readDeltaFrame(t, conn)
				upserts, _ := animalChanges(t, data)
				if mode.paths && len(upserts) != 0 {
					t.Fatalf("opt-in websocket still streams quiet animals: %s", data)
				}
				if !mode.paths && len(upserts) == 0 {
					t.Fatal("legacy or unknown negotiation omitted live animal poses")
				}
				got := receiver.apply(t, data)
				if mode.paths && got.Animals[0].Motion == nil {
					t.Fatal("opt-in websocket omitted the route descriptor")
				}
			}
		})
	}
}
