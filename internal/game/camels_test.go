package game

import (
	"reflect"
	"testing"
	"time"
)

func atCamelKeeper(p *player) { p.x, p.y, p.z, p.island, p.inCave = 226, 0, -126, "", false }

func TestCamelHerdPurchasesAndPersistence(t *testing.T) {
	w, p := testWorld()
	atCamelKeeper(p)
	p.profile.Coins = 2000
	for i := 0; i < 4; i++ {
		mustAct(t, w, Action{Action: "camel_buy"})
	}
	if len(p.profile.Camels) != 4 || p.profile.Coins != 600 || !p.profile.CamelOwned {
		t.Fatalf("each purchase should add a permanent camel: %+v", p.profile)
	}
	seen := map[string]bool{}
	for _, c := range p.profile.Camels {
		if c.ID == "" || seen[c.ID] || c.RentalUntil != 0 {
			t.Fatalf("camel identity is not unique and permanent: %+v", c)
		}
		seen[c.ID] = true
	}
	if len(w.camelNodes(p)) != 3 {
		t.Fatal("previous mounts should remain visible as parked camels")
	}
	saved := w.Profiles()
	restored := New(saved).Profiles()["one"]
	if !reflect.DeepEqual(restored.Camels, p.profile.Camels) {
		t.Fatalf("camel identities or parking places changed after restart: %+v", restored.Camels)
	}
	saved["one"].Camels[0].X = -200
	snapshot := w.Snapshot("one")
	snapshot.Me.Camels[0].X = -150
	if p.profile.Camels[0].X < 0 {
		t.Fatal("saved profiles or snapshots share the live herd backing array")
	}
}

func TestLegacyCamelOwnershipMigration(t *testing.T) {
	for _, old := range []Profile{
		{ID: "one", CamelOwned: true},
		{ID: "one", CamelRentalUntil: time.Now().Unix() + 1200},
	} {
		w := New(map[string]Profile{"one": old})
		profile := w.Profiles()["one"]
		if len(profile.Camels) != 1 || profile.CamelOwned != old.CamelOwned || profile.Camels[0].RentalUntil != old.CamelRentalUntil {
			t.Fatalf("legacy companion was lost: %+v", profile.Camels)
		}
		again := New(w.Profiles()).Profiles()["one"]
		if !reflect.DeepEqual(again.Camels, profile.Camels) {
			t.Fatal("reloading a migrated companion duplicated or relocated it")
		}
	}
}

func TestCamelMountRequiresOwnershipProximityAndLayer(t *testing.T) {
	w, p := testWorld()
	atCamelKeeper(p)
	p.profile.Coins = 350
	mustAct(t, w, Action{Action: "camel_buy"})
	id := p.mountedCamelID
	before := cloneMap(p.profile.Inventory)
	mustAct(t, w, Action{Action: "camel_dismount"})
	if p.ridingCamel || !reflect.DeepEqual(before, p.profile.Inventory) {
		t.Fatal("dismount should park the camel without consuming food")
	}
	c := p.profile.Camels[0]
	if distance(p.x, p.z, c.X, c.Z) > 5 || distance(p.x, p.z, c.X, c.Z) < 1 {
		t.Fatal("dismounted camel should wait nearby with space to approach it")
	}
	w.Join("two", "Other baker")
	other := w.players["two"]
	other.x, other.z = c.X, c.Z
	if err := w.Act("two", Action{Action: "camel_mount", Target: id}); err == nil {
		t.Fatal("another player mounted an owned camel")
	}
	p.x -= 20
	if err := w.Act("one", Action{Action: "camel_mount", Target: id}); err == nil {
		t.Fatal("remote camel mounting succeeded")
	}
	p.x, p.z = c.X, c.Z
	mustAct(t, w, Action{Action: "interact", Target: id})
	if !p.ridingCamel || p.mountedCamelID != id {
		t.Fatal("E interaction did not mount the selected owned camel")
	}
	p.x, p.z, p.inCave = 120, -117, true
	syncMountedCamel(p)
	mustAct(t, w, Action{Action: "camel_dismount"})
	c = p.profile.Camels[0]
	p.x, p.z, p.inCave = c.X, c.Z, false
	if err := w.Act("one", Action{Action: "camel_mount", Target: id}); err == nil {
		t.Fatal("surface player mounted an underground camel")
	}
	p.inCave = true
	mustAct(t, w, Action{Action: "camel_mount", Target: id})
}

func TestMountedCamelTravelsHomeCaveAndMainland(t *testing.T) {
	w, p := testWorld()
	buyTestIsland(t, w, p)
	atCamelKeeper(p)
	p.profile.Coins = 1050
	for range 3 {
		mustAct(t, w, Action{Action: "camel_buy"})
	}
	id := p.mountedCamelID
	parked := append([]Camel{}, p.profile.Camels[:2]...)
	assertMount := func(island string, inCave bool) {
		t.Helper()
		c := mountedCamel(p)
		if !p.ridingCamel || p.mountedCamelID != id || c == nil || c.Island != island || c.InCave != inCave || c.X != p.x || c.Z != p.z || c.Y != groundY(p) {
			t.Fatalf("mounted camel did not follow rider: %+v", c)
		}
		if !reflect.DeepEqual(parked, p.profile.Camels[:2]) {
			t.Fatal("teleport moved camels that were parked elsewhere")
		}
		for _, n := range w.Snapshot("one").Nodes {
			if n.ID == id {
				t.Fatal("mounted camel was duplicated as a parked node")
			}
		}
	}
	mustAct(t, w, Action{Action: "teleport_home"})
	assertMount("one", false)
	p.x, p.z = islandPortalX, islandPortalZ
	mustAct(t, w, Action{Action: "teleport_cave"})
	assertMount("", true)
	mustAct(t, w, Action{Action: "teleport_home"})
	assertMount("one", false)
	mustAct(t, w, Action{Action: "teleport_main"})
	assertMount("", false)
	mustAct(t, w, Action{Action: "teleport_home"})
	mustAct(t, w, Action{Action: "camel_dismount"})
	if len(w.camelNodes(p)) != 1 {
		t.Fatal("only the camel brought home should be present on that island")
	}
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	if !reflect.DeepEqual(restored.players["one"].profile.Camels, p.profile.Camels) {
		t.Fatal("home/mainland herd positions did not survive restart")
	}
}

func TestCamelRentalExpiryDoesNotAffectOwnedHerd(t *testing.T) {
	w, p := testWorld()
	now := time.Now()
	w.now = func() time.Time { return now }
	atCamelKeeper(p)
	p.profile.Coins = 1000
	mustAct(t, w, Action{Action: "camel_buy"})
	ownedID := p.mountedCamelID
	mustAct(t, w, Action{Action: "camel_rent"})
	if len(p.profile.Camels) != 2 || p.mountedCamelID == ownedID {
		t.Fatal("rental should coexist with a permanent herd")
	}
	now = now.Add(20 * time.Minute)
	w.Tick(.05)
	if p.ridingCamel || len(p.profile.Camels) != 1 || p.profile.Camels[0].ID != ownedID || !p.profile.CamelOwned || p.profile.CamelRentalUntil != 0 {
		t.Fatal("expired rental either remained mounted or removed an owned camel")
	}
	mustAct(t, w, Action{Action: "camel_mount", Target: ownedID})
	now = now.Add(time.Hour)
	w.Tick(.05)
	if !p.ridingCamel || p.mountedCamelID != ownedID {
		t.Fatal("permanently owned camel expired")
	}
}

func TestCamelMountCannotCrossIslandWall(t *testing.T) {
	w, p := testWorld()
	buyTestIsland(t, w, p)
	mustAct(t, w, Action{Action: "teleport_home"})
	p.x, p.z = 0, 0
	p.profile.Camels = []Camel{{ID: "one_camel_1", Island: "one", X: 0, Z: 4}}
	p.profile.HomeIsland.Objects = []BuildObject{{ID: "wall", Kind: "wall", X: 0, Z: 2}}
	if err := w.Act("one", Action{Action: "camel_mount", Target: "one_camel_1"}); err == nil {
		t.Fatal("mounting bypassed a wall")
	}
}

func TestCamelIslandEdgeParkingSurvivesRestart(t *testing.T) {
	w, p := testWorld()
	buyTestIsland(t, w, p)
	mustAct(t, w, Action{Action: "teleport_home"})
	p.x, p.z = 37, 0
	p.profile.Camels = []Camel{{ID: "one_camel_1", Island: "one", X: p.x, Z: p.z}}
	camelCompatibility(&p.profile)
	mustAct(t, w, Action{Action: "camel_mount", Target: "one_camel_1"})
	mustAct(t, w, Action{Action: "camel_dismount"})
	if !reflect.DeepEqual(New(w.Profiles()).Profiles()["one"].Camels, p.profile.Camels) {
		t.Fatal("a camel parked on the playable island edge moved on restart")
	}
}

func TestOfflineMountedVisitorCamelEvictionIsSaved(t *testing.T) {
	w, owner := testWorld()
	buyTestIsland(t, w, owner)
	owner.profile.HomeIsland.Visitors = true
	w.Join("two", "Visitor")
	visitor := w.players["two"]
	visitor.profile.Friends["one"] = true
	visitor.friendsTeleport = true
	visitor.profile.Camels = []Camel{{ID: "two_camel_1", X: visitor.x, Z: visitor.z}}
	camelCompatibility(&visitor.profile)
	if err := w.Act("two", Action{Action: "camel_mount", Target: "two_camel_1"}); err != nil {
		t.Fatal(err)
	}
	if err := w.Act("two", Action{Action: "visit_island", Target: "one"}); err != nil {
		t.Fatal(err)
	}
	w.Leave("two")
	mustAct(t, w, Action{Action: "island_visitors", Enabled: false})
	if visitor.island != "" || !visitor.ridingCamel {
		t.Fatal("closing an island should return an offline rider and camel together")
	}
	saved := w.Profiles()["two"].Camels
	if len(saved) != 1 || saved[0].Island != "" || saved[0].X != visitor.x || saved[0].Z != visitor.z {
		t.Fatal("offline camel teleport was absent from saved profiles")
	}
}
