package game

import (
	"fmt"
	"math"
	"regexp"
	"testing"
	"time"
)

func buyTestIsland(t *testing.T, w *World, p *player) {
	t.Helper()
	p.profile.Coins = 1000
	p.x, p.z = 111, -124
	if err := w.Act(p.profile.ID, Action{Action: "eat_lollipop", Item: "pink"}); err != nil {
		t.Fatal(err)
	}
	if err := w.Act(p.profile.ID, Action{Action: "buy_home"}); err != nil {
		t.Fatal(err)
	}
}

func TestIslandPurchaseSeparateTeleportAndMigration(t *testing.T) {
	w, p := testWorld()
	p.profile.Coins = 200
	if err := w.Act("one", Action{Action: "buy_home"}); err == nil {
		t.Fatal("purchase bypassed lollipop")
	}
	p.x, p.z = 111, -124
	mustAct(t, w, Action{Action: "eat_lollipop", Item: "pink"})
	p.profile.Coins = 199
	if err := w.Act("one", Action{Action: "buy_home"}); err == nil || p.profile.Coins != 199 {
		t.Fatal("insufficient balance changed")
	}
	p.profile.Coins = 200
	mustAct(t, w, Action{Action: "buy_home"})
	if p.profile.Coins != 0 || p.profile.HomeIsland == nil || p.island != "" {
		t.Fatal("purchase should grant island without teleporting")
	}
	mustAct(t, w, Action{Action: "teleport_home"})
	mustAct(t, w, Action{Action: "select", Slot: 5})
	if p.island != "one" || p.profile.Selected != "build_menu" {
		t.Fatal("home teleport / sixth hotbar slot missing")
	}
	setItem(p, "sugar", 3)
	mustAct(t, w, Action{Action: "select", Slot: 5})
	if err := w.Act("one", Action{Action: "throw", Item: "sugar"}); err == nil {
		t.Fatal("home combat allowed")
	}
	mustAct(t, w, Action{Action: "teleport_main"})
	if p.island != "" {
		t.Fatal("return portal failed")
	}
	if err := w.Act("one", Action{Action: "build_home", Item: "tent"}); err == nil {
		t.Fatal("main island construction remains available")
	}
	legacy := Profile{ID: "legacy", Name: "Old", Home: &Home{Kind: "cabin", X: -80, Z: -30, Health: 260}}
	restored := New(map[string]Profile{"legacy": legacy})
	pr := restored.Profiles()["legacy"]
	if pr.Home != nil || pr.HomeIsland == nil || pr.Recovery["wood"] != 18 || pr.Recovery["stone"] != 12 {
		t.Fatal("old shelter investment was not migrated")
	}
	if got := New(restored.Profiles()).Profiles()["legacy"].Recovery["wood"]; got != 18 {
		t.Fatal("migration refunded twice")
	}
}

func TestFriendCodesRequestsPrivacyAndOfflinePersistence(t *testing.T) {
	w, p := testWorld()
	w.Join("two", "Friend")
	other := w.players["two"]
	if !regexp.MustCompile(`^[A-Z]{4}[0-9]{4}$`).MatchString(p.profile.FriendCode) || p.profile.FriendCode == other.profile.FriendCode {
		t.Fatal("bad friend code")
	}
	code, user := p.profile.FriendCode, p.profile.Username
	mustAct(t, w, Action{Action: "customize", CallName: "New Baker"})
	if p.profile.FriendCode != code || p.profile.Username != user {
		t.Fatal("permanent identity changed")
	}
	if got := w.Snapshot("two").Players[0].FriendCode; got != "" {
		t.Fatal("private friend code exposed")
	}
	mustAct(t, w, Action{Action: "friend_public", Enabled: true})
	if got := w.Snapshot("two").Players[0].FriendCode; got != code {
		t.Fatal("public friend code missing")
	}
	w.Leave("two")
	mustAct(t, w, Action{Action: "friend_request", Target: other.profile.FriendCode})
	restored := New(w.Profiles())
	restored.Join("two", "Friend")
	if !restored.players["two"].profile.FriendRequests["one"] {
		t.Fatal("offline friend request not persisted")
	}
	w.Join("two", "Friend")
	if err := w.Act("two", Action{Action: "friend_accept", Target: "one"}); err != nil {
		t.Fatal(err)
	}
	if !p.profile.Friends["two"] || !other.profile.Friends["one"] {
		t.Fatal("friendship not reciprocal")
	}
	if len(w.Snapshot("one").Friends) != 1 {
		t.Fatal("missing friend view")
	}
	copy := w.Profiles()
	delete(copy["one"].Friends, "two")
	if !p.profile.Friends["two"] {
		t.Fatal("profile export aliases friend map")
	}
	mustAct(t, w, Action{Action: "friend_remove", Target: "two"})
	if other.profile.Friends["one"] {
		t.Fatal("removal not reciprocal")
	}
}

func TestIslandVisitorBuilderChestIsolationAndRevocation(t *testing.T) {
	w, p := testWorld()
	buyTestIsland(t, w, p)
	w.Join("two", "Friend")
	friend := w.players["two"]
	p.profile.Friends["two"] = true
	friend.profile.Friends["one"] = true
	friend.friendsTeleport = true
	if err := w.Act("two", Action{Action: "visit_island", Target: "one"}); err == nil {
		t.Fatal("visited closed island")
	}
	mustAct(t, w, Action{Action: "island_visitors", Enabled: true})
	if err := w.Act("two", Action{Action: "visit_island", Target: "one"}); err != nil {
		t.Fatal(err)
	}
	setItem(friend, "wood", 30)
	setItem(friend, "stone", 10)
	if err := w.Act("two", Action{Action: "build", Item: "wall", X: 4, Z: 24}); err == nil {
		t.Fatal("unpermitted visitor built")
	}
	friend.x, friend.z = 4, 20
	if err := w.Act("two", Action{Action: "chest_deposit", Item: "wood", Amount: 6}); err != nil {
		t.Fatal(err)
	}
	if p.profile.HomeIsland.Chest["wood"] != 6 {
		t.Fatal("donation not saved to owner")
	}
	if err := w.Act("two", Action{Action: "chest_withdraw", Item: "wood", Amount: 1}); err == nil {
		t.Fatal("visitor stole materials")
	}
	s := w.Snapshot("two")
	if len(s.Players) != 0 || len(s.Animals) != 0 || len(s.Nodes) != 1 || s.Nodes[0].Kind != "island_portal" || len(s.Island.Chest) != 0 {
		t.Fatal("island leaked main entities or private chest")
	}
	mustAct(t, w, Action{Action: "island_builder", Target: "two", Enabled: true})
	if err := w.Act("two", Action{Action: "chest_withdraw", Item: "wood", Amount: 2}); err != nil {
		t.Fatal(err)
	}
	if err := w.Act("two", Action{Action: "build", Item: "wall", X: 4, Z: 24, Rotation: 45}); err != nil {
		t.Fatal(err)
	}
	if len(p.profile.HomeIsland.Objects) != 2 || p.profile.HomeIsland.Objects[1].Rotation != 45 {
		t.Fatal("friend construction failed")
	}
	copy := w.Snapshot("two")
	copy.Island.Objects[1].X = 999
	copy.Island.Chest["wood"] = 999
	if p.profile.HomeIsland.Objects[1].X == 999 || p.profile.HomeIsland.Chest["wood"] == 999 {
		t.Fatal("snapshot aliases island")
	}
	mustAct(t, w, Action{Action: "island_builder", Target: "two", Enabled: false})
	if err := w.Act("two", Action{Action: "destroy_build", Target: p.profile.HomeIsland.Objects[1].ID}); err == nil {
		t.Fatal("revoked builder destroyed wall")
	}
	w.Leave("two")
	mustAct(t, w, Action{Action: "island_visitors", Enabled: false})
	w.Join("two", "Friend")
	if friend.island != "" {
		t.Fatal("offline visitor bypassed closed island")
	}
}

func TestIslandReturnPlatformForOwnersVisitorsAndExistingSaves(t *testing.T) {
	w, p := testWorld()
	buyTestIsland(t, w, p)
	mustAct(t, w, Action{Action: "teleport_home"})
	if err := w.Act("one", Action{Action: "teleport_cave"}); err == nil {
		t.Fatal("platform can be used remotely")
	}
	for _, item := range []string{"wood", "stone", "stick"} {
		setItem(p, item, 100)
	}
	for _, a := range []Action{
		{Action: "build", Item: "wall", X: 0, Z: 28},
		{Action: "build", Item: "roof", X: 0, Y: 5, Z: 28},
		{Action: "build", Item: "wall", X: 4, Z: 28, Rotation: 45},
	} {
		if err := w.Act("one", a); err == nil {
			t.Fatal("construction blocked the platform landing area")
		}
	}
	mustAct(t, w, Action{Action: "build", Item: "chair", X: 6, Z: 24})
	chair := p.profile.HomeIsland.Objects[len(p.profile.HomeIsland.Objects)-1]
	if err := w.Act("one", Action{Action: "move_build", Target: chair.ID, X: 0, Z: 28}); err == nil {
		t.Fatal("moving furniture blocked the platform")
	}
	p.x, p.z = 0, 28
	mustAct(t, w, Action{Action: "interact", Target: "island_portal"})
	if p.island != "" || p.x != 120 || p.z != -117 || groundY(p) != -10 || w.staticBlocked(p.x, p.z, .42) {
		t.Fatal("platform did not return to an accessible underground cave fork")
	}
	if err := w.Act("one", Action{Action: "teleport_cave"}); err == nil {
		t.Fatal("platform worked on the main island")
	}
	w.Join("visitor", "Visitor")
	visitor := w.players["visitor"]
	w.setIsland(visitor, "one")
	visitor.x, visitor.z = 0, 28
	if err := w.Act("visitor", Action{Action: "teleport_cave"}); err != nil {
		t.Fatalf("visitor cannot leave through owner's platform: %v", err)
	}

	legacy := Profile{ID: "legacy", Name: "Old owner", HomeIsland: &Island{Objects: []BuildObject{{ID: "old_oven", Kind: "oven", X: 0, Z: 28, Rotation: 45, Lit: true}}}}
	restored := New(map[string]Profile{"legacy": legacy})
	restored.Join("legacy", "Old owner")
	owner := restored.players["legacy"]
	restored.setIsland(owner, "legacy")
	snapshot := restored.Snapshot("legacy")
	if len(snapshot.Nodes) != 1 || snapshot.Nodes[0].ID != "island_portal" || snapshot.Nodes[0].Island != "legacy" {
		t.Fatal("existing island did not receive its permanent platform")
	}
	if len(snapshot.Island.Objects) != 1 || snapshot.Island.Objects[0].ID != "old_oven" || overlapsIslandPortal(snapshot.Island.Objects[0]) || snapshot.Island.Objects[0].Rotation != 45 || !snapshot.Island.Objects[0].Lit {
		t.Fatal("existing furnishing was not preserved and moved clear of the platform")
	}
	again := New(restored.Profiles()).Profiles()["legacy"].HomeIsland
	if len(again.Objects) != 1 || again.Objects[0] != snapshot.Island.Objects[0] {
		t.Fatal("platform migration changed an already migrated island")
	}
}

func TestIslandPlatformMigrationAllowsExistingFloorAndDecorOverlap(t *testing.T) {
	// An entirely paved island with border plants is valid construction, but
	// offers no wholly empty footprint for the oven covering its new platform.
	objects := []BuildObject{{ID: "saved_oven", Kind: "oven", X: 0, Z: 28, Lit: true}, {ID: "saved_mixer", Kind: "mixer", X: 0, Z: 24}}
	for x := -30.0; x <= 30; x += 3 {
		for z := -30.0; z <= 30; z += 3 {
			if math.Hypot(x, z)+math.Hypot(4, 4)/2 <= 34 {
				objects = append(objects, BuildObject{ID: fmt.Sprintf("floor_%g_%g", x, z), Kind: "floor", X: x, Z: z})
			}
		}
	}
	for i := 0; i < 120; i++ {
		a := float64(i) * 2 * math.Pi / 120
		objects = append(objects, BuildObject{ID: fmt.Sprintf("plant_%d", i), Kind: "plant", X: 31.5 * math.Cos(a), Z: 31.5 * math.Sin(a)})
	}
	saved := Profile{ID: "paved", Name: "Paved island", HomeIsland: &Island{Objects: objects}}
	w := New(map[string]Profile{"paved": saved})
	h := w.Profiles()["paved"].HomeIsland
	if len(h.Objects) != len(objects) {
		t.Fatal("platform migration discarded a saved furnishing")
	}
	for i, o := range h.Objects {
		if overlapsIslandPortal(o) {
			t.Fatalf("%s still covers the platform after migrating a paved island", o.ID)
		}
		before := objects[i]
		before.X, before.Z = o.X, o.Z
		if o != before {
			t.Fatalf("migration changed saved furnishing data for %s", o.ID)
		}
	}
	oven, mixer := h.Objects[0], h.Objects[1]
	if rectanglesOverlap(oven.X, oven.Z, 2, 1.7, oven.Rotation, mixer.X, mixer.Z, 1.5, 1.2, mixer.Rotation) {
		t.Fatal("fallback migration stacked appliances")
	}
	reloaded := New(w.Profiles()).Profiles()["paved"].HomeIsland
	for i, o := range reloaded.Objects {
		if o != h.Objects[i] {
			t.Fatalf("migration moved %s again after restart", o.ID)
		}
	}
}

func TestConstructionRotationOverlapMoveRefundCollisionAndUse(t *testing.T) {
	w, p := testWorld()
	buyTestIsland(t, w, p)
	mustAct(t, w, Action{Action: "teleport_home"})
	p.x, p.z = 0, 0
	for _, i := range []string{"wood", "stone", "stick", "berry"} {
		setItem(p, i, 100)
	}
	mustAct(t, w, Action{Action: "build", Item: "oven", X: 4, Z: 0, Rotation: 50})
	oven := p.profile.HomeIsland.Objects[1]
	if oven.Rotation != 45 {
		t.Fatal("rotation not in 45-degree increments")
	}
	if err := w.Act("one", Action{Action: "build", Item: "mixer", X: 4, Y: 5, Z: 0}); err == nil {
		t.Fatal("stacked appliances accepted")
	}
	mustAct(t, w, Action{Action: "build", Item: "table", X: 4, Y: 3, Z: 0})
	mustAct(t, w, Action{Action: "build", Item: "plant", X: 4, Y: 3, Z: 0})
	if !w.islandBlocked(p, 4, 0) {
		t.Fatal("walked through oven")
	}
	wood := p.profile.Inventory["wood"]
	stone := p.profile.Inventory["stone"]
	mustAct(t, w, Action{Action: "move_build", Target: oven.ID, X: 6, Z: 0, Rotation: 90})
	if p.profile.Inventory["wood"] != wood {
		t.Fatal("moving charged materials")
	}
	mustAct(t, w, Action{Action: "destroy_build", Target: oven.ID})
	if p.profile.Inventory["wood"] != wood+2 || p.profile.Inventory["stone"] != stone+6 {
		t.Fatal("destroy should refund half")
	}
	mustAct(t, w, Action{Action: "build", Item: "chair", X: 2, Z: 0})
	chair := p.profile.HomeIsland.Objects[len(p.profile.HomeIsland.Objects)-1]
	mustAct(t, w, Action{Action: "use_build", Target: chair.ID})
	if p.posture != "sitting" || p.x != 2 {
		t.Fatal("chair not usable")
	}
	p.x, p.y, p.z = 0, 0, 0
	p.posture = ""
	mustAct(t, w, Action{Action: "build", Item: "door", X: 0, Z: 3})
	door := p.profile.HomeIsland.Objects[len(p.profile.HomeIsland.Objects)-1]
	if !w.islandBlocked(p, 0, 3) {
		t.Fatal("closed door is passable")
	}
	mustAct(t, w, Action{Action: "use_build", Target: door.ID})
	if w.islandBlocked(p, 0, 3) {
		t.Fatal("open door remains blocked")
	}
	p.x, p.z = 0, 3
	if err := w.Act("one", Action{Action: "use_build", Target: door.ID}); err == nil {
		t.Fatal("closing door trapped player inside collision")
	}
	if !p.profile.HomeIsland.Objects[len(p.profile.HomeIsland.Objects)-1].Open {
		t.Fatal("failed close modified door")
	}
}

func TestIslandGardenOvenPondAndPermanentFire(t *testing.T) {
	w, p := testWorld()
	now := time.Unix(1700000000, 0)
	w.now = func() time.Time { return now }
	buyTestIsland(t, w, p)
	mustAct(t, w, Action{Action: "teleport_home"})
	p.x, p.z = 0, 0
	for _, i := range []string{"wood", "stone", "stick", "berry", "nut", "dough"} {
		setItem(p, i, 100)
	}
	mustAct(t, w, Action{Action: "build", Item: "garden_bed", X: 4, Z: 0})
	garden := p.profile.HomeIsland.Objects[1].ID
	mustAct(t, w, Action{Action: "use_build", Target: garden, Item: "berry"})
	if err := w.Act("one", Action{Action: "use_build", Target: garden}); err == nil {
		t.Fatal("garden harvested early")
	}
	now = now.Add(121 * time.Second)
	mustAct(t, w, Action{Action: "use_build", Target: garden})
	if p.profile.HomeIsland.Objects[1].Planted != "" {
		t.Fatal("garden did not harvest")
	}
	mustAct(t, w, Action{Action: "build", Item: "oven", X: -4, Z: 0})
	mustAct(t, w, Action{Action: "craft", Item: "cake"})
	mustAct(t, w, Action{Action: "build", Item: "pond", X: 0, Z: -8})
	mustAct(t, w, Action{Action: "build", Item: "fire", X: 0, Z: 4})
	fire := p.profile.HomeIsland.Objects[len(p.profile.HomeIsland.Objects)-1].ID
	w.time += 1501
	w.Tick(.1)
	if !p.profile.HomeIsland.Objects[len(p.profile.HomeIsland.Objects)-1].Lit {
		t.Fatal("home fire expired")
	}
	mustAct(t, w, Action{Action: "use_build", Target: fire})
	if p.profile.HomeIsland.Objects[len(p.profile.HomeIsland.Objects)-1].Lit {
		t.Fatal("home fire did not extinguish")
	}
	mustAct(t, w, Action{Action: "teleport_main"})
	fish := findAnimal(w, "fish")
	fish.Egg = false
	fish.Health = 10
	fish.Need = "wounded"
	p.x, p.z = fish.X, fish.Z
	mustAct(t, w, Action{Action: "adopt_animal", Target: fish.ID})
	if fish.Health != 0 || p.profile.HomeIsland.Objects[3].Adopted != "fish" {
		t.Fatal("adoption did not transfer wounded fish to pond")
	}
}

func TestCamelPricingGiftsRentalExpiryAndPersistence(t *testing.T) {
	w, p := testWorld()
	now := time.Unix(1700000000, 0)
	w.now = func() time.Time { return now }
	p.profile.Coins = 1000
	if err := w.Act("one", Action{Action: "camel_buy"}); err == nil {
		t.Fatal("remote camel purchase")
	}
	for _, n := range w.nodes {
		if n.ID == "abu_fanous" {
			p.x, p.z = n.X, n.Z
		}
	}
	mustAct(t, w, Action{Action: "camel_rent"})
	if p.profile.Coins != 950 || !p.ridingCamel || p.profile.CamelRentalUntil != now.Unix()+1200 {
		t.Fatal("rental contract incorrect")
	}
	now = now.Add(1201 * time.Second)
	w.Tick(.1)
	if p.ridingCamel {
		t.Fatal("expired rental remains mounted")
	}
	if err := w.Act("one", Action{Action: "camel_gift"}); err == nil {
		t.Fatal("incomplete gift qualified for discount")
	}
	for _, r := range recipes {
		setItem(p, r.ID, 1)
	}
	mustAct(t, w, Action{Action: "camel_gift"})
	for _, r := range recipes {
		if p.profile.Inventory[r.ID] != 0 {
			t.Fatal("gift not charged")
		}
	}
	mustAct(t, w, Action{Action: "camel_buy"})
	if !p.profile.CamelOwned || p.profile.Coins != 775 {
		t.Fatal("discounted camel price incorrect")
	}
	if !New(w.Profiles()).Profiles()["one"].CamelOwned {
		t.Fatal("permanent camel lost on restart")
	}
}

func TestIslandStairsAndFloorsAreWalkable(t *testing.T) {
	w, p := testWorld()
	buyTestIsland(t, w, p)
	mustAct(t, w, Action{Action: "teleport_home"})
	p.x, p.y, p.z = 0, 0, -2.2
	p.profile.HomeIsland.Objects = []BuildObject{{ID: "s", Kind: "stairs"}, {ID: "f", Kind: "floor", X: 0, Y: 3, Z: 4}}
	for i := 0; i < 32; i++ {
		w.Input("one", Input{Z: 1})
		w.Tick(.025)
	}
	if p.y < 2.5 {
		t.Fatalf("stairs did not climb: (%f,%f,%f)", p.x, p.y, p.z)
	}
	if w.islandBlocked(p, 0, 3) {
		t.Fatalf("floor above staircase cannot be walked on at y=%v z=%v", p.y, p.z)
	}
	p.y = 0
	p.z = 0
	if !w.islandBlocked(p, 1, 1.5) {
		t.Fatal("can walk through high side of stairs")
	}
	if !rectanglesOverlap(0, 0, 4, 1, 45, 1, 1, 2, 2, 0) || rectanglesOverlap(0, 0, 4, 1, 45, 20, 20, 2, 2, 0) {
		t.Fatal("rotated footprint collision broken")
	}
	if math.IsNaN(w.islandGround(p)) {
		t.Fatal("invalid ground height")
	}
}

func TestConstructionRefundIsAtomicWhenBagFull(t *testing.T) {
	w, p := testWorld()
	buyTestIsland(t, w, p)
	mustAct(t, w, Action{Action: "teleport_home"})
	p.x, p.z = 0, 0
	p.profile.HomeIsland.Objects = []BuildObject{{ID: "oven", Kind: "oven", X: 4}}
	for i := range p.profile.BagSlots {
		p.profile.BagSlots[i] = Stack{Item: "sugar", Count: inventoryLimit}
	}
	for i := range p.profile.SafeSlots {
		p.profile.SafeSlots[i] = Stack{Item: "sugar", Count: inventoryLimit}
	}
	for i := range p.profile.Hotbar {
		p.profile.Hotbar[i] = Stack{Item: "sugar", Count: inventoryLimit}
	}
	syncInventory(&p.profile)
	before := p.profile.Inventory["sugar"]
	if err := w.Act("one", Action{Action: "destroy_build", Target: "oven"}); err == nil {
		t.Fatal("refund overflow silently discarded")
	}
	if len(p.profile.HomeIsland.Objects) != 1 || p.profile.Inventory["wood"] != 0 || p.profile.Inventory["stone"] != 0 || p.profile.Inventory["sugar"] != before {
		t.Fatal("failed destruction modified inventory or object")
	}
}

func TestMainCampfireExpiresAfterTwentyFiveMinutes(t *testing.T) {
	w, p := testWorld()
	p.x, p.z = -35, -35
	for _, item := range []string{"wood", "stick", "stone"} {
		setItem(p, item, 10)
	}
	mustAct(t, w, Action{Action: "craft", Item: "fire"})
	fire := w.nodes[len(w.nodes)-1]
	if fire.ExpiresAt != 1500 {
		t.Fatalf("main campfire expiration=%v", fire.ExpiresAt)
	}
	w.time = 1499
	w.Tick(.1)
	found := false
	for _, n := range w.nodes {
		if n.ID == fire.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("campfire expired before 25 minutes")
	}
	w.time = 1500
	w.Tick(.1)
	for _, n := range w.nodes {
		if n.ID == fire.ID {
			t.Fatal("main campfire burned longer than 25 minutes")
		}
	}
}

func TestTreeRegrowthDoesNotTrapPlayer(t *testing.T) {
	w, p := testWorld()
	var tree *Node
	for _, n := range w.nodes {
		if n.Kind == "wood" {
			tree = n
			break
		}
	}
	if tree == nil {
		t.Fatal("no tree")
	}
	tree.Available = false
	tree.readyAt = 0
	tree.ChopRemaining = 0
	p.x, p.z = tree.X, tree.Z
	w.Tick(.1)
	if tree.Available {
		t.Fatal("tree regrew around player")
	}
	p.x += 5
	w.time = tree.readyAt
	w.Tick(.1)
	if !tree.Available || tree.ChopRemaining != tree.ChopTotal {
		t.Fatal("clear tree failed to regrow")
	}
}
