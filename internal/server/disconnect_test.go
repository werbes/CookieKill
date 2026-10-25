package server

import (
	"errors"
	"testing"
	"time"

	"cookiekill/internal/game"
	"github.com/coder/websocket"
)

func TestWebsocketReconnectResumesOnlyWithinGraceWindow(t *testing.T) {
	for _, test := range []struct {
		name   string
		lost   bool
		code   websocket.StatusCode
		resume bool
	}{
		{name: "lost connection resumes", lost: true, resume: true},
		{name: "normal close unloads", code: websocket.StatusNormalClosure},
		{name: "page navigation unloads", code: websocket.StatusGoingAway},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, srv := testServer(t)
			client := signIn(t, srv.URL, "alice@example.com")
			const resumeID = "alice-browser-tab"
			alice := dialWithResume(t, srv.URL, client, resumeID)
			first := snapshot(t, alice, nil)
			bob := dial(t, srv.URL, signIn(t, srv.URL, "bob@example.com"))
			snapshot(t, bob, func(s game.Snapshot) bool { return len(s.Players) == 1 })

			send(t, alice, map[string]any{"type": "input", "yaw": 1.25, "pitch": .5})
			send(t, alice, map[string]any{"type": "action", "action": "move_item", "from": "hotbar", "fromSlot": 0, "to": "bag", "toSlot": 0})
			snapshot(t, alice, func(s game.Snapshot) bool {
				return s.Me.Yaw == 1.25 && s.Me.BagSlots[0].Count == 10
			})
			beforeClose := snapshot(t, alice, nil)
			if test.lost {
				alice.CloseNow()
			} else {
				alice.Close(test.code, "Leaving")
			}

			var detached ownerView
			deadline := time.Now().Add(2 * time.Second)
			for {
				detached = inspectTestOwner(t, s, first.Me.ID)
				if detached.players == 1 && (detached.snapshot.Me.ID == first.Me.ID) == test.resume {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("connection did not detach: peers=%d runtime=%q", detached.players, detached.snapshot.Me.ID)
				}
				time.Sleep(time.Millisecond)
			}
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

			reconnected := dialWithResume(t, srv.URL, client, resumeID)
			restored := snapshot(t, reconnected, nil)
			if restored.Me.ID != first.Me.ID || restored.Me.BagSlots[0].Count != 10 {
				t.Fatal("reconnect lost account progress")
			}
			if restored.Layout == nil {
				t.Fatal("reconnect did not receive a complete keyframe")
			}
			if test.resume {
				if restored.Me.X != beforeClose.Me.X || restored.Me.Z != beforeClose.Me.Z || restored.Me.Yaw != 1.25 || restored.Me.Pitch != .5 {
					t.Fatal("reconnect did not resume the interrupted runtime")
				}
				if restored.Time < beforeClose.Time {
					t.Fatal("resumed keyframe moved simulation time backwards")
				}
			} else if restored.Me.Yaw != 0 || restored.Me.Pitch != 0 || restored.Me.X == beforeClose.Me.X {
				t.Fatal("normal close restored transient session state")
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

func TestExpiredReconnectRuntimeUnloads(t *testing.T) {
	world := game.New(nil)
	world.Join("alice", "Alice")
	world.Disconnect("alice")
	c := &coordinator{
		world:     world,
		peers:     make(map[string]*peer),
		suspended: map[string]suspendedSession{"alice": {resumeID: "tab", until: time.Now().Add(-time.Second)}},
	}
	c.expireSuspended(time.Now())
	if got := world.Snapshot("alice").Me.ID; got != "" {
		t.Fatalf("expired runtime was retained: %q", got)
	}
	if len(c.suspended) != 0 {
		t.Fatal("expired reconnect deadline was retained")
	}
}

func TestOnlyTransportFailuresUseReconnectGrace(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "dropped transport", err: errors.New("connection reset"), want: true},
		{name: "abnormal closure", err: websocket.CloseError{Code: websocket.StatusAbnormalClosure}, want: true},
		{name: "delta resync", err: websocket.CloseError{Code: websocket.StatusCode(4000)}, want: true},
		{name: "network offline", err: websocket.CloseError{Code: websocket.StatusCode(4002)}, want: true},
		{name: "normal close", err: websocket.CloseError{Code: websocket.StatusNormalClosure}},
		{name: "page navigation", err: websocket.CloseError{Code: websocket.StatusGoingAway}},
		{name: "policy close", err: websocket.CloseError{Code: websocket.StatusPolicyViolation}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := resumableDisconnect(test.err); got != test.want {
				t.Fatalf("resumableDisconnect(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}
