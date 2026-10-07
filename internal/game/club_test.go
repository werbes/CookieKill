package game

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func clubWorld() (*World, *player) {
	w, p := testWorld()
	p.x, p.z, p.inCave = -191, -237, true
	p.profile.Coins = 1000
	return w, p
}

func TestClubWheelExactRequestedProbabilities(t *testing.T) {
	counts := [7]int{}
	for roll := 0; roll < 100; roll++ {
		sector := clubSectorForRoll(roll)
		if sector < 0 || sector >= len(counts) {
			t.Fatalf("roll %d has no outcome", roll)
		}
		counts[sector]++
	}
	if counts != [7]int{7, 10, 10, 10, 30, 30, 3} {
		t.Fatalf("wrong odds: %v", counts)
	}
	for _, roll := range []int{-1, 100, 101} {
		if clubSectorForRoll(roll) != -1 {
			t.Fatal("out-of-range entropy accepted")
		}
	}
}

func TestClubWheelAllSectorsExactWholeCoinPayouts(t *testing.T) {
	for sector, multiplier := range []int{50, 30, 20, 10, 5, 1, 100} {
		w, p := clubWorld()
		w.clubRandom = func() (int, error) { return sector, nil }
		stake := 37
		mustAct(t, w, Action{Action: "club_gamble", Target: "club_casino", Amount: stake})
		payout := stake * multiplier / 10
		if p.profile.Coins != 1000-stake+payout || p.clubResult == nil || p.clubResult.Sector != sector || p.clubResult.Payout != payout || p.clubResult.Stake != stake || p.clubResult.Multiplier != float64(multiplier)/10 {
			t.Fatalf("sector %d incorrect result: coins=%d result=%+v", sector, p.profile.Coins, p.clubResult)
		}
		before := p.profile.Coins
		if err := w.Act("one", Action{Action: "club_gamble", Target: "club_casino", Amount: stake}); err == nil || p.profile.Coins != before {
			t.Fatal("immediate repeated spin changed coins")
		}
		w.time += 1
		mustAct(t, w, Action{Action: "club_gamble", Target: "club_casino", Amount: p.profile.Coins})
		if p.profile.Coins != before*multiplier/10 {
			t.Fatal("full wallet stake did not pay exactly")
		}
	}
}

func TestClubWheelValidationAndRandomFailureAreAtomic(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount int
		setup  func(*World, *player)
	}{
		{"zero", 0, nil}, {"negative", -1, nil}, {"over wallet", 1001, nil},
		{"surface", 10, func(w *World, p *player) { p.inCave = false }},
		{"desert cave", 10, func(w *World, p *player) { p.x, p.z = 120, -117 }},
		{"island", 10, func(w *World, p *player) { p.island = "one" }},
		{"distant", 10, func(w *World, p *player) { p.x = -212 }},
		{"wrong height", 10, func(w *World, p *player) { p.jumpOffset = 5 }},
		{"entropy unavailable", 10, func(w *World, p *player) { w.clubRandom = func() (int, error) { return 0, errors.New("entropy") } }},
		{"invalid sector", 10, func(w *World, p *player) { w.clubRandom = func() (int, error) { return 7, nil } }},
		{"numerical cap", 1, func(w *World, p *player) { p.profile.Coins = coinLimit }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, p := clubWorld()
			if tc.setup != nil {
				tc.setup(w, p)
			}
			before := p.profile.Coins
			if err := w.Act("one", Action{Action: "club_gamble", Target: "club_casino", Amount: tc.amount}); err == nil {
				t.Fatal("invalid bet accepted")
			}
			if p.profile.Coins != before || p.clubResult != nil || p.cooldowns["club_gamble"] != 0 {
				t.Fatal("failed bet changed wallet/result/cooldown")
			}
		})
	}
}

func TestClubWinningsPersistAboveOldCoinLimitWithoutClipping(t *testing.T) {
	w, p := clubWorld()
	p.profile.Coins = inventoryLimit
	w.clubRandom = func() (int, error) { return 6, nil }
	mustAct(t, w, Action{Action: "club_gamble", Target: "club_casino", Amount: inventoryLimit})
	if p.profile.Coins != 10*inventoryLimit {
		t.Fatal("large jackpot clipped")
	}
	addCoins(p, 3)
	if p.profile.Coins != 10*inventoryLimit+3 {
		t.Fatal("ordinary reward reduced large wallet")
	}
	saved := New(w.Profiles()).Profiles()["one"]
	if saved.Coins != p.profile.Coins {
		t.Fatal("restart clipped winnings")
	}
}

func TestClubSafeNumberBoundaryAndPrivateDetachedResults(t *testing.T) {
	w, p := clubWorld()
	p.profile.Coins = coinLimit - 9
	w.clubRandom = func() (int, error) { return 6, nil }
	mustAct(t, w, Action{Action: "club_gamble", Target: "club_casino", Amount: 1})
	if p.profile.Coins != coinLimit {
		t.Fatal("safe-limit payout not exact")
	}
	addCoins(p, coinLimit)
	if p.profile.Coins != coinLimit {
		t.Fatal("ordinary reward overflowed")
	}
	if got := sanitize(Profile{Coins: coinLimit + 1}).Coins; got != coinLimit {
		t.Fatal("corrupt wallet not bounded")
	}
	w.Join("two", "Neighbor")
	if w.Snapshot("two").Me.ClubResult != nil {
		t.Fatal("another player's wheel result leaked")
	}
	snapshot := w.Snapshot("one")
	snapshot.Me.ClubResult.Payout = 999
	snapshot.ClubDrinks[0].Price = 0
	if p.clubResult.Payout != 10 || clubDrinks[0].Price != 8 {
		t.Fatal("snapshot aliases live result or catalog")
	}
}

func TestClubBarPurchasesConsumptionAndPersistentGeneralStorage(t *testing.T) {
	w, p := clubWorld()
	p.x, p.z = -213, -232
	for _, drink := range clubDrinks {
		before := p.profile.Coins
		mustAct(t, w, Action{Action: "buy_drink", Target: "club_bar", Item: drink.ID, Amount: 2})
		if p.profile.Coins != before-2*drink.Price || p.profile.Inventory[drink.ID] != 2 {
			t.Fatalf("wrong price/items for %s", drink.ID)
		}
		p.health, p.stamina = 50, 40
		mustAct(t, w, Action{Action: "drink", Item: drink.ID})
		if p.health != 50+drink.Heal || p.stamina != 40+drink.Stamina || p.profile.Inventory[drink.ID] != 1 {
			t.Fatalf("wrong drink effect for %s", drink.ID)
		}
		w.time += 1
	}
	saved := New(w.Profiles()).Profiles()["one"]
	for _, drink := range clubDrinks {
		if saved.Inventory[drink.ID] != 1 {
			t.Fatalf("drink %s did not persist", drink.ID)
		}
	}
	p.inCave = false
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "buy_drink", Target: "club_bar", Item: "beer"}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("surface purchase accepted or changed inventory")
	}
	// Drinking does not require returning to the bar, and the ordinary hotbar
	// eat action recognizes drinks too.
	mustAct(t, w, Action{Action: "eat", Item: "water"})
	if p.profile.Inventory["water"] != 0 {
		t.Fatal("hotbar eat did not consume drink")
	}
}

func TestClubBarFullInventoryAndInsufficientCoinsDoNotCharge(t *testing.T) {
	w, p := clubWorld()
	p.x, p.z = -213, -232
	for i := range p.profile.BagSlots {
		p.profile.BagSlots[i] = Stack{Item: "wood", Count: inventoryLimit}
	}
	syncInventory(&p.profile)
	before := p.profile.Coins
	if err := w.Act("one", Action{Action: "buy_drink", Target: "club_bar", Item: "beer"}); err == nil || p.profile.Coins != before {
		t.Fatal("full bag charged for unavailable item")
	}
	p.profile.BagSlots[0] = Stack{}
	syncInventory(&p.profile)
	p.profile.Coins = 1
	if err := w.Act("one", Action{Action: "buy_drink", Target: "club_bar", Item: "water"}); err == nil || p.profile.Inventory["water"] != 0 || p.profile.Coins != 1 {
		t.Fatal("unaffordable drink changed economy")
	}
}

func TestClubSeatsExclusiveAndReleasedByMovementAndTravel(t *testing.T) {
	w, p := clubWorld()
	p.x, p.z = -213, -243
	mustAct(t, w, Action{Action: "club_sit", Target: "club_seat_1"})
	if p.posture != "sitting" || p.clubSeatID != "club_seat_1" || groundY(p) != ClubFloorY || p.x != -213 || p.z != -245 {
		t.Fatal("sitting did not use chair floor position")
	}
	w.Join("two", "Visitor")
	other := w.players["two"]
	other.x, other.z, other.inCave = -213, -243, true
	if err := w.Act("two", Action{Action: "club_sit", Target: "club_seat_1"}); err == nil {
		t.Fatal("two players occupy one chair")
	}
	w.Input("one", Input{X: 1})
	w.Tick(.05)
	if p.clubSeatID != "" || p.posture != "" {
		t.Fatal("moving did not release chair")
	}
	if err := w.Act("two", Action{Action: "club_sit", Target: "club_seat_1"}); err != nil {
		t.Fatal(err)
	}
	if err := w.Act("two", Action{Action: "interact", Target: "club_seat_1"}); err != nil || other.posture != "" {
		t.Fatal("E did not toggle chair")
	}
	mustAct(t, w, Action{Action: "club_sit", Target: "club_seat_1"})
	w.Leave("one")
	if err := w.Act("two", Action{Action: "club_sit", Target: "club_seat_1"}); err != nil {
		t.Fatal("disconnect did not release chair:", err)
	}
	w.Join("one", "Returning visitor")
	if p.clubSeatID != "" || p.posture != "" {
		t.Fatal("reconnecting stole another player's chair")
	}
	if err := w.Act("two", Action{Action: "club_sit", Target: "club_seat_1"}); err != nil {
		t.Fatal(err)
	}
	mustAct(t, w, Action{Action: "club_sit", Target: "club_seat_1"})
	w.setIsland(p, "")
	if p.clubSeatID != "" || p.posture != "" {
		t.Fatal("travel retained chair")
	}
}

func TestForestClubApproachStairsLayersAndFurniture(t *testing.T) {
	w, p := testWorld()
	p.x, p.z = -35, -192
	for _, target := range []Point{{ClubX, -192}, {ClubX, -230}, {-213, -232}, {-202, -238}, {-191, -237}, {ClubX, -230}, {ClubX, -200}} {
		for i := 0; i < 700 && distance(p.x, p.z, target.X, target.Z) > .3; i++ {
			dx, dz := target.X-p.x, target.Z-p.z
			d := math.Hypot(dx, dz)
			w.Input("one", Input{X: dx / d, Z: dz / d})
			w.Tick(.05)
		}
		if distance(p.x, p.z, target.X, target.Z) > .3 {
			t.Fatalf("club path blocked at(%v,%v), target%+v", p.x, p.z, target)
		}
		if target.Z < -227 && (!p.inCave || groundY(p) != ClubFloorY) {
			t.Fatal("club arrival did not reach underground floor")
		}
	}
	if p.inCave || groundY(p) != 0 {
		t.Fatal("club stairs did not exit to forest")
	}
	p.x, p.z, p.inCave = -202, -238, false
	if !w.moveMainland(p, -201.8, -238, 0) || p.inCave || groundY(p) != 0 {
		t.Fatal("surface player fell through club roof")
	}
	p.inCave = true
	if w.moveMainland(p, -220, -238, ClubFloorY) {
		t.Fatal("walked out through underground chamber wall")
	}
	for _, point := range []struct{ x, z, y float64 }{{ClubX, -207, 0}, {ClubX, -217, -6}, {ClubX, -227, -12}, {-213, -245, -12}, {120, -117, CaveFloorY}} {
		if y := caveGroundY(point.x, point.z); y != point.y {
			t.Fatalf("ground at %+v=%v", point, y)
		}
	}
	if !w.solidBlockedAt(-215.5, ClubFloorY, -232, .42) || !w.solidBlockedAt(-188.5, ClubFloorY, -237, .42) {
		t.Fatal("club furniture has no collision")
	}
	if w.layout.Version != 9 {
		t.Fatal("club map change did not invalidate cached layout")
	}
}

func TestClubCamelsAndDroppedItemsStayOnUndergroundLayer(t *testing.T) {
	w, p := clubWorld()
	p.x, p.z = -202, -238
	p.profile.Camels = []Camel{{ID: "one_camel_1", X: p.x, Y: ClubFloorY, Z: p.z, InCave: true}}
	normalizeCamels(&p.profile)
	if p.profile.Camels[0].Y != ClubFloorY || !p.profile.Camels[0].InCave {
		t.Fatal("parked club camel reset during restore")
	}
	setItem(p, "water", 5)
	slot := -1
	for i, s := range p.profile.BagSlots {
		if s.Item == "water" {
			slot = i
		}
	}
	mustAct(t, w, Action{Action: "drop_item", From: "bag", FromSlot: slot, Amount: 5})
	for id, drop := range w.drops {
		if !drop.node.InCave || drop.node.Y != ClubFloorY {
			t.Fatal("club drop has wrong layer/height")
		}
		p.inCave = false
		if err := w.Act("one", Action{Action: "pickup_item", Target: id}); err == nil {
			t.Fatal("picked club drink through roof")
		}
		p.inCave = true
		mustAct(t, w, Action{Action: "pickup_item", Target: id})
	}
	if p.profile.Inventory["water"] != 5 {
		t.Fatal("club drink drop/pickup lost items")
	}
}

func TestClubTablesBlockWalkingAndLeaveChairsAccessible(t *testing.T) {
	w, p := clubWorld()
	for _, x := range []float64{-213, -209, -205, -201} {
		if !w.solidBlockedAt(x, ClubFloorY, -246.5, .42) || w.solidBlockedAt(x, ClubFloorY+1, -246.5, .42) {
			t.Fatal("table collider does not match its footprint and height")
		}
		for _, z := range []float64{-245, -248} {
			if w.solidBlockedAt(x, ClubFloorY, z, .42) {
				t.Fatal("table collider obstructs a chair")
			}
		}
	}
	p.x, p.z = -213, -244
	for i := 0; i < 20; i++ {
		w.Input("one", Input{Z: -1})
		w.Tick(.05)
	}
	if p.z < -245.53 || p.z > -245 {
		t.Fatalf("walking did not stop at the round tabletop: z=%v", p.z)
	}
	for _, target := range []Point{{-211.75, -245}, {-211.75, -248}, {-213, -248}} {
		for i := 0; i < 100 && distance(p.x, p.z, target.X, target.Z) > .2; i++ {
			dx, dz := target.X-p.x, target.Z-p.z
			d := math.Hypot(dx, dz)
			w.Input("one", Input{X: dx / d, Z: dz / d})
			w.Tick(.05)
		}
		if distance(p.x, p.z, target.X, target.Z) > .2 {
			t.Fatal("table blocked the aisle to a rear chair")
		}
	}
	mustAct(t, w, Action{Action: "club_sit", Target: "club_seat_5"})
}

func TestLargeWalletDeathSharesUseExactIntegerRounding(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		victim, killer, loss int
	}{
		{"five percent rounds down", 9007199254740979, 9007199254740980, 450359962737048},
		{"ten percent rounds up", coinLimit, 0, 900719925474100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, killer := testWorld()
			w.Join("two", "Victim")
			victim := w.players["two"]
			victim.buffs = map[string]float64{}
			victim.profile.Coins, killer.profile.Coins = tc.victim, tc.killer
			if !w.damagePlayer(killer, victim, 1000, "test") {
				t.Fatal("test hit did not defeat victim")
			}
			if victim.profile.Coins != tc.victim-tc.loss || killer.profile.Coins != min(coinLimit, tc.killer+tc.loss) {
				t.Fatalf("inexact coin share: victim=%d killer=%d", victim.profile.Coins, killer.profile.Coins)
			}
		})
	}
}
