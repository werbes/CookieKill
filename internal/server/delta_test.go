package server

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"cookiekill/internal/game"
	"github.com/coder/websocket"
)

// The test receiver deliberately works from the wire format, independently of
// the encoder, so a successful reconstruction verifies the protocol contract.
type testDeltaReceiver struct {
	state map[string]json.RawMessage
	seq   uint64
}

func (r *testDeltaReceiver) apply(t *testing.T, data []byte) game.Snapshot {
	t.Helper()
	var frame map[string]json.RawMessage
	decodeDeltaTest(t, data, &frame)
	var kind string
	var seq uint64
	decodeDeltaTest(t, frame["type"], &kind)
	decodeDeltaTest(t, frame["seq"], &seq)
	switch kind {
	case "snapshot":
		if seq != 1 {
			t.Fatalf("initial sequence = %d, want 1", seq)
		}
		r.state = frame
		delete(r.state, "type")
		delete(r.state, "seq")
	case "delta":
		var base uint64
		decodeDeltaTest(t, frame["base"], &base)
		if r.state == nil || base != r.seq || seq != r.seq+1 {
			t.Fatalf("invalid delta chain: receiver=%d, base=%d, seq=%d", r.seq, base, seq)
		}
		for key, value := range frame {
			switch key {
			case "type", "seq", "base":
			case "me":
				var current, changes map[string]json.RawMessage
				decodeDeltaTest(t, r.state[key], &current)
				decodeDeltaTest(t, value, &changes)
				for field, replacement := range changes {
					current[field] = replacement
				}
				r.state[key] = marshalDeltaTest(t, current)
			case "players", "nodes", "animals", "projectiles", "homes":
				identity := "id"
				if key == "homes" {
					identity = "owner"
				}
				var current []map[string]json.RawMessage
				decodeDeltaTest(t, r.state[key], &current)
				var changes struct {
					Upsert []map[string]json.RawMessage `json:"upsert"`
					Remove []string                     `json:"remove"`
				}
				decodeDeltaTest(t, value, &changes)
				indexed := make(map[string]map[string]json.RawMessage)
				for _, entity := range current {
					var id string
					decodeDeltaTest(t, entity[identity], &id)
					indexed[id] = entity
				}
				for _, id := range changes.Remove {
					delete(indexed, id)
				}
				for _, entity := range changes.Upsert {
					var id string
					decodeDeltaTest(t, entity[identity], &id)
					if indexed[id] == nil {
						indexed[id] = make(map[string]json.RawMessage)
					}
					for field, replacement := range entity {
						indexed[id][field] = replacement
					}
				}
				current = make([]map[string]json.RawMessage, 0, len(indexed))
				for _, entity := range indexed {
					current = append(current, entity)
				}
				r.state[key] = marshalDeltaTest(t, current)
			default:
				r.state[key] = value
			}
		}
	default:
		t.Fatalf("unexpected state message %q", kind)
	}
	r.seq = seq
	var snapshot game.Snapshot
	decodeDeltaTest(t, marshalDeltaTest(t, r.state), &snapshot)
	return snapshot
}

func decodeDeltaTest(t *testing.T, data []byte, value any) {
	t.Helper()
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatal(err)
	}
}

func marshalDeltaTest(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func encodeDeltaTest(t *testing.T, encoder *stateEncoder, snapshot game.Snapshot) []byte {
	t.Helper()
	data, err := encoder.encode(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertDeltaSnapshot(t *testing.T, got, want game.Snapshot) {
	t.Helper()
	normalize := func(snapshot game.Snapshot) map[string]any {
		var value map[string]any
		decodeDeltaTest(t, marshalDeltaTest(t, snapshot), &value)
		for _, key := range []string{"players", "nodes", "animals", "projectiles", "homes"} {
			entities, _ := value[key].([]any)
			if entities == nil {
				entities = []any{}
			}
			identity := "id"
			if key == "homes" {
				identity = "owner"
			}
			sort.Slice(entities, func(i, j int) bool {
				return entities[i].(map[string]any)[identity].(string) < entities[j].(map[string]any)[identity].(string)
			})
			value[key] = entities
		}
		return value
	}
	a, b := normalize(got), normalize(want)
	for key, expected := range b {
		if !reflect.DeepEqual(a[key], expected) {
			t.Errorf("reconstructed %s differs:\n got %s\nwant %s", key, marshalDeltaTest(t, a[key]), marshalDeltaTest(t, expected))
		}
	}
}

func deltaFixture() game.Snapshot {
	home := game.Home{Owner: "alice", Kind: "cabin", X: 10, Z: 20, Safe: true, Health: 50, MaxHealth: 50}
	return game.Snapshot{
		Time: 1, ServerTime: 100,
		Me: game.Self{Profile: game.Profile{
			ID: "alice", Name: "Alice", Coins: 7, Bakery: true, Home: &home,
			Inventory: map[string]int{"sugar": 2, "wood": 3}, Discovered: map[string]bool{"sugar": true},
			Hotbar: [5]game.Stack{{Item: "sugar", Count: 2}},
		}, X: 1, Health: 50, Stamina: 60, Buffs: map[string]float64{"speed": 2}},
		Players:     []game.PlayerView{{ID: "bob", Name: "Bob", X: 3, Health: 100, Buffs: map[string]float64{"speed": 2}}},
		Nodes:       []game.Node{{ID: "bakery", Kind: "bakery_plot", Available: true, Owner: "alice", Paint: map[string]string{"wall": "red"}, Equipment: map[string]bool{"oven": true}}, {ID: "gone", Kind: "fire"}},
		Animals:     []game.Animal{{ID: "fish", Species: "fish", X: 4, Health: 22, MaxHealth: 22, Need: "trapped", Disposition: "friendly"}, {ID: "gone", Species: "shark"}},
		Projectiles: []game.Projectile{{ID: "shot", Owner: "alice", X: 3, VX: 4}},
		Homes:       []game.Home{home},
		Events:      []game.Event{{ID: 1, Actor: "alice", Kind: "craft", Text: "private recipe"}},
		Recipes:     []game.Recipe{{ID: "sugar", Name: "Sugar cookie", Known: true, Cost: map[string]int{"dough": 1}}},
		Layout:      &game.Layout{Version: 2},
	}
}

func TestDeltaReconstructsChangesRemovalsAndClearedValues(t *testing.T) {
	first := deltaFixture()
	encoder := stateEncoder{delta: true}
	receiver := testDeltaReceiver{}
	initial := encodeDeltaTest(t, &encoder, first)
	// Encoding an unsent frame must not advance the client's baseline.
	if retry := encodeDeltaTest(t, &encoder, first); !bytes.Equal(initial, retry) {
		t.Fatal("encoding before commit changed the initial frame")
	}
	assertDeltaSnapshot(t, receiver.apply(t, initial), first)
	encoder.commit(first)
	var next game.Snapshot
	decodeDeltaTest(t, marshalDeltaTest(t, first), &next)
	next.Time, next.ServerTime = 2, 101
	next.Me.Coins, next.Me.Bakery, next.Me.Home = 0, false, nil
	next.Me.X, next.Me.Health, next.Me.Stamina = 0, 0, 0
	next.Me.Inventory = map[string]int{"sugar": 1}
	next.Me.Discovered = map[string]bool{}
	next.Me.Buffs = nil
	next.Me.Hotbar = [5]game.Stack{}
	next.Players[0].X = 0
	next.Players[0].Buffs = map[string]float64{}
	next.Players = append(next.Players, game.PlayerView{ID: "carol", Name: "Carol", Health: 100})
	next.Nodes = next.Nodes[:1]
	next.Nodes[0].Available = false
	next.Nodes[0].Owner, next.Nodes[0].Paint, next.Nodes[0].Equipment = "", nil, nil
	next.Nodes = append(next.Nodes, game.Node{ID: "new", Kind: "fire", Available: true})
	next.Animals = next.Animals[:1]
	next.Animals[0].X, next.Animals[0].Health = 0, 0
	next.Animals[0].Z, next.Animals[0].Heading = 1.23456789012345, -2.34567890123456
	next.Animals[0].Need, next.Animals[0].Disposition = "", ""
	next.Projectiles = []game.Projectile{}
	next.Homes = []game.Home{{Owner: "bob", Kind: "hut", X: 4, Health: 20}}
	next.Events = []game.Event{}
	next.Recipes = []game.Recipe{}
	update := encodeDeltaTest(t, &encoder, next)
	assertDeltaSnapshot(t, receiver.apply(t, update), next)
	encoder.commit(next)
	var unchanged map[string]json.RawMessage
	data := encodeDeltaTest(t, &encoder, next)
	decodeDeltaTest(t, data, &unchanged)
	for _, key := range []string{"me", "players", "nodes", "animals", "projectiles", "homes", "events", "recipes", "layout"} {
		if _, repeated := unchanged[key]; repeated {
			t.Errorf("unchanged %s was retransmitted", key)
		}
	}
	if unchanged["time"] == nil || unchanged["serverTime"] == nil {
		t.Fatal("delta omitted simulation or wall time")
	}
	assertDeltaSnapshot(t, receiver.apply(t, data), next)
	encoder.commit(next)
	var third game.Snapshot
	decodeDeltaTest(t, marshalDeltaTest(t, next), &third)
	third.Players = []game.PlayerView{}
	third.Animals = []game.Animal{}
	third.Homes[0].X, third.Homes[0].Health = 0, 0
	third.Projectiles = []game.Projectile{{ID: "new-shot", Owner: "bob", X: 1, VX: 4}}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, &encoder, third)), third)
	encoder.commit(third)
	var fourth game.Snapshot
	decodeDeltaTest(t, marshalDeltaTest(t, third), &fourth)
	fourth.Projectiles[0].X, fourth.Projectiles[0].VX = 2, 0
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, &encoder, fourth)), fourth)
}

func TestDeltaRecipientsHaveIndependentBaselines(t *testing.T) {
	alice := deltaFixture()
	var bob game.Snapshot
	decodeDeltaTest(t, marshalDeltaTest(t, alice), &bob)
	bob.Me.ID, bob.Me.Name, bob.Me.Coins = "bob", "Bob", 99
	bob.Animals[0].Disposition = "hostile"
	bob.Events = []game.Event{}
	bob.Recipes[0].Known, bob.Recipes[0].Name = false, "Undiscovered recipe"
	a, b := stateEncoder{delta: true}, stateEncoder{delta: true}
	ar, br := testDeltaReceiver{}, testDeltaReceiver{}
	ar.apply(t, encodeDeltaTest(t, &a, alice))
	br.apply(t, encodeDeltaTest(t, &b, bob))
	a.commit(alice)
	b.commit(bob)
	alice.Time, bob.Time = 2, 2
	alice.Me.Coins = 8
	assertDeltaSnapshot(t, ar.apply(t, encodeDeltaTest(t, &a, alice)), alice)
	assertDeltaSnapshot(t, br.apply(t, encodeDeltaTest(t, &b, bob)), bob)
	// A fresh connection must receive its own complete baseline and layout.
	reconnect := stateEncoder{delta: true}
	rr := testDeltaReceiver{}
	assertDeltaSnapshot(t, rr.apply(t, encodeDeltaTest(t, &reconnect, bob)), bob)
}

func TestDeltaQueueDropsKeepLastWrittenBaseline(t *testing.T) {
	w := game.New(nil)
	w.Join("alice", "Alice")
	s := &Server{world: w, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	p := &peer{out: make(chan outbound, 2)}
	encoder := stateEncoder{delta: true}
	receiver := testDeltaReceiver{}
	s.enqueueSnapshot("alice", p)
	initial := <-p.out
	if initial.snapshot == nil {
		t.Fatal("snapshot was encoded before reaching the writer")
	}
	receiver.apply(t, encodeDeltaTest(t, &encoder, *initial.snapshot))
	encoder.commit(*initial.snapshot)
	if err := w.Act("alice", game.Action{Action: "move_item", From: "hotbar", FromSlot: 0, To: "safe", ToSlot: 0}); err != nil {
		t.Fatal(err)
	}
	w.Input("alice", game.Input{X: 1})
	for n := 0; n < 5; n++ {
		w.Tick(.1)
		s.enqueueSnapshot("alice", p)
	}
	if len(p.out) != cap(p.out) {
		t.Fatal("queue did not remain bounded")
	}
	// Drain a stale frame without writing it, as can happen when coalescing a
	// backlog. Neither queued nor discarded frames can become delta baselines.
	<-p.out
	latest := <-p.out
	if initial.snapshot.Me.SafeSlots[0].Count != 0 || initial.snapshot.Me.Hotbar[0].Count != 10 {
		t.Fatal("queued snapshot changed when the authoritative world changed")
	}
	got := receiver.apply(t, encodeDeltaTest(t, &encoder, *latest.snapshot))
	assertDeltaSnapshot(t, got, w.Snapshot("alice"))
	if got.Me.SafeSlots[0].Count != 10 || receiver.seq != 2 {
		t.Fatal("discarded queued state corrupted the delivered update")
	}
}

func TestLegacyStateEncoderRetainsFullSnapshots(t *testing.T) {
	encoder := stateEncoder{}
	first := deltaFixture()
	for n := 0; n < 2; n++ {
		var frame map[string]json.RawMessage
		decodeDeltaTest(t, encodeDeltaTest(t, &encoder, first), &frame)
		if string(frame["type"]) != `"snapshot"` || frame["seq"] != nil || frame["base"] != nil || frame["nodes"] == nil || frame["animals"] == nil || frame["me"] == nil {
			t.Fatal("legacy client did not receive an unsequenced full snapshot")
		}
		if (frame["layout"] != nil) != (n == 0) {
			t.Fatal("legacy layout must be sent only with the first successful frame")
		}
		encoder.commit(first)
	}
}

func readDeltaFrame(t *testing.T, c *websocket.Conn) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	kind, data, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if kind != websocket.MessageText {
		t.Fatal("state frame is not JSON text")
	}
	return data
}

func TestDeltaWebsocketNegotiationAndReconnect(t *testing.T) {
	for _, mode := range []websocket.CompressionMode{websocket.CompressionDisabled, websocket.CompressionNoContextTakeover} {
		name := "uncompressed"
		if mode != websocket.CompressionDisabled {
			name = "deflate"
		}
		t.Run(name, func(t *testing.T) {
			_, srv := testServer(t)
			client := signIn(t, srv.URL, "delta@example.com")
			connect := func() *websocket.Conn {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				c, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?updates=delta-v1", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": {srv.URL}}, CompressionMode: mode})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { c.CloseNow() })
				c.SetReadLimit(2 << 20)
				compressed := strings.Contains(response.Header.Get("Sec-WebSocket-Extensions"), "permessage-deflate")
				if compressed != (mode != websocket.CompressionDisabled) {
					t.Fatalf("unexpected compression negotiation: %q", response.Header.Get("Sec-WebSocket-Extensions"))
				}
				return c
			}
			c := connect()
			receiver := testDeltaReceiver{}
			first := receiver.apply(t, readDeltaFrame(t, c))
			if first.Layout == nil || len(first.Nodes) == 0 || len(first.Animals) == 0 {
				t.Fatal("opt-in client missing initial world state")
			}
			send(t, c, map[string]any{"type": "action", "action": "move_item", "from": "hotbar", "fromSlot": 0, "to": "safe", "toSlot": 0})
			deadline := time.Now().Add(3 * time.Second)
			for {
				next := receiver.apply(t, readDeltaFrame(t, c))
				if next.Me.SafeSlots[0].Count == 10 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("action was not delivered through delta updates")
				}
			}
			// Error messages share the output queue but must not advance the
			// state sequence or become the baseline for the next delta.
			send(t, c, map[string]string{"type": "invalid-test-message"})
			for {
				data := readDeltaFrame(t, c)
				var envelope struct {
					Type string `json:"type"`
				}
				decodeDeltaTest(t, data, &envelope)
				if envelope.Type == "error" {
					break
				}
				receiver.apply(t, data)
				if time.Now().After(deadline) {
					t.Fatal("server did not report the invalid message")
				}
			}
			receiver.apply(t, readDeltaFrame(t, c))
			c.CloseNow()
			reconnected := connect()
			fresh := testDeltaReceiver{}
			restored := fresh.apply(t, readDeltaFrame(t, reconnected))
			if restored.Layout == nil || restored.Me.ID != first.Me.ID || restored.Me.SafeSlots[0].Count != 10 {
				t.Fatal("reconnection did not reset the transport baseline and preserve progress")
			}
		})
	}
}

func TestDeltaSteadyStateBandwidth(t *testing.T) {
	w := game.New(nil)
	w.Join("bandwidth", "Bandwidth test")
	encoder := stateEncoder{delta: true}
	first := w.Snapshot("bandwidth")
	encoder.commit(first)
	var compressed bytes.Buffer
	// coder/websocket v1.8.15 uses flate.BestSpeed. Flush and discard the
	// four-byte tail to reproduce a permessage-deflate message without takeover.
	writer, err := flate.NewWriter(&compressed, flate.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	var fullBytes, deltaBytes, deflatedBytes int
	const frames = 100
	for n := 0; n < frames; n++ {
		w.Tick(.05)
		w.Tick(.05)
		snapshot := w.Snapshot("bandwidth")
		delta := encodeDeltaTest(t, &encoder, snapshot)
		encoder.commit(snapshot)
		snapshot.Layout = nil
		full := marshalDeltaTest(t, struct {
			Type string `json:"type"`
			game.Snapshot
		}{"snapshot", snapshot})
		fullBytes += len(full)
		deltaBytes += len(delta)
		compressed.Reset()
		writer.Reset(&compressed)
		if _, err := writer.Write(delta); err != nil {
			t.Fatal(err)
		}
		if err := writer.Flush(); err != nil {
			t.Fatal(err)
		}
		deflatedBytes += compressed.Len() - 4
	}
	t.Logf("%d simulated updates: full %.0f B/update, delta %.0f B/update (%.1f%% reduction), deflated delta %.0f B/update (%.1f%% reduction)", frames, float64(fullBytes)/frames, float64(deltaBytes)/frames, 100*(1-float64(deltaBytes)/float64(fullBytes)), float64(deflatedBytes)/frames, 100*(1-float64(deflatedBytes)/float64(fullBytes)))
	if deltaBytes >= fullBytes/3 {
		t.Fatalf("delta payloads exceed one third of complete snapshot payloads: %d / %d", deltaBytes, fullBytes)
	}
	if deflatedBytes >= fullBytes/6 {
		t.Fatalf("compressed delta payloads exceed one sixth of complete snapshot payloads: %d / %d", deflatedBytes, fullBytes)
	}
}

// Snapshot construction is deliberately outside the timed section: this
// measures the transport encoder's CPU and allocation cost for the same world.
func BenchmarkStateEncoder(b *testing.B) {
	for _, players := range []int{1, 64} {
		w := game.New(nil)
		for n := 0; n < players; n++ {
			w.Join(fmt.Sprintf("player_%02d", n), fmt.Sprintf("Player %d", n))
		}
		for n := 0; n < 200; n++ {
			w.Tick(.05)
		}
		first := w.Snapshot("player_00")
		for n := 0; n < players; n++ {
			w.Input(fmt.Sprintf("player_%02d", n), game.Input{X: 1, Sprint: true})
		}
		w.Tick(.05)
		w.Tick(.05)
		second := w.Snapshot("player_00")
		for _, mode := range []struct {
			name  string
			delta bool
		}{{"legacy", false}, {"delta", true}} {
			b.Run(fmt.Sprintf("players=%d/%s", players, mode.name), func(b *testing.B) {
				encoder := stateEncoder{delta: mode.delta}
				encoder.commit(first)
				b.ReportAllocs()
				b.ResetTimer()
				bytes := 0
				for n := 0; n < b.N; n++ {
					snapshot := second
					if n%2 == 1 {
						snapshot = first
					}
					data, err := encoder.encode(snapshot)
					if err != nil {
						b.Fatal(err)
					}
					bytes += len(data)
					encoder.commit(snapshot)
				}
				b.StopTimer()
				b.ReportMetric(float64(bytes)/float64(b.N), "payload-B/op")
			})
		}
	}
}
