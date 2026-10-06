package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"cookiekill/internal/auth"
	"cookiekill/internal/game"
	"cookiekill/internal/store"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func testServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	a, err := auth.New(auth.Config{DevMode: true, SessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(a, db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	httpServer := httptest.NewServer(s.Handler(fstest.MapFS{"index.html": {Data: []byte("CookieKill")}}, false))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done; s.Close(); httpServer.Close() })
	return s, httpServer
}

func signIn(t *testing.T, base, email string) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	res, err := client.Post(base+"/api/login", "application/json", strings.NewReader(`{"email":"`+email+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	var challenge struct {
		DevCode string `json:"devCode"`
	}
	json.NewDecoder(res.Body).Decode(&challenge)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || challenge.DevCode == "" {
		t.Fatalf("login failed: %d %+v", res.StatusCode, challenge)
	}
	b, _ := json.Marshal(map[string]string{"email": email, "code": challenge.DevCode})
	res, err = client.Post(base+"/api/verify", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("verify: %s", body)
	}
	return client
}

func dial(t *testing.T, base string, client *http.Client) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(base, "http")+"/ws", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": {base}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	c.SetReadLimit(2 << 20)
	return c
}

func snapshot(t *testing.T, c *websocket.Conn, predicate func(game.Snapshot) bool) game.Snapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		var snap struct {
			Type string `json:"type"`
			game.Snapshot
		}
		if err := wsjson.Read(ctx, c, &snap); err != nil {
			t.Fatal(err)
		}
		if snap.Type == "snapshot" && (predicate == nil || predicate(snap.Snapshot)) {
			return snap.Snapshot
		}
	}
}

func send(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, c, v); err != nil {
		t.Fatal(err)
	}
}

func TestAnonymousAndCrossOriginWebsocketRejected(t *testing.T) {
	_, srv := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, res, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err == nil || res == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous accepted: %v", err)
	}
	client := signIn(t, srv.URL, "alice@example.com")
	_, res, err = websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": {"https://evil.example"}}})
	if err == nil || res == nil || res.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin accepted: %v", err)
	}
}

func TestMultiplayerMovementProtectionPersistenceAndLogout(t *testing.T) {
	s, srv := testServer(t)
	aliceHTTP := signIn(t, srv.URL, "alice@example.com")
	alice := dial(t, srv.URL, aliceHTTP)
	first := snapshot(t, alice, nil)
	if first.Me.Inventory["sugar"] != 10 || first.Me.Area != "forest" {
		t.Fatalf("incorrect start %+v", first.Me)
	}
	if first.Layout == nil || len(first.Layout.Plots) != 12 || len(first.Layout.Coast) < 50 {
		t.Fatal("first multiplayer snapshot is missing the shared world map")
	}
	if len(first.Recipes) == 0 || first.Recipes[0].Name == "Sugar cookie" || first.Recipes[0].Damage != 0 {
		t.Fatal("undiscovered recipe details leaked over the network")
	}
	bob := dial(t, srv.URL, signIn(t, srv.URL, "bob@example.com"))
	snapshot(t, bob, func(s game.Snapshot) bool { return len(s.Players) == 1 })
	send(t, alice, map[string]any{"type": "input", "x": 1, "z": 0, "yaw": 0, "pitch": 0})
	send(t, alice, map[string]any{"type": "action", "action": "move_item", "from": "hotbar", "fromSlot": 0, "to": "safe", "toSlot": 0})
	after := snapshot(t, alice, func(s game.Snapshot) bool { return s.Me.X > first.Me.X && s.Me.SafeSlots[0].Item == "sugar" })
	if after.Me.X-first.Me.X > 10 {
		t.Fatal("movement teleported")
	}
	if after.Me.SafeSlots[0].Count != 10 || len(after.Me.SafeSlots) != 3 || len(after.Me.BagSlots) != 15 || len(after.Me.Hotbar) != 5 {
		t.Fatal("protection action not applied")
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	saved, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved[after.Me.ID].SafeSlots[0].Count != 10 {
		t.Fatal("progress not persisted")
	}
	res, err := aliceHTTP.Post(srv.URL+"/api/logout", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	send(t, alice, map[string]string{"type": "ping"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		if _, _, err := alice.Read(ctx); err != nil {
			if ctx.Err() != nil {
				t.Fatal("logout failed to invalidate game connection")
			}
			break
		}
	}
}

func TestDuplicateSessionReplacesConnection(t *testing.T) {
	_, srv := testServer(t)
	client := signIn(t, srv.URL, "alice@example.com")
	one := dial(t, srv.URL, client)
	first := snapshot(t, one, nil)
	two := dial(t, srv.URL, client)
	second := snapshot(t, two, nil)
	if second.Me.ID != first.Me.ID || len(second.Players) != 0 {
		t.Fatal("duplicate player created")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		if _, _, err := one.Read(ctx); err != nil {
			if ctx.Err() != nil {
				t.Fatal("stale connection survived")
			}
			if websocket.CloseStatus(err) != websocket.StatusCode(4001) {
				t.Fatalf("missing replacement close code: %v", err)
			}
			break
		}
	}
}

func TestSecurityHeadersAndStaticClient(t *testing.T) {
	_, srv := testServer(t)
	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if string(body) != "CookieKill" || res.Header.Get("Content-Security-Policy") == "" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("client/security headers missing")
	}
}

func TestPublicWorldLayoutContainsSceneryWithoutPlayerProfiles(t *testing.T) {
	_, srv := testServer(t)
	res, err := http.Get(srv.URL + "/api/world")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var payload map[string]json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 1 || payload["layout"] == nil {
		t.Fatal("world preview exposed dynamic player data")
	}
	var layout game.Layout
	if err := json.Unmarshal(payload["layout"], &layout); err != nil {
		t.Fatal(err)
	}
	if layout.Version != 3 || len(layout.Plots) != 12 || len(layout.Zones) != 0 || len(layout.Props) < 200 {
		t.Fatal("public preview lacks new world")
	}
}
