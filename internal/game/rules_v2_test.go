package game

import (
	"math"
	"sort"
	"strings"
	"testing"
	"time"
)

func nodePosition(w *World, p *player, id string) {
	for _, n := range w.nodes {
		if n.ID == id {
			if n.PlotID != "" {
				if owner := w.players[w.plotOwner(n.PlotID)]; owner != nil {
					if point, ok := w.bakeryStationServicePoint(owner, strings.TrimPrefix(n.Kind, "bakery_"), false); ok {
						p.x, p.z = point.X, point.Z
						return
					}
				}
			}
			p.x, p.z = n.X, n.Z
			return
		}
	}
	panic("missing node " + id)
}

func TestSlotMigrationRetainsEveryItemAndOnlyMigratesOnce(t *testing.T) {
	old := map[string]int{}
	for item := range itemSet {
		old[item] = 7
	}
	w := New(map[string]Profile{"one": {Name: "Old Baker", Inventory: old, Protected: []string{"sugar", "berry_cookie", "cake", "nut_cookie", "sun_cookie"}, Selected: "protein_cookie"}})
	w.Join("one", "Login Name")
	p := w.players["one"]
	if len(p.profile.SafeSlots) != 0 || p.profile.InventoryVersion != 3 || p.profile.Hotbar[0].Item != "protein_cookie" {
		t.Fatal("migration lost requested priority slots")
	}
	for item, count := range old {
		if p.profile.Inventory[item]+p.profile.Recovery[item] != count {
			t.Fatalf("migration lost %s", item)
		}
	}
	if len(p.profile.Recovery) == 0 {
		t.Fatal("full legacy inventory must retain overflow")
	}
	restored := New(w.Profiles())
	restored.Join("one", "Login Name")
	r := restored.players["one"]
	for item, count := range old {
		if r.profile.Inventory[item]+r.profile.Recovery[item] != count {
			t.Fatalf("repeat migration duplicated or lost %s", item)
		}
	}
	before := w.Snapshot("one")
	before.Me.Hotbar[0].Count = 999
	before.Me.Recovery["wood"] = 999
	if p.profile.Hotbar[0].Count == 999 || p.profile.Recovery["wood"] == 999 {
		t.Fatal("snapshot aliases canonical storage")
	}
}

func TestInventoryMovesSwapsAndHotbarRestriction(t *testing.T) {
	w, p := testWorld()
	gain(p, "wood", 12)
	gain(p, "berry_cookie", 4)
	mustAct(t, w, Action{Action: "move_item", From: "bag", FromSlot: 0, To: "bag", ToSlot: 19})
	if p.profile.BagSlots[19] != (Stack{"wood", 12}) {
		t.Fatal("expanded inventory slots must accept resources")
	}
	if err := w.Act("one", Action{Action: "throw", Item: "berry_cookie"}); err == nil {
		t.Fatal("bag cookie bypassed hotbar")
	}
	mustAct(t, w, Action{Action: "move_item", From: "bag", FromSlot: 1, To: "hotbar", ToSlot: 1, Amount: 2})
	if p.profile.Hotbar[1].Count != 2 || p.profile.BagSlots[1].Count != 2 {
		t.Fatal("split stack count wrong")
	}
	mustAct(t, w, Action{Action: "select", Slot: 1})
	mustAct(t, w, Action{Action: "throw"})
	if p.profile.Hotbar[1].Count != 1 || p.profile.Inventory["berry_cookie"] != 3 {
		t.Fatal("throw did not consume hotbar only")
	}
	mustAct(t, w, Action{Action: "move_item", From: "bag", FromSlot: 19, To: "hotbar", ToSlot: 1})
	if p.profile.BagSlots[19].Item != "berry_cookie" || p.profile.Hotbar[1].Item != "wood" {
		t.Fatal("different items should swap")
	}
	if err := w.Act("one", Action{Action: "move_item", From: "bag", FromSlot: 15, To: "safe", ToSlot: 0}); err == nil {
		t.Fatal("slot bounds bypassed")
	}
}

func TestDeathTransfersOnlyCoinsAndKeepsEveryInventorySlot(t *testing.T) {
	for _, tc := range []struct {
		name                string
		killer, coins, want int
	}{{"poorer", 0, 119, 12}, {"richer", 200, 119, 5}, {"equal", 119, 119, 5}, {"one coin", 0, 1, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			w, killer := testWorld()
			w.Join("two", "Victim")
			v := w.players["two"]
			v.buffs = map[string]float64{}
			v.profile.Coins, killer.profile.Coins = tc.coins, tc.killer
			for i := range v.profile.BagSlots {
				v.profile.BagSlots[i] = Stack{"wood", i + 2}
			}
			for i := range v.profile.Hotbar {
				v.profile.Hotbar[i] = Stack{"cake", i + 3}
			}
			syncInventory(&v.profile)
			bag, hotbar, killerBag, killerHotbar := v.profile.BagSlots, v.profile.Hotbar, killer.profile.BagSlots, killer.profile.Hotbar
			w.damagePlayer(killer, v, 1000, "test")
			if killer.profile.Coins != tc.killer+tc.want || v.profile.Coins != tc.coins-tc.want {
				t.Fatal("wrong wealth-based share")
			}
			if v.profile.BagSlots != bag || v.profile.Hotbar != hotbar || killer.profile.BagSlots != killerBag || killer.profile.Hotbar != killerHotbar {
				t.Fatal("death deleted or transferred inventory items")
			}
		})
	}
}

func TestDeathLootDoesNotDeleteStacksWhenKillerBagIsFull(t *testing.T) {
	w, killer := testWorld()
	keys := []string{}
	for item := range itemSet {
		if item != "wood" && item != "sugar" {
			keys = append(keys, item)
		}
	}
	sort.Strings(keys)
	for i := range killer.profile.BagSlots {
		killer.profile.BagSlots[i] = Stack{keys[i], 1}
	}
	syncInventory(&killer.profile)
	w.Join("two", "Victim")
	v := w.players["two"]
	v.buffs = map[string]float64{}
	v.profile.BagSlots[0] = Stack{"wood", 25}
	syncInventory(&v.profile)
	w.damagePlayer(killer, v, 1000, "test")
	if v.profile.Inventory["wood"] != 25 || killer.profile.Inventory["wood"] != 0 || len(killer.profile.Recovery) != 0 {
		t.Fatal("full killer inventory destroyed loot or bypassed bag size")
	}
}

func TestFullBagPurchaseCraftAndGatherAreAtomic(t *testing.T) {
	w, p := testWorld()
	keys := []string{}
	for item := range itemSet {
		if item != "dough" && item != "sugar" && item != "berry_cookie" && item != "berry" && item != "baking_sugar" {
			keys = append(keys, item)
		}
	}
	sort.Strings(keys)
	for i := range p.profile.BagSlots {
		p.profile.BagSlots[i] = Stack{keys[i], 3}
	}
	syncInventory(&p.profile)
	p.profile.Coins = 100
	nodePosition(w, p, "ingredient_shop")
	if err := w.Act("one", Action{Action: "buy_ingredient", Target: "ingredient_shop", Item: "baking_sugar"}); err == nil {
		t.Fatal("full bag purchase accepted")
	}
	if p.profile.Coins != 100 || len(p.profile.Recovery) > 0 {
		t.Fatal("failed purchase charged coins or created extra storage")
	}
	var node *Node
	for _, n := range w.nodes {
		if n.Kind == "berry" && n.respawn > 0 {
			node = n
			break
		}
	}
	if node == nil {
		t.Fatal("missing renewable berry fixture")
	}
	p.x, p.z = node.X, node.Z
	if err := w.Act("one", Action{Action: "interact", Target: node.ID}); err == nil || !node.Available {
		t.Fatal("full bag gather consumed resource")
	}
	p.profile.BagSlots[0] = Stack{"dough", 2}
	p.profile.BagSlots[1] = Stack{"berry", 4}
	syncInventory(&p.profile)
	nodePosition(w, p, "desert_oven")
	if err := w.Act("one", Action{Action: "craft", Item: "berry_cookie"}); err == nil {
		t.Fatal("craft accepted output without free space")
	}
	if p.profile.Inventory["dough"] != 2 || p.profile.Inventory["berry"] != 4 || p.profile.Discovered["berry_cookie"] {
		t.Fatal("failed craft spent ingredients or discovered recipe")
	}
}

func TestCookbookDiscoveryIsPrivateAndPersistent(t *testing.T) {
	w, p := testWorld()
	w.Join("two", "Other")
	for _, r := range w.Snapshot("one").Recipes {
		if r.Known || r.Damage != 0 || r.Heal != 0 || r.Description != "" || r.Name != "Undiscovered recipe" || len(r.Cost) == 0 {
			t.Fatal("unknown recipe leaked identity or lost formula")
		}
	}
	gain(p, "dough", 1)
	nodePosition(w, p, "desert_oven")
	mustAct(t, w, Action{Action: "craft", Item: "sugar"})
	if !p.profile.Discovered["sugar"] || !p.profile.Hats["chef"] {
		t.Fatal("craft did not save discovery/achievement")
	}
	for _, e := range w.Snapshot("two").Events {
		if e.Kind == "craft" && e.Actor == "one" {
			t.Fatal("other player's discovery revealed recipe name")
		}
	}
	restored := New(w.Profiles())
	restored.Join("one", "Other login name")
	if !restored.Snapshot("one").Recipes[0].Known {
		t.Fatal("cookbook forgotten on restart")
	}
}

func TestBakeryPlotsPricesProgressionAndPaint(t *testing.T) {
	w, p := testWorld()
	p.profile.Coins = 1000
	nodePosition(w, p, "bakery_plot_0")
	mustAct(t, w, Action{Action: "buy_land", Target: "bakery_plot_0"})
	if p.profile.Coins != 750 {
		t.Fatal("bakery should cost250")
	}
	w.Join("two", "Other")
	other := w.players["two"]
	other.profile.Coins = 1000
	nodePosition(w, other, "bakery_plot_0")
	if err := w.Act("two", Action{Action: "buy_land", Target: "bakery_plot_0"}); err == nil {
		t.Fatal("two owners claimed same plot")
	}
	nodePosition(w, other, "bakery_plot_1")
	if err := w.Act("two", Action{Action: "buy_land", Target: "bakery_plot_1"}); err != nil {
		t.Fatal(err)
	}
	for _, cost := range []int{50, 75, 100, 125} {
		before := p.profile.Coins
		mustAct(t, w, Action{Action: "upgrade"})
		if p.profile.Coins != before-cost {
			t.Fatalf("upgrade expected%d", cost)
		}
	}
	nodePosition(w, p, "bakery_plot_0_mixer")
	baseBakingIngredients(p, 1)
	mustAct(t, w, Action{Action: "make_dough"})
	if job := p.profile.Production["bakery_mixer"].Job; job == nil || job.Amount != 40 || job.Remaining != 40 || p.profile.Inventory["dough"] != 0 {
		t.Fatal("upgrades changed the fixed mixing recipe or timer")
	}
	p.profile.BakeryLevel = 10000
	if bakeryBoost(p) >= 1 {
		t.Fatal("upgrade effects should be bounded")
	}
	p.profile.BakeryLevel = 4
	nodePosition(w, p, "kitchen_shop")
	for _, item := range []string{"mixer", "oven", "display", "paint_teal"} {
		mustAct(t, w, Action{Action: "trade", Target: "kitchen_shop", Item: item})
	}
	nodePosition(w, p, "bakery_plot_0")
	mustAct(t, w, Action{Action: "paint", Target: "walls", Item: "teal"})
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	if restored.players["one"].profile.Paint["walls"] != "teal" || restored.plotOwner("bakery_plot_1") != "" || !restored.profiles["two"].Bakery {
		t.Fatal("restart should preserve purchases/paint while releasing city plots")
	}
}

func TestAvatarCallNameCooldownAndEarnedHats(t *testing.T) {
	w, p := testWorld()
	now := time.Unix(2000000000, 0)
	w.now = func() time.Time { return now }
	username := p.profile.Username
	a := Avatar{Skin: 4, Shirt: "hoodie", Pants: "shorts", ShirtColor: "purple", PantsColor: "black"}
	mustAct(t, w, Action{Action: "customize", CallName: "New Call Name", Avatar: &a})
	if p.profile.Username != username || p.profile.Name != "New Call Name" || p.profile.Avatar != a {
		t.Fatal("customization changed permanent name or lost appearance")
	}
	if err := w.Act("one", Action{Action: "customize", CallName: "Too Soon"}); err == nil {
		t.Fatal("call-name cooldown bypassed")
	}
	w.Leave("one")
	w.Join("one", "Login bypass")
	p = w.players["one"]
	if p.profile.Name != "New Call Name" {
		t.Fatal("login bypassed call-name cooldown")
	}
	if err := w.Act("one", Action{Action: "customize", CallName: "Too Soon"}); err == nil {
		t.Fatal("reconnect bypassed saved call-name cooldown")
	}
	now = now.Add(time.Hour)
	mustAct(t, w, Action{Action: "customize", CallName: "Next Name"})
	a.Hat = "champion"
	if err := w.Act("one", Action{Action: "customize", Avatar: &a}); err == nil {
		t.Fatal("unearned hat allowed")
	}
	p.profile.Kills = 5
	awardHats(&p.profile)
	mustAct(t, w, Action{Action: "customize", Avatar: &a})
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	q := restored.players["one"]
	if q.profile.Username != username || q.profile.CallName != "Next Name" || q.profile.CallNameChangedAt != now.Unix() || q.profile.Avatar.Hat != "champion" {
		t.Fatal("identity/cooldown/hat did not persist")
	}
}

func TestMainIslandResidentialConstructionRemoved(t *testing.T) {
	w, p := testWorld()
	gain(p, "wood", 30)
	gain(p, "stick", 20)
	gain(p, "stone", 20)
	p.x, p.z = -85, -93.5
	if err := w.Act("one", Action{Action: "build_home", Item: "tent"}); err == nil {
		t.Fatal("old main-island home construction still available")
	}
	p.x, p.z = -35, -35
	if err := w.Act("one", Action{Action: "build_home", Item: "cabin"}); err == nil {
		t.Fatal("main-island cabin construction still available")
	}
	if p.profile.Home != nil || p.profile.Inventory["wood"] != 30 || len(w.StaticLayout().Zones) != 0 {
		t.Fatal("rejected build spent materials or left residential plots on the map")
	}
}

func TestHitVolumesRewardsAndPreKillWealth(t *testing.T) {
	w, p := testWorld()
	w.Join("two", "Target")
	v := w.players["two"]
	p.x, p.z = -35, -30
	v.x, v.z = -35, -35
	v.buffs = map[string]float64{}
	for _, tc := range []struct {
		x, y  float64
		coins int
	}{{0, 1.68, 5}, {.49, 1.05, 3}, {.19, .15, 3}, {0, 1.04, 1}} {
		shot := &Projectile{Owner: "one", Item: "sugar", X: v.x + tc.x, Y: tc.y, Z: v.z + 1, VZ: -10, damage: 1, born: w.time}
		p.profile.Coins = 0
		w.projectiles = []*Projectile{shot}
		w.moveProjectiles(.2)
		if p.profile.Coins != tc.coins {
			t.Fatalf("hit(%v,%v) reward=%d want=%d", tc.x, tc.y, p.profile.Coins, tc.coins)
		}
	}
	p.profile.Coins = 97
	v.profile.Coins = 100
	v.health = 1
	w.projectiles = []*Projectile{{Owner: "one", Item: "sugar", X: v.x, Y: 1.68, Z: v.z + 1, VZ: -10, damage: 100, born: w.time}}
	w.moveProjectiles(.2)
	if p.profile.Coins != 112 || v.profile.Coins != 90 {
		t.Fatal("headshot reward altered poorer-killer branch before loot")
	}
	w.projectiles = []*Projectile{{Owner: "one", X: -35, Y: 1, Z: -46, VZ: -10, damage: 10, born: w.time}}
	before := p.profile.Coins
	w.moveProjectiles(.2)
	if p.profile.Coins != before {
		t.Fatal("dummy produced PvP hit coins")
	}
	// A trained swimmer's base speed exceeds the shark's six metres per second.
	if 4*(1+10*.06) <= 6 || math.IsNaN(v.health) {
		t.Fatal("swim progression cannot overtake sharks")
	}
}

func TestDesertBarterNeverAcceptsCoins(t *testing.T) {
	w, p := testWorld()
	p.profile.Coins = 100
	nodePosition(w, p, "desert_trader")
	if err := w.Act("one", Action{Action: "trade", Target: "desert_trader", Item: "coins"}); err == nil || p.profile.Coins != 100 {
		t.Fatal("desert allowed coin trading")
	}
	gain(p, "nut", 3)
	mustAct(t, w, Action{Action: "trade", Target: "desert_trader", Item: "nut"})
	if p.profile.Inventory["nut"] != 0 || p.profile.Inventory["cactus_cookie"] != 2 {
		t.Fatal("nut barter failed")
	}
}

func TestSharksOutswimNewPlayersUntilTheyTrain(t *testing.T) {
	w, p := testWorld()
	p.x, p.z = -180, 150
	w.Input("one", Input{X: 1, Sprint: true})
	w.Tick(.2)
	if d := p.x + 180; math.Abs(d-.8) > .00001 {
		t.Fatalf("untrained swimming unexpectedly accelerated: %v", d)
	}
	p.profile.Levels["swim"] = 10
	p.x = -180
	w.Input("one", Input{X: 1, Sprint: true})
	w.Tick(.2)
	if d := p.x + 180; d <= 6*.2 {
		t.Fatal("trained swimmer cannot outrun a shark")
	}
}

func TestSelectedDuplicateHotbarStackIsConsumed(t *testing.T) {
	w, p := testWorld()
	p.profile.Hotbar[1] = Stack{"sugar", 4}
	syncInventory(&p.profile)
	mustAct(t, w, Action{Action: "select", Slot: 1})
	mustAct(t, w, Action{Action: "eat"})
	if p.profile.Hotbar[0].Count != 10 || p.profile.Hotbar[1].Count != 3 {
		t.Fatal("using a selected duplicate stack consumed another hotbar slot")
	}
}

func TestProjectileMuzzleCannotStartBeyondWall(t *testing.T) {
	w, p := testWorld()
	w.colliders = []Collider{{X: 0, Z: 0, Width: 10, Depth: .2, Height: 3}}
	p.x, p.z, p.yaw = 0, .52, 0
	p.buffs["spawn_shield"] = w.time + 3
	if err := w.Act("one", Action{Action: "throw"}); err == nil {
		t.Fatal("a throw started on the far side of the wall")
	}
	if len(w.projectiles) != 0 || p.profile.Inventory["sugar"] != 10 || !w.active(p, "spawn_shield") {
		t.Fatal("blocked throw consumed a cookie or spawn protection")
	}
}

func TestBoostedMovementCannotSkipThinCabinWall(t *testing.T) {
	w, p := testWorld()
	p.profile.Home = &Home{Owner: "one", Kind: "cabin", X: -85, Z: -90, Health: 260, MaxHealth: 260}
	p.x, p.z = -85, -88.5
	p.profile.Levels["legs"] = 10
	p.buffs["speed"] = 10
	w.Input("one", Input{Z: 1, Sprint: true})
	w.Tick(.05)
	if p.z >= -88.35 {
		t.Fatal("boosted sprint tunneled through the cabin's back wall")
	}
}

func TestClosingIslandVisitsReturnsVisitorsToMain(t *testing.T) {
	w, p := testWorld()
	p.profile.HomeIsland = &Island{Owner: "one", Visitors: true, Builders: map[string]bool{}}
	w.Join("two", "Visitor")
	v := w.players["two"]
	w.setIsland(v, "one")
	mustAct(t, w, Action{Action: "island_visitors", Enabled: false})
	if v.island != "" || p.profile.HomeIsland.Visitors {
		t.Fatal("closing visits left a visitor on the island")
	}
}

func TestLegacyHomesMigrateToIslandsWithoutLosingMaterials(t *testing.T) {
	w, p := testWorld()
	p.profile.Home = &Home{Owner: "one", Kind: "tent", X: -85, Z: -90, Health: 120, MaxHealth: 120}
	w.Leave("one")
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	q := restored.players["one"]
	if q.profile.Home != nil || q.profile.HomeIsland == nil || q.profile.Recovery["wood"] != 6 || q.profile.Recovery["stick"] != 8 || q.profile.Recovery["stone"] != 4 {
		t.Fatal("legacy home did not migrate to an island with material recovery")
	}
	again := New(restored.Profiles())
	again.Join("one", "Baker")
	if again.players["one"].profile.Recovery["wood"] != 6 {
		t.Fatal("restarting repeated the legacy material refund")
	}
}

func TestShelterFootprintAndCampfirePlacementRespectObstacles(t *testing.T) {
	w, p := testWorld()
	gain(p, "wood", 30)
	gain(p, "stick", 20)
	gain(p, "stone", 20)
	p.x, p.z = -85, -93.5
	w.colliders = []Collider{{X: -82.25, Z: -87.9, Radius: .1, Height: 3}}
	if err := w.Act("one", Action{Action: "build_home", Item: "cabin"}); err == nil || p.profile.Home != nil || p.profile.Inventory["wood"] != 30 {
		t.Fatal("cabin corner overlapped an obstacle or failed placement spent resources")
	}
	w.colliders = []Collider{{X: -83, Z: -93.5, Radius: .2, Height: 3}}
	if err := w.Act("one", Action{Action: "craft", Item: "fire"}); err == nil || p.profile.Inventory["wood"] != 30 {
		t.Fatal("campfire placed inside an obstacle")
	}
	w.colliders = nil
	p.z = -79.5
	if err := w.Act("one", Action{Action: "build_home", Item: "cabin"}); err == nil {
		t.Fatal("cleared ground bypassed home-island requirement")
	}
}

func TestFirstBakeryUpgradeHasVisibleBoundedBenefits(t *testing.T) {
	w, p := testWorld()
	p.profile.Coins = 1000
	nodePosition(w, p, "bakery_plot_0")
	mustAct(t, w, Action{Action: "buy_land", Target: "bakery_plot_0"})
	mustAct(t, w, Action{Action: "upgrade"})
	nodePosition(w, p, "bakery_plot_0_mixer")
	baseBakingIngredients(p, 1)
	mustAct(t, w, Action{Action: "make_dough"})
	if job := p.profile.Production["bakery_mixer"].Job; job == nil || job.Amount != 40 || job.Remaining != 40 || p.profile.Inventory["dough"] != 0 {
		t.Fatal("first upgrade changed the fixed forty-dough, forty-second recipe")
	}
	advance(w, 40)
	mustAct(t, w, Action{Action: "production_collect", Target: "bakery_plot_0_mixer", Item: "dough", Amount: 40})
	holdTestItem(t, p, "dough")
	nodePosition(w, p, "bakery_plot_0_oven")
	mustAct(t, w, Action{Action: "craft", Item: "sugar", Amount: 20})
	if p.profile.Production["bakery_oven"].Job.Remaining != 100 || p.profile.Inventory["sugar"] != 10 {
		t.Fatal("upgraded oven skipped the five-second timer per unit")
	}
	advance(w, 100)
	nodePosition(w, p, "bakery_plot_0_cooling_table")
	mustAct(t, w, Action{Action: "production_collect", Target: "bakery_plot_0_cooling_table", Item: "sugar", Amount: 20})
	if p.profile.Inventory["sugar"] != 30 {
		t.Fatal("upgraded oven changed the one-cookie-per-dough yield")
	}
	nodePosition(w, p, "bakery_plot_0_display")
	mustAct(t, w, Action{Action: "trade", Target: "bakery_plot_0_display", Item: "sugar", Amount: 20})
	if p.profile.Coins != 700 || p.profile.BakeryDisplay["sugar"] != 20 {
		t.Fatal("stocking a display must not pay coins before a customer purchases")
	}
	p.bakery.nextCustomer = w.time
	advance(w, 6)
	if len(p.bakery.customers) != 1 || p.bakery.customers[0].Phase != "waiting" {
		t.Fatal("stocked bakery did not receive a waiting customer")
	}
	mustAct(t, w, Action{Action: "bakery_serve", Target: p.bakery.customers[0].ID})
	if p.profile.Coins != 711 || p.profile.BakeryDisplay["sugar"] != 15 {
		t.Fatal("customer batch sale did not include the bakery upgrade bonus")
	}
	p.profile.BakeryLevel = 1000000
	if price := bakerySalePrice(p, "sugar", 5); bakeryBoost(p) >= 1 || price <= 10 || price > 15 {
		t.Fatal("large upgrade counts escaped the bounded customer sale bonus")
	}
}

func TestAnimalHitsFollowJumpHeight(t *testing.T) {
	w, _ := testWorld()
	a := &Animal{ID: "jumping", Species: "dolphin", X: -80, Z: 100, Y: 2.5, Scale: 1, Health: 100, MaxHealth: 100}
	w.animals = []*Animal{a}
	w.projectiles = []*Projectile{{Owner: "one", Item: "sugar", X: -80, Y: 2.6, Z: 101, VZ: -10, damage: 18, born: w.time}}
	w.moveProjectiles(.2)
	if a.Health != 82 {
		t.Fatal("a visible jumping animal could not be hit at its actual height")
	}
	w.projectiles = []*Projectile{{Owner: "one", Item: "sugar", X: -80, Y: .1, Z: 101, VZ: -10, damage: 18, born: w.time}}
	w.moveProjectiles(.2)
	if a.Health != 82 {
		t.Fatal("jumping animal was hit by a shot passing below it")
	}
}

func TestBakeryDisplayBlocksMovementAndShotsOnlyWhileClaimed(t *testing.T) {
	w, p := testWorld()
	p.profile.Coins = 1000
	nodePosition(w, p, "bakery_plot_0")
	x, z := p.x+5, p.z+3
	if w.solidBlocked(x, z, .42) {
		t.Fatal("unbought display has an invisible collision box")
	}
	mustAct(t, w, Action{Action: "buy_land", Target: "bakery_plot_0"})
	if !w.solidBlocked(x, z, .42) {
		t.Fatal("base bakery display is not solid")
	}
	nodePosition(w, p, "kitchen_shop")
	mustAct(t, w, Action{Action: "trade", Target: "kitchen_shop", Item: "display"})
	if !w.solidBlocked(x, z, .42) {
		t.Fatal("purchased display is not solid")
	}
	p.x, p.z = x, z+1.1
	w.Input("one", Input{Z: -1})
	w.Tick(.2)
	if p.z < z+.82 {
		t.Fatal("player walked through a purchased display")
	}
	w.Join("two", "Target")
	victim := w.players["two"]
	victim.x, victim.z = x, z-2
	victim.buffs = map[string]float64{}
	w.projectiles = []*Projectile{{Owner: "one", Item: "sugar", X: x, Y: 1, Z: z + 2, VZ: -50, damage: 18, born: w.time}}
	w.moveProjectiles(.1)
	if victim.health != 100 || len(w.projectiles) != 0 {
		t.Fatal("projectile passed through a purchased display")
	}
	w.Leave("one")
	if w.solidBlocked(x, z, .42) {
		t.Fatal("released bakery left an invisible display collider")
	}
	restored := New(w.Profiles())
	if restored.solidBlocked(x, z, .42) {
		t.Fatal("restart restored an offline display to an unclaimed plot")
	}
}
