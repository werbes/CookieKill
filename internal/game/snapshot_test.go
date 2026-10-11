package game

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPlayerSnapshotShowsOnlyOccupiedSelectedHeldItem(t *testing.T) {
	w, p := testWorld()
	w.Join("visitor", "Visitor")
	p.profile.Hotbar[1] = Stack{Item: "bouquet_daisies", Count: 3}
	p.profile.SelectedSlot = 1
	s := w.Snapshot("visitor")
	if len(s.Players) != 1 || s.Players[0].HeldItem != "bouquet_daisies" {
		t.Fatal("selected bouquet missing from other player's view")
	}
	encoded, err := json.Marshal(s.Players[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"inventory", "hotbar", "bagSlots", "selectedSlot"} {
		if strings.Contains(string(encoded), `"`+private+`"`) {
			t.Fatalf("held item leaked %s", private)
		}
	}
	p.profile.SelectedSlot = 2
	if w.Snapshot("visitor").Players[0].HeldItem != "" {
		t.Fatal("empty selected slot did not clear held item")
	}
	p.profile.SelectedSlot = 1
	p.profile.Hotbar[1].Count = 0
	if w.Snapshot("visitor").Players[0].HeldItem != "" {
		t.Fatal("depleted stack remained visible")
	}
	p.profile.SelectedSlot = -1
	if w.Snapshot("visitor").Players[0].HeldItem != "" {
		t.Fatal("invalid selected slot exposed held item")
	}
}

func TestSnapshotUpdateMatchesFullSnapshotWithoutLayout(t *testing.T) {
	for _, islands := range []bool{false, true} {
		w, ids := benchmarkWorld(16, islands)
		for _, id := range ids {
			full := w.Snapshot(id)
			if full.Layout == nil || len(full.Layout.Props) == 0 {
				t.Fatal("initial snapshot did not include the procedural map")
			}
			full.Layout = nil
			if update := w.SnapshotUpdate(id); !reflect.DeepEqual(full, update) {
				t.Fatalf("update for %s (island=%t) changed data beyond layout omission", id, islands)
			}
		}
	}
}

func TestSnapshotUpdateDetachesNestedDataAndOtherSnapshots(t *testing.T) {
	w, p := testWorld()
	w.now = func() time.Time { return time.Unix(2_000_000_000, 0) }
	w.Join("two", "Other baker")
	w.players["two"].buffs["speed"] = 100
	p.profile.Bakery, p.profile.BakeryPlot = true, "bakery_plot_0"
	p.profile.Equipment["oven"], p.profile.Paint["walls"] = true, "teal"
	p.profile.HomeIsland = &Island{Owner: "one", Builders: map[string]bool{"two": true}, Chest: map[string]int{"wood": 20}, Objects: []BuildObject{{ID: "test_chair", Kind: "chair", X: 2}}}
	p.profile.Friends["two"] = true
	p.profile.FriendRequests["two"] = true
	w.nodes = append(w.nodes, &Node{ID: "colored_fixture", Kind: "fixture", Available: true, Paint: map[string]string{"walls": "blue"}, Equipment: map[string]bool{"oven": true}})
	before, changed := w.SnapshotUpdate("one"), w.SnapshotUpdate("one")
	changed.Me.Inventory["sugar"] = 999
	changed.Me.Levels["legs"] = 10
	changed.Me.Reputation["new"] = 3
	changed.Me.Recovery["wood"] = 999
	changed.Me.Discovered["cake"] = true
	changed.Me.Hats["builder"] = false
	changed.Me.Equipment["oven"] = false
	changed.Me.Paint["walls"] = "black"
	changed.Me.Friends["two"] = false
	changed.Me.FriendRequests["two"] = false
	changed.Me.HomeIsland.Builders["two"] = false
	changed.Me.HomeIsland.Chest["wood"] = 999
	changed.Me.HomeIsland.Objects[0].X = 999
	changed.Players[0].Buffs["speed"] = 999
	changed.Recipes[0].Cost["dough"] = 999
	changed.BuildCatalog[0].Cost["wood"] = 999
	for i := range changed.Nodes {
		node := &changed.Nodes[i]
		if node.Paint != nil {
			node.Paint["walls"] = "black"
		}
		if node.Equipment != nil {
			node.Equipment["oven"] = false
		}
		node.X = 999
	}
	changed.Animals[0].Health = 1
	changed.Friends[0].CallName = "Changed only in this snapshot"
	if after := w.SnapshotUpdate("one"); !reflect.DeepEqual(before, after) {
		t.Fatal("mutating an update changed the world, shared recipes, or a previous update")
	}
	full := w.Snapshot("one")
	full.Layout.Props[0].X = 999
	if w.Snapshot("one").Layout.Props[0].X == 999 {
		t.Fatal("initial map copy aliases world layout")
	}
}

func TestSnapshotUpdatePreservesIslandAndAccountPrivacy(t *testing.T) {
	w, owner := testWorld()
	owner.profile.HomeIsland = &Island{Owner: "one", Visitors: true, Builders: map[string]bool{}, Chest: map[string]int{"wood": 20}, Objects: []BuildObject{{ID: "chair", Kind: "chair", X: 2}}}
	w.Join("visitor", "Visitor")
	w.Join("mainland", "Mainland")
	w.Join("elsewhere", "Elsewhere")
	visitor := w.players["visitor"]
	owner.island, visitor.island, w.players["elsewhere"].island = "one", "one", "elsewhere"
	owner.profile.PublicFriendCode = false
	owner.profile.Discovered["cake"] = true
	visitor.profile.Friends["one"] = true
	w.eventActor("Owner's private discovery", "craft", 0, 0, "one")
	w.eventActor("Owner's private notification", "notification", 0, 0, "one")
	w.eventActor("Visitor's notification", "notification", 0, 0, "visitor")
	view := w.SnapshotUpdate("visitor")
	if len(view.Players) != 1 || view.Players[0].ID != "one" || view.Players[0].FriendCode != "" {
		t.Fatal("update leaked another island's player or a private friend code")
	}
	if len(view.Nodes) != 2 || view.Nodes[0].Kind != "island_portal" || view.Nodes[1].Kind != "island_upgrade" || len(view.Animals) != 0 || len(view.Projectiles) != 0 {
		t.Fatal("island update included mainland entities")
	}
	if view.Island == nil || len(view.Island.Chest) != 0 || len(view.Island.Builders) != 0 || len(view.Island.Objects) != 1 {
		t.Fatal("visitor cannot see furniture or can see private storage/permissions")
	}
	if len(view.Events) != 2 { // Their own message plus the scheduled-race reminder.
		t.Fatal("island update dropped a private notification or race reminder")
	}
	for _, event := range view.Events {
		if event.Actor != "visitor" {
			t.Fatal("island update leaked another account's notifications or discovery")
		}
	}
	for _, recipe := range view.Recipes {
		if recipe.ID == "cake" && recipe.Known {
			t.Fatal("another baker's discovery became visible")
		}
	}
	owner.profile.PublicFriendCode = true
	owner.profile.HomeIsland.Builders["visitor"] = true
	allowed := w.SnapshotUpdate("visitor")
	if allowed.Players[0].FriendCode != owner.profile.FriendCode || allowed.Island.Chest["wood"] != 20 {
		t.Fatal("public code or granted chest access disappeared")
	}
	allowed.Island.Chest["wood"] = 999
	allowed.Island.Objects[0].X = 999
	if owner.profile.HomeIsland.Chest["wood"] != 20 || owner.profile.HomeIsland.Objects[0].X == 999 {
		t.Fatal("visitor snapshot aliases the owner's island")
	}
}

func TestFriendViewsResolveOnlyRelationshipsWithCurrentProfilePrecedence(t *testing.T) {
	w, p := testWorld()
	w.profiles["friend"] = sanitize(Profile{ID: "friend", Name: "Saved friend"})
	w.profiles["request"] = sanitize(Profile{ID: "request", Name: "Saved request"})
	w.Join("friend", "Saved friend")
	w.players["friend"].profile.CallName = "Online friend"
	w.Join("request", "Saved request")
	w.players["request"].profile.CallName = "Offline request"
	w.Leave("request")
	p.profile.Friends = map[string]bool{"friend": true, "missing": true, "ignored": false}
	p.profile.FriendRequests = map[string]bool{"friend": true, "request": true, "ignored": false}
	views := w.friendViews(p)
	if len(views) != 2 || views[0].ID != "friend" || views[0].CallName != "Online friend" || !views[0].Request || !views[0].Online || views[1].ID != "request" || views[1].CallName != "Offline request" || views[1].Online {
		t.Fatalf("relationship lookup lost ordering, uniqueness, or live profile precedence: %+v", views)
	}
}
