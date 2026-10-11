package server

import (
	"testing"

	"cookiekill/internal/game"
	"github.com/coder/websocket"
)

func TestWebsocketDisconnectUnloadsPlayerAndReconnectRestoresOnlyProgress(t *testing.T) {
	for _, graceful := range []bool{true, false} {
		name := "lost connection"
		if graceful {
			name = "browser close"
		}
		t.Run(name, func(t *testing.T) {
			s, srv := testServer(t)
			client := signIn(t, srv.URL, "alice@example.com")
			alice := dial(t, srv.URL, client)
			first := snapshot(t, alice, nil)
			bob := dial(t, srv.URL, signIn(t, srv.URL, "bob@example.com"))
			snapshot(t, bob, func(s game.Snapshot) bool { return len(s.Players) == 1 })

			send(t, alice, map[string]any{"type": "input", "yaw": 1.25, "pitch": .5})
			send(t, alice, map[string]any{"type": "action", "action": "move_item", "from": "hotbar", "fromSlot": 0, "to": "bag", "toSlot": 0})
			snapshot(t, alice, func(s game.Snapshot) bool {
				return s.Me.Yaw == 1.25 && s.Me.BagSlots[0].Count == 10
			})
			if graceful {
				alice.Close(websocket.StatusNormalClosure, "Leaving")
			} else {
				alice.CloseNow()
			}

			// Detach must unload immediately, without a simulation grace period.
			waitCoordinator(t, "disconnected player was retained", func() bool {
				view := inspectTestOwner(t, s, first.Me.ID)
				return view.players == 1 && view.snapshot.Me.ID == ""
			})
			snapshot(t, bob, func(s game.Snapshot) bool { return len(s.Players) == 0 })
			if err := s.Save(); err != nil {
				t.Fatal(err)
			}
			profiles, err := s.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if profiles[first.Me.ID].BagSlots[0].Count != 10 {
				t.Fatal("unloading lost saved inventory")
			}

			reconnected := dial(t, srv.URL, client)
			restored := snapshot(t, reconnected, nil)
			if restored.Me.ID != first.Me.ID || restored.Me.BagSlots[0].Count != 10 {
				t.Fatal("reconnect lost account progress")
			}
			if restored.Me.Yaw != 0 || restored.Me.Pitch != 0 || restored.Layout == nil {
				t.Fatal("reconnect reused transient state instead of a fresh session")
			}
			send(t, reconnected, map[string]any{"type": "input", "x": 1})
			moved := snapshot(t, reconnected, func(s game.Snapshot) bool { return s.Me.X > restored.Me.X })
			if moved.Time <= restored.Time {
				t.Fatal("reconnected simulation did not advance")
			}
			if view := inspectTestOwner(t, s, first.Me.ID); view.players != 2 {
				t.Fatalf("reconnect created an extra runtime player: players=%d", view.players)
			}
		})
	}
}
