package game

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func townWorld() (*World, *player) {
	w, p := testWorld()
	p.x, p.z = 168, 155
	p.profile.Coins = 100
	return w, p
}
func townNode(t *testing.T, w *World, id string) *Node {
	t.Helper()
	for _, n := range w.nodes {
		if n.ID == id {
			return n
		}
	}
	t.Fatalf("missing town node %s", id)
	return nil
}
func fillTownBag(p *player) {
	for i := range p.profile.BagSlots {
		p.profile.BagSlots[i] = Stack{"wood", inventoryLimit}
	}
	syncInventory(&p.profile)
}

func TestTownBuildingsAndPickableGardenHaveClearRoutes(t *testing.T) {
	w, p := townWorld()
	boards, flowers := 0, 0
	for _, n := range w.nodes {
		if n.Kind == "leaderboard_board" {
			boards++
			if !w.moveMainland(p, n.X, n.Z+.8, 0) {
				t.Fatalf("board %s has no standing room", n.ID)
			}
		}
		if _, ok := townFlowerFor(n.Kind); ok {
			flowers++
			if n.Amount != 1 || n.respawn != 45 || w.staticBlocked(n.X, n.Z, .4) {
				t.Fatalf("flower inaccessible or invalid: %+v", n)
			}
		}
	}
	if boards != 5 || flowers != 9 {
		t.Fatalf("got %d boards and %d flowers", boards, flowers)
	}
	for _, route := range [][3]float64{{130, 163, 146.8}, {168, 161, 154}, {198, 161, 154}} {
		p.x, p.z = route[0], route[1]
		for p.z > route[2] {
			next := p.z - .2
			if !w.moveMainland(p, p.x, next, 0) {
				t.Fatalf("shop route blocked at %v,%v", p.x, next)
			}
			p.z = next
		}
	}
	for _, prop := range w.layout.Props {
		if strings.HasPrefix(prop.ID, "town_") {
			continue
		}
		if prop.X > 119 && prop.X < 208 && prop.Z > 135 && prop.Z < 177 {
			t.Fatalf("decor obstructs town block: %+v", prop)
		}
	}
}

func TestTownFlowerGatherSaleRegrowthAndBouquetGift(t *testing.T) {
	w, p := townWorld()
	n := townNode(t, w, "town_flower_0_0")
	p.x, p.z = n.X, n.Z
	mustAct(t, w, Action{Action: "interact", Target: n.ID})
	if n.Available || p.profile.Inventory["flower_daisy"] != 1 {
		t.Fatal("visible flower did not become one inventory stem")
	}
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "interact", Target: n.ID}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("picked flower could be collected twice")
	}
	advance(w, 45.1)
	if !n.Available {
		t.Fatal("flower never regrew")
	}
	p.x, p.z = 168, 155
	mustAct(t, w, Action{Action: "sell_flower", Target: "flower_shop", Item: "flower_daisy"})
	if p.profile.Coins != 103 || p.profile.CoinsEarned != 3 || p.profile.Inventory["flower_daisy"] != 0 {
		t.Fatal("single stem price or earnings wrong")
	}
	mustAct(t, w, Action{Action: "buy_bouquet", Target: "flower_shop", Item: "flower_bouquet", Amount: 2})
	if p.profile.Coins != 63 || p.profile.Inventory["flower_bouquet"] != 2 || p.profile.CoinsEarned != 3 {
		t.Fatal("bouquet purchase wrong")
	}
	w.Join("friend", "Mia")
	friend := w.players["friend"]
	friend.x, friend.z = p.x+3.9, p.z
	mustAct(t, w, Action{Action: "give_bouquet", Target: "friend"})
	if p.profile.Inventory["flower_bouquet"] != 1 || friend.profile.Inventory["flower_bouquet"] != 1 || p.profile.Coins != 63 || friend.profile.Coins != 0 {
		t.Fatal("gift did not transfer exactly one bouquet")
	}
	if w.events[len(w.events)-1].Kind != "gift" {
		t.Fatal("recipient received no gift event")
	}
}

func TestTownPurchasesAndSalesRejectInvalidRequestsAtomically(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		setup func(*player, *Action)
	}{
		{"far", func(p *player, a *Action) { p.x = 160 }},
		{"cave", func(p *player, a *Action) { p.inCave = true }},
		{"island", func(p *player, a *Action) { p.island = "one" }},
		{"above", func(p *player, a *Action) { p.jumpOffset = 3 }},
		{"wrong counter", func(p *player, a *Action) { a.Target = "pet_shop" }},
		{"no target", func(p *player, a *Action) { a.Target = "" }},
		{"bad item", func(p *player, a *Action) { a.Item = "cake" }},
		{"bad count", func(p *player, a *Action) { a.Amount = -1 }},
		{"too many", func(p *player, a *Action) { a.Amount = 21 }},
		{"poor", func(p *player, a *Action) { p.profile.Coins = 19 }},
		{"full bag", func(p *player, a *Action) { fillTownBag(p) }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			w, p := townWorld()
			a := Action{Action: "buy_bouquet", Target: "flower_shop", Item: "flower_bouquet", Amount: 1}
			scenario.setup(p, &a)
			before := cloneProfile(p.profile)
			if err := w.Act("one", a); err == nil || !reflect.DeepEqual(before, p.profile) {
				t.Fatal("invalid bouquet purchase succeeded or mutated profile")
			}
		})
	}
	w, p := townWorld()
	setItem(p, "flower_tulip", 2)
	p.profile.Coins = coinLimit - 4
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "sell_flower", Target: "flower_shop", Item: "flower_tulip"}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("overflowing wallet lost a flower")
	}
	p.profile.Coins = 0
	for _, item := range []string{"flower_bouquet", "wood", "cake", "flower_lavender"} {
		before := cloneProfile(p.profile)
		if err := w.Act("one", Action{Action: "sell_flower", Target: "flower_shop", Item: item}); err == nil || !reflect.DeepEqual(before, p.profile) {
			t.Fatalf("invalid flower sale %s altered profile", item)
		}
	}
}

func TestTownGiftChecksBothInventoriesAndPhysicalLayer(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		setup func(*World, *player, *player, *Action)
	}{
		{"self", func(w *World, p, q *player, a *Action) { a.Target = p.profile.ID }},
		{"unknown", func(w *World, p, q *player, a *Action) { a.Target = "nobody" }},
		{"disconnected", func(w *World, p, q *player, a *Action) { q.disconnected = true }},
		{"giver disconnected", func(w *World, p, q *player, a *Action) { p.disconnected = true }},
		{"far", func(w *World, p, q *player, a *Action) { q.x = p.x + 4.01 }},
		{"cave", func(w *World, p, q *player, a *Action) { q.inCave = true }},
		{"island", func(w *World, p, q *player, a *Action) { q.island = "friend" }},
		{"height", func(w *World, p, q *player, a *Action) { q.jumpOffset = 2.01 }},
		{"full recipient", func(w *World, p, q *player, a *Action) { fillTownBag(q) }},
		{"no bouquet", func(w *World, p, q *player, a *Action) { take(p, "flower_bouquet", 1) }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			w, p := townWorld()
			setItem(p, "flower_bouquet", 1)
			w.Join("friend", "Mia")
			q := w.players["friend"]
			q.x, q.z = p.x, p.z
			a := Action{Action: "give_bouquet", Target: "friend"}
			scenario.setup(w, p, q, &a)
			beforeP, beforeQ := cloneProfile(p.profile), cloneProfile(q.profile)
			if err := w.Act("one", a); err == nil || !reflect.DeepEqual(beforeP, cloneProfile(p.profile)) || !reflect.DeepEqual(beforeQ, cloneProfile(q.profile)) {
				t.Fatal("invalid gift changed a player inventory")
			}
		})
	}
	// Same-island gifts work, and a full bag with an existing stack has capacity.
	w, p := townWorld()
	w.Join("friend", "Mia")
	q := w.players["friend"]
	p.island, q.island = "one", "one"
	p.x, p.z, q.x, q.z = 0, 0, 0, 0
	setItem(p, "flower_bouquet", 1)
	fillTownBag(q)
	q.profile.BagSlots[0] = Stack{"flower_bouquet", inventoryLimit - 1}
	syncInventory(&q.profile)
	mustAct(t, w, Action{Action: "give_bouquet", Target: "friend"})
	if q.profile.Inventory["flower_bouquet"] != inventoryLimit || p.profile.Inventory["flower_bouquet"] != 0 {
		t.Fatal("valid same-island stacked gift failed")
	}
}

func TestTownStatisticsTrackConnectedTimeAndOnlyActualEarnings(t *testing.T) {
	w, p := townWorld()
	w.Tick(.4)
	if math.Abs(p.profile.PlaySeconds-.4) > 1e-8 {
		t.Fatal("connected time missing")
	}
	w.SetConnected("one", false)
	w.Tick(1)
	if math.Abs(p.profile.PlaySeconds-.4) > 1e-8 {
		t.Fatal("disconnect grace counted as playtime")
	}
	w.Leave("one")
	w.Tick(1)
	w.Join("one", "Baker")
	w.Tick(.25)
	p = w.players["one"]
	if math.Abs(p.profile.PlaySeconds-.65) > 1e-8 {
		t.Fatal("offline time counted or reconnect lost time")
	}
	p.profile.Coins = coinLimit - 2
	addCoins(p, 5)
	addCoins(p, -9)
	addCoins(p, 99)
	if p.profile.Coins != coinLimit || p.profile.CoinsEarned != 2 {
		t.Fatal("earnings counted money not credited")
	}
	p.profile.CoinsEarned = coinLimit - 1
	p.profile.Coins = 0
	addCoins(p, 5)
	if p.profile.CoinsEarned != coinLimit {
		t.Fatal("lifetime statistic overflowed")
	}
	casino, c := clubWorld()
	c.profile.CoinsEarned = 8
	casino.clubRandom = func() (int, error) { return 6, nil }
	mustAct(t, casino, Action{Action: "club_gamble", Target: "club_casino", Amount: 10})
	if c.profile.Coins != 1090 || c.profile.CoinsEarned != 8 {
		t.Fatal("gambling payout entered earnings board")
	}
	old := New(map[string]Profile{"old": {Name: "Old player", Coins: 5000}}).Profiles()["old"]
	if old.CoinsEarned != 0 || old.PlaySeconds != 0 || old.CamelCoins != 0 {
		t.Fatal("old balances became fabricated lifetime statistics")
	}
}

func TestTownBakeryCustomerSalesCountAsEarned(t *testing.T) {
	w, p := businessWorld(t)
	nodePosition(w, p, "bakery_plot_0_display")
	mustAct(t, w, Action{Action: "bakery_stock", Item: "sugar", Amount: 5})
	c := firstWaitingCustomer(t, w, p)
	mustAct(t, w, Action{Action: "bakery_serve", Target: c.ID})
	if p.profile.CoinsEarned != 10 {
		t.Fatalf("real bakery sale omitted from earnings: %d", p.profile.CoinsEarned)
	}
}

func TestTownBoardsIncludeSavedOfflineLivePlayersAndStableTies(t *testing.T) {
	saved := map[string]Profile{"a": {Name: "Ada", CamelCoins: 10, CoinsEarned: 20, PlaySeconds: 30, Kills: 4}, "b": {Name: "Bea", CamelCoins: 10, CoinsEarned: 40, PlaySeconds: 60, Kills: 8}, "c": {Name: "Cee", CamelCoins: 1}}
	for i := 0; i < 12; i++ {
		id := string(rune('d' + i))
		saved[id] = Profile{Name: "Saved " + id, CamelCoins: 5}
	}
	w := New(saved)
	w.Join("b", "Bea")
	w.players["b"].profile.CamelCoins = 11
	w.Leave("b")
	w.Join("c", "Cee")
	w.players["c"].profile.CoinsEarned = 50
	snapshot := w.Snapshot("c")
	boards := snapshot.Town.Leaderboards
	if len(boards) != 5 || len(boards[0].Entries) != 10 || boards[0].Total != 15 || boards[0].Entries[0].ID != "b" || boards[2].Entries[0].ID != "c" {
		t.Fatalf("saved/offline/live ranks wrong: %+v", boards)
	}
	if boards[0].Self == nil || boards[0].Self.ID != "c" || boards[0].Self.Rank != 15 {
		t.Fatal("own rank outside top10 missing")
	}
	if boards[0].Entries[2].Rank != 3 || boards[0].Entries[3].Rank != 3 || boards[0].Entries[2].ID != "d" {
		t.Fatal("ties lack stable shared ranks")
	}
	if !reflect.DeepEqual(boards, w.SnapshotUpdate("c").Town.Leaderboards) {
		t.Fatal("unchanged leaderboard order is nondeterministic")
	}
	snapshot.Town.Leaderboards[0].Entries[0].Name = "Mutated"
	snapshot.Town.FlowerCatalog[0].SellPrice = 900
	snapshot.Town.Betty.Lines[0].Reply = "Changed"
	next := w.Snapshot("c")
	if next.Town.Leaderboards[0].Entries[0].Name == "Mutated" || next.Town.FlowerCatalog[0].SellPrice != 3 || next.Town.Betty.Lines[0].Reply == "Changed" {
		t.Fatal("snapshot aliases world data")
	}
}

func TestTownStatsNormalizeCorruptValues(t *testing.T) {
	for _, bad := range []float64{-1, math.NaN(), math.Inf(1)} {
		p := sanitize(Profile{CamelCoins: -1, CoinsEarned: -1, PlaySeconds: bad})
		if p.CamelCoins != 0 || p.CoinsEarned != 0 || p.PlaySeconds != 0 {
			t.Fatal("invalid lifetime stats retained")
		}
	}
}

func TestCookieRingBoardRanksSavedRingKillsSeparatelyFromAllKills(t *testing.T) {
	w := New(map[string]Profile{
		"overall": {Name: "Overall", Kills: 80, CookieRingKills: 2},
		"ring":    {Name: "Ring", Kills: 30, CookieRingKills: 25},
		"old":     {Name: "Old save", Kills: 100},
	})
	boards := w.townSnapshot("ring").Leaderboards
	if len(boards) != 5 || boards[3].Entries[0].ID != "old" || boards[4].ID != "cookie_ring_kills" || boards[4].Entries[0].ID != "ring" || boards[4].Self.Value != 25 {
		t.Fatalf("overall/ring rankings were conflated: %+v", boards)
	}
	p := Profile{CookieRingKills: -5}
	normalizeTownStats(&p)
	if p.CookieRingKills != 0 {
		t.Fatal("negative saved ring kills were not normalized")
	}
}
