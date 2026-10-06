package game

import (
	"math"
	"testing"
)

func testWorld() (*World, *player) {
	w := New(nil)
	w.Join("one", "Baker")
	p := w.players["one"]
	p.buffs = map[string]float64{}
	return w, p
}
func setItem(p *player, item string, count int) {
	take(p, item, p.profile.Inventory[item])
	gain(p, item, count)
	if _, cookie := recipeFor(item); cookie && count > 0 && !hasHotbar(p, item) {
		for i, s := range p.profile.Hotbar {
			if s.Count == 0 {
				for j, b := range p.profile.BagSlots {
					if b.Item == item {
						p.profile.Hotbar[i] = b
						p.profile.BagSlots[j] = Stack{}
						syncInventory(&p.profile)
						return
					}
				}
			}
		}
	}
}
func findAnimal(w *World, species string) *Animal {
	for _, a := range w.animals {
		if a.Species == species {
			return a
		}
	}
	panic("missing animal")
}
func mustAct(t *testing.T, w *World, a Action) {
	t.Helper()
	if err := w.Act("one", a); err != nil {
		t.Fatalf("action %+v: %v", a, err)
	}
}
func advance(w *World, seconds float64) {
	for seconds > 0 {
		dt := min(.1, seconds)
		w.Tick(dt)
		seconds -= dt
	}
}

func TestJoiningPersistenceAndDetachedCopies(t *testing.T) {
	w, p := testWorld()
	if p.profile.Inventory["sugar"] != 10 || p.profile.Coins != 0 {
		t.Fatal("new account must receive exactly ten sugar cookies, no coins")
	}
	setItem(p, "sugar", 3)
	p.profile.Coins = 17
	p.profile.Protected = []string{"sugar"}
	snapshot := w.Snapshot("one")
	snapshot.Me.Inventory["sugar"] = 999
	snapshot.Me.Protected[0] = "cake"
	snapshot.Recipes[0].Cost["dough"] = 99
	if p.profile.Inventory["sugar"] != 3 || p.profile.Protected[0] != "sugar" || recipes[0].Cost["dough"] != 1 {
		t.Fatal("snapshot aliases mutable world data")
	}
	profiles := w.Profiles()
	profiles["one"].Inventory["sugar"] = 500
	if p.profile.Inventory["sugar"] != 3 {
		t.Fatal("profile export aliases inventory")
	}
	w.Leave("one")
	w.Join("one", "Baker")
	if got := w.players["one"].profile.Inventory["sugar"]; got != 3 {
		t.Fatalf("reconnect granted starter cookies: %d", got)
	}
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	if got := restored.Snapshot("one").Me; got.Coins != 17 || got.Inventory["sugar"] != 3 {
		t.Fatalf("lost saved progression: %+v", got)
	}
}

func TestMovementIsAuthoritativeAndFinite(t *testing.T) {
	w, p := testWorld()
	p.x = -50
	p.z = -50
	w.Input("one", Input{X: 1, Z: 1, Yaw: math.Pi / 2})
	w.Tick(.2)
	if got := distance(p.x, p.z, -50, -50); math.Abs(got-1.2) > 1e-6 {
		t.Fatalf("diagonal movement speed = %v", got)
	}
	x, z := p.x, p.z
	w.Input("one", Input{X: math.NaN(), Z: math.Inf(1), Yaw: 0})
	w.Tick(.05)
	if !finite(p.x) || !finite(p.z) || p.x < x || p.z < z {
		t.Fatal("invalid input corrupted world")
	}
	w.Input("one", Input{X: 0, Z: 0})
	advance(w, 2)
	x, z = p.x, p.z
	w.Tick(math.NaN())
	w.Tick(-1)
	if p.x != x || p.z != z {
		t.Fatal("invalid time step changed movement")
	}
	p.x = MaxX - 1.1
	w.Input("one", Input{X: 1000, Sprint: true})
	w.Tick(.2)
	if p.x > MaxX-1 {
		t.Fatal("player escaped world bounds")
	}
	advance(w, 1)
	x = p.x
	advance(w, 1)
	if p.x != x {
		t.Fatal("stale input continued moving player")
	}
}

func TestReconnectPreservesHealthPositionAndProductionCooldown(t *testing.T) {
	w, p := testWorld()
	p.profile.Bakery = true
	p.profile.BakeryPlot = "bakery_plot_0"
	p.x = 24
	p.z = 72
	p.health = 35
	mustAct(t, w, Action{Action: "make_dough"})
	w.Input("one", Input{X: 1, Sprint: true})
	w.Leave("one")
	advance(w, 1)
	w.Join("one", "Baker")
	p = w.players["one"]
	if p.health != 35 || p.x != 24 || p.z != 72 {
		t.Fatal("reconnect healed or relocated player")
	}
	if p.input.X != 0 || p.input.Sprint {
		t.Fatal("reconnect retained movement input")
	}
	if err := w.Act("one", Action{Action: "make_dough"}); err == nil {
		t.Fatal("reconnect bypassed production cooldown")
	}
	if w.active(p, "spawn_shield") {
		t.Fatal("reconnect granted new combat protection")
	}
	advance(w, 8)
	mustAct(t, w, Action{Action: "make_dough"})
	if p.profile.Inventory["dough"] != 4 {
		t.Fatal("production did not resume after original cooldown")
	}
}

func TestGatherRangeAndRenewal(t *testing.T) {
	w, p := testWorld()
	var node *Node
	for _, n := range w.nodes {
		if n.respawn > 0 && n.Kind != "chest" {
			node = n
			break
		}
	}
	p.x = 90
	p.z = 90
	if err := w.Act("one", Action{Action: "interact", Target: node.ID}); err == nil {
		t.Fatal("remote gathering allowed")
	}
	p.x = node.X
	p.z = node.Z
	mustAct(t, w, Action{Action: "interact", Target: node.ID})
	if p.profile.Inventory[node.Kind] != node.Amount || node.Available {
		t.Fatal("gather did not consume node or grant inventory")
	}
	advance(w, .5)
	if err := w.Act("one", Action{Action: "interact", Target: node.ID}); err == nil {
		t.Fatal("depleted node could be gathered")
	}
	advance(w, 26)
	if !node.Available {
		t.Fatal("resource never renewed")
	}
}

func TestFireAndRecipesChargeIngredients(t *testing.T) {
	w, p := testWorld()
	p.x = -60
	p.z = -60
	setItem(p, "dough", 8)
	if err := w.Act("one", Action{Action: "craft", Item: "sugar"}); err == nil {
		t.Fatal("baking without fire allowed")
	}
	setItem(p, "wood", 2)
	setItem(p, "stick", 2)
	setItem(p, "stone", 3)
	mustAct(t, w, Action{Action: "craft", Item: "fire"})
	advance(w, 1.1)
	if p.profile.Inventory["wood"] != 0 || p.profile.Inventory["stone"] != 0 {
		t.Fatal("fire did not charge resources")
	}
	mustAct(t, w, Action{Action: "craft", Item: "sugar", Amount: 3})
	if p.profile.Inventory["dough"] != 5 || p.profile.Inventory["sugar"] != 13 {
		t.Fatal("recipe inventory accounting wrong")
	}
	advance(w, 1.1)
	if err := w.Act("one", Action{Action: "craft", Item: "berry_cookie"}); err == nil {
		t.Fatal("crafted without berries")
	}
	if p.profile.Inventory["dough"] != 5 {
		t.Fatal("failed recipe consumed dough")
	}
	p.x = 30
	p.z = -30
	setItem(p, "cactus", 2)
	setItem(p, "berry", 2)
	mustAct(t, w, Action{Action: "craft", Item: "sun_cookie"})
	if p.profile.Inventory["sun_cookie"] != 1 {
		t.Fatal("desert heat did not bake sun cookie")
	}
	for _, amount := range []int{-2, 21, math.MaxInt} {
		if err := w.Act("one", Action{Action: "craft", Item: "sugar", Amount: amount}); err == nil {
			t.Fatalf("invalid amount %d accepted", amount)
		}
	}
}

func TestCookieHealingBuffsAndThrowCooldown(t *testing.T) {
	w, p := testWorld()
	p.health = 40
	setItem(p, "protein_cookie", 1)
	mustAct(t, w, Action{Action: "eat", Item: "sugar"})
	if p.health != 58 || p.profile.Inventory["sugar"] != 9 {
		t.Fatal("sugar healing incorrect")
	}
	if w.active(p, "superstrength") || w.active(p, "speed") {
		t.Fatal("plain sugar added an ability")
	}
	advance(w, 1.1)
	mustAct(t, w, Action{Action: "eat", Item: "protein_cookie"})
	if !w.active(p, "superstrength") {
		t.Fatal("protein cookie did not activate strength")
	}
	mustAct(t, w, Action{Action: "throw", Item: "sugar"})
	if len(w.projectiles) != 1 || p.profile.Inventory["sugar"] != 8 {
		t.Fatal("throw did not launch and consume cookie")
	}
	if err := w.Act("one", Action{Action: "throw", Item: "sugar"}); err == nil {
		t.Fatal("rapid throw bypassed cooldown")
	}
	advance(w, 10.1)
	if w.active(p, "superstrength") {
		t.Fatal("superstrength exceeded ten seconds")
	}
}

func TestProjectilesHitPlayersAndApplyEffects(t *testing.T) {
	w, p := testWorld()
	w.Join("two", "Target")
	target := w.players["two"]
	target.buffs = map[string]float64{}
	p.x = -60
	p.z = -60
	p.yaw = 0
	target.x = -60
	target.z = -68
	setItem(p, "berry_cookie", 1)
	mustAct(t, w, Action{Action: "throw", Item: "berry_cookie"})
	advance(w, .4)
	if target.health != 80 || !w.active(target, "slowed") {
		t.Fatalf("expected 20 damage and slow; health=%v buffs=%v", target.health, target.buffs)
	}
	if len(w.projectiles) != 0 {
		t.Fatal("projectile survived hitting player")
	}
	if event := w.events[len(w.events)-1]; event.Kind != "hit" || event.Actor != "one" {
		t.Fatalf("hit confirmation attributed to wrong player: %+v", event)
	}
	advance(w, 1)
	setItem(p, "nut_cookie", 1)
	mustAct(t, w, Action{Action: "throw", Item: "nut_cookie"})
	advance(w, .4)
	if !w.active(target, "weakened") {
		t.Fatal("nut cookie did not weaken target")
	}
	advance(w, 1)
	setItem(p, "protein_cookie", 1)
	mustAct(t, w, Action{Action: "eat", Item: "protein_cookie"})
	mustAct(t, w, Action{Action: "throw", Item: "sugar"})
	advance(w, .4)
	if target.profile.Deaths != 1 || target.health != 100 || area(target.x, target.z) != "forest" {
		t.Fatal("superstrength hit did not defeat and respawn target")
	}
	if event := w.events[len(w.events)-1]; event.Kind != "defeat" || event.Actor != "one" {
		t.Fatalf("defeat confirmation attributed to wrong player: %+v", event)
	}
}

func TestProteinExpiryIsCheckedWhenProjectileHits(t *testing.T) {
	w, p := testWorld()
	w.Join("two", "Target")
	target := w.players["two"]
	target.buffs = map[string]float64{}
	p.x = -60
	p.z = -60
	target.x = -60
	target.z = -68
	p.buffs["superstrength"] = w.time + .15
	mustAct(t, w, Action{Action: "throw", Item: "sugar"})
	advance(w, .4)
	if target.profile.Deaths != 0 || target.health != 82 {
		t.Fatalf("expired strength affected impact: deaths=%d health=%v", target.profile.Deaths, target.health)
	}
}

func TestProteinRecipeAcceptsBothProducts(t *testing.T) {
	w, p := testWorld()
	p.x = 29
	p.z = -30
	setItem(p, "dough", 3)
	setItem(p, "protein_powder", 1)
	setItem(p, "protein_drink", 2)
	mustAct(t, w, Action{Action: "craft", Item: "protein_cookie", Amount: 3})
	if p.profile.Inventory["protein_cookie"] != 3 || p.profile.Inventory["protein_powder"] != 0 || p.profile.Inventory["protein_drink"] != 0 || p.profile.Inventory["dough"] != 0 {
		t.Fatal("protein recipe did not combine and charge both protein products")
	}
}

func TestPeaceTradesExactTrashWeightAndChecksRange(t *testing.T) {
	w, p := testWorld()
	setItem(p, "trash", 4)
	if err := w.Act("one", Action{Action: "trade", Target: "peace", Item: "trash", Amount: 4}); err == nil {
		t.Fatal("traded remotely")
	}
	p.x = -30
	p.z = 8
	mustAct(t, w, Action{Action: "trade", Target: "peace", Item: "trash", Amount: 4})
	if p.profile.Coins != 12 || p.profile.Inventory["trash"] != 0 {
		t.Fatal("Peace must pay exactly 3 coins per kg")
	}
	if err := w.Act("one", Action{Action: "trade", Target: "peace", Item: "trash"}); err == nil {
		t.Fatal("sold nonexistent trash")
	}
	p.x = 32
	p.z = -30
	setItem(p, "berry", 3)
	mustAct(t, w, Action{Action: "trade", Target: "desert_trader", Item: "berry"})
	if p.profile.Inventory["berry"] != 0 || p.profile.Inventory["sun_cookie"] != 2 {
		t.Fatal("desert barter accounting wrong")
	}
}

func TestGymLocationCapsAndProteinRecovery(t *testing.T) {
	w, p := testWorld()
	p.profile.Coins = 100
	if err := w.Act("one", Action{Action: "train", Item: "arms"}); err == nil {
		t.Fatal("remote training accepted")
	}
	p.x = 45
	p.z = 20
	setItem(p, "protein_drink", 1)
	mustAct(t, w, Action{Action: "consume_protein", Item: "protein_drink"})
	for _, kind := range []string{"legs", "stamina", "arms", "swim"} {
		mustAct(t, w, Action{Action: "train", Item: kind})
		if err := w.Act("one", Action{Action: "train", Item: kind}); err == nil {
			t.Fatal("training recovery bypassed")
		}
		advance(w, 1.6)
	}
	if p.profile.Coins != 92 || p.profile.Levels["swim"] != 1 {
		t.Fatal("gym accounting incorrect")
	}
	p.profile.Levels["arms"] = 10
	if err := w.Act("one", Action{Action: "train", Item: "arms"}); err == nil {
		t.Fatal("training exceeded stat cap")
	}
}

func TestAnimalsRememberWitnessedKillsAndKindness(t *testing.T) {
	w, p := testWorld()
	victim := findAnimal(w, "turtle")
	victim.Need = "trapped"
	witness := findAnimal(w, "shark")
	distant := findAnimal(w, "whale")
	victim.X = -30
	victim.Z = 50
	witness.X = -31
	witness.Z = 51
	distant.X = -90
	distant.Z = 95
	p.x = -30
	p.z = 50
	mustAct(t, w, Action{Action: "interact", Target: victim.ID})
	if victim.Need != "" || p.profile.Reputation[victim.ID] != 3 || p.profile.Inventory["pearl"] != 1 {
		t.Fatal("turtle rescue did not grant trust and pearl")
	}
	if p.profile.Reputation[witness.ID] != 1 {
		t.Fatal("nearby witness did not remember kindness")
	}
	w.damageAnimal(p, victim, 1000)
	if p.profile.Reputation[witness.ID] >= 0 {
		t.Fatal("witness did not remember killing")
	}
	if p.profile.Reputation[distant.ID] != 0 {
		t.Fatal("animal remembered a killing it could not witness")
	}
	advance(w, .1)
	if witness.Disposition != "hunting" {
		t.Fatal("shark did not seek revenge")
	}
	saved := w.Profiles()
	restored := New(saved)
	restored.Join("one", "Baker")
	if restored.players["one"].profile.Reputation[witness.ID] >= 0 {
		t.Fatal("animal memory was not persisted")
	}
}

func TestPersistedProfilesAreSanitized(t *testing.T) {
	w := New(map[string]Profile{"one": {Coins: -1, Inventory: map[string]int{"sugar": -1, "cake": inventoryLimit + 10, "unknown": 123}, Levels: map[string]int{"arms": 999}, Protected: []string{"sugar", "sugar", "wood", "cake"}, Selected: "wood"}})
	w.Join("one", "Baker")
	p := w.players["one"]
	if p.profile.Coins != 0 || p.profile.Inventory["cake"] != inventoryLimit || p.profile.Levels["arms"] != 10 || p.profile.Selected != "" {
		t.Fatal("invalid save data escaped normalization")
	}
	if _, ok := p.profile.Inventory["unknown"]; ok {
		t.Fatal("unknown item restored")
	}
	if len(p.profile.Protected) != 1 {
		t.Fatal("invalid protected slots restored")
	}
}
