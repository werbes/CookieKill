package game

import (
	"reflect"
	"testing"
)

func businessWorld(t *testing.T) (*World, *player) {
	t.Helper()
	w, p := testWorld()
	p.profile.Coins = 1000
	nodePosition(w, p, "bakery_plot_0")
	mustAct(t, w, Action{Action: "buy_land", Target: "bakery_plot_0"})
	return w, p
}
func firstWaitingCustomer(t *testing.T, w *World, p *player) *bakeryCustomer {
	t.Helper()
	for i := 0; i < 300; i++ {
		for _, c := range p.bakery.customers {
			if c.Phase == "waiting" {
				return c
			}
		}
		w.Tick(.1)
	}
	t.Fatal("no customer walked to the stocked display")
	return nil
}
func TestBakeryStockThenRealCustomerPurchaseAndReplay(t *testing.T) {
	w, p := businessWorld(t)
	nodePosition(w, p, "bakery_plot_0_display")
	before := p.profile.Coins
	mustAct(t, w, Action{Action: "bakery_stock", Target: "bakery_plot_0_display", Item: "sugar", Amount: 5})
	if p.profile.Coins != before || p.profile.Inventory["sugar"] != 5 || p.profile.BakeryDisplay["sugar"] != 5 {
		t.Fatal("stocking sold goods immediately or lost items")
	}
	c := firstWaitingCustomer(t, w, p)
	service, _ := w.bakeryStationServicePoint(p, "display", false)
	if c.Item != "sugar" || c.Amount != 5 || distance(c.x, c.z, service.X, service.Z) > .01 {
		t.Fatal("customer order or route wrong")
	}
	if w.Snapshot("one").Me.BakeryBusiness.Customers[0].Price != 10 {
		t.Fatal("customer snapshot omitted the order price")
	}
	p.profile.BakeryLevel = 1
	if w.Snapshot("one").Me.BakeryBusiness.Customers[0].Price != 11 {
		t.Fatal("waiting order price did not reflect the latest bakery upgrade")
	}
	mustAct(t, w, Action{Action: "bakery_serve", Target: c.ID})
	if p.profile.Coins != before+11 || p.profile.BakeryDisplay["sugar"] != 0 || c.Phase != "leaving" {
		t.Fatal("serving did not exchange exact display goods for coins")
	}
	if err := w.Act("one", Action{Action: "bakery_serve", Target: c.ID}); err == nil || p.profile.Coins != before+11 {
		t.Fatal("replayed serving duplicated sale")
	}
	advance(w, 6)
	if len(p.bakery.customers) != 0 {
		t.Fatal("served customer never left")
	}
}
func TestBakeryCustomersRequireDisplayStockAndWaitForExactOrder(t *testing.T) {
	w, p := businessWorld(t)
	advance(w, 25)
	if len(p.bakery.customers) != 0 || p.profile.Coins != 750 {
		t.Fatal("empty display attracted paid inventory sales")
	}
	nodePosition(w, p, "bakery_plot_0_display")
	mustAct(t, w, Action{Action: "bakery_stock", Item: "sugar", Amount: 5})
	c := firstWaitingCustomer(t, w, p)
	mustAct(t, w, Action{Action: "bakery_unstock", Item: "sugar", Amount: 5})
	before := p.profile.Coins
	if err := w.Act("one", Action{Action: "bakery_serve", Target: c.ID}); err == nil || p.profile.Coins != before || c.Phase != "waiting" {
		t.Fatal("customer bought directly from personal bag")
	}
	advance(w, 61)
	if c.Phase != "leaving" || c.Amount != 0 {
		t.Fatal("unserved customer did not leave empty handed")
	}
}
func TestBakeryStaffHiresOnceAndAutomatesAwayFromOwner(t *testing.T) {
	w, p := businessWorld(t)
	for _, role := range []string{"chef", "cashier", "server"} {
		mustAct(t, w, Action{Action: "bakery_hire", Target: "bakery_plot_0", Item: role})
	}
	if p.profile.Coins != 0 {
		t.Fatal("wrong one-time staff prices")
	}
	if p.bakery.chefStatus != "Starting shift" {
		t.Fatal("newly hired chef still appears unhired")
	}
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "bakery_hire", Item: "chef"}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("duplicate hire charged again")
	}
	bag, hotbar := p.profile.BagSlots, p.profile.Hotbar
	p.profile.BakeryPantry = map[string]int{"baking_sugar": 1, "butter": 1, "flour": 1, "egg": 1}
	p.x, p.z = -35, -35
	advance(w, 105)
	if p.profile.Coins <= 0 || !p.profile.Discovered["sugar"] {
		t.Fatal("chef/cashier did not generate real customer sales")
	}
	if p.profile.BagSlots != bag || p.profile.Hotbar != hotbar {
		t.Fatal("staff consumed or filled personal inventory")
	}
	nodes := w.Snapshot("one").Nodes
	staff := 0
	for _, n := range nodes {
		if n.Kind == "bakery_chef" || n.Kind == "bakery_cashier" || n.Kind == "bakery_server" {
			staff++
			if n.Owner != "one" || n.PlotID != "bakery_plot_0" || n.Available {
				t.Fatal("staff node lacks ownership or became gathering resource")
			}
		}
	}
	if staff != 3 {
		t.Fatal("staff did not appear in bakery")
	}
}
func TestBakeryChefUsesPantryKnownRecipesAndCapacity(t *testing.T) {
	w, p := businessWorld(t)
	mustAct(t, w, Action{Action: "bakery_hire", Item: "chef"})
	nodePosition(w, p, "bakery_plot_0_oven")
	if err := w.Act("one", Action{Action: "bakery_chef_recipe", Item: "berry_cookie"}); err == nil {
		t.Fatal("chef taught an undiscovered recipe")
	}
	p.profile.Discovered["berry_cookie"] = true
	mustAct(t, w, Action{Action: "bakery_chef_recipe", Item: "berry_cookie"})
	setItem(p, "berry", 10)
	setItem(p, "dough", 10)
	advance(w, 13)
	if len(p.profile.BakeryDisplay) != 0 || p.profile.Inventory["berry"] != 10 {
		t.Fatal("chef used owner's bag without pantry deposit")
	}
	nodePosition(w, p, "bakery_plot_0_fridge")
	mustAct(t, w, Action{Action: "bakery_pantry_deposit", Item: "berry", Amount: 2})
	mustAct(t, w, Action{Action: "bakery_pantry_deposit", Item: "dough", Amount: 1})
	advance(w, 20)
	if p.profile.Production["bakery_cooling_table"].Output["berry_cookie"] != 1 || p.profile.BakeryDisplay["berry_cookie"] != 0 || p.profile.BakeryPantry["berry"] != 0 || p.profile.BakeryPantry["dough"] != 0 || p.profile.Inventory["berry"] != 8 || p.profile.Inventory["dough"] != 9 {
		t.Fatal("chef did not use exact pantry recipe")
	}
	p.profile.BakeryDisplay = map[string]int{"sugar": bakeryDisplayCapacity}
	p.profile.Production["bakery_cooling_table"].Output = map[string]int{"sugar": productionCapacity}
	mustAct(t, w, Action{Action: "bakery_pantry_deposit", Item: "berry", Amount: 2})
	mustAct(t, w, Action{Action: "bakery_pantry_deposit", Item: "dough", Amount: 1})
	beforePantry := cloneMap(p.profile.BakeryPantry)
	p.bakery.nextCustomer = w.time + 100
	advance(w, 12)
	if !reflect.DeepEqual(beforePantry, p.profile.BakeryPantry) || bakeryStockCount(p.profile.BakeryDisplay) != bakeryDisplayCapacity {
		t.Fatal("full display lost ingredients or overflowed")
	}
}
func TestBakeryBusinessTransfersPermissionsAndAtomicFailure(t *testing.T) {
	w, p := businessWorld(t)
	nodePosition(w, p, "bakery_plot_0_display")
	if err := w.Act("one", Action{Action: "bakery_stock", Item: "stone"}); err == nil {
		t.Fatal("display accepted non-cookie")
	}
	p.profile.BakeryDisplay["sugar"] = bakeryDisplayCapacity - 1
	before := cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "bakery_stock", Item: "sugar", Amount: 2}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("overfull stocking was not atomic")
	}
	w.Join("two", "Visitor")
	other := w.players["two"]
	nodePosition(w, other, "bakery_plot_0_display")
	for _, a := range []Action{{Action: "bakery_stock", Item: "sugar"}, {Action: "bakery_unstock", Item: "sugar"}, {Action: "bakery_hire", Item: "chef"}} {
		if err := w.Act("two", a); err == nil {
			t.Fatal("visitor controlled bakery")
		}
	}
	nodePosition(w, p, "bakery_plot_0_oven")
	if err := w.Act("one", Action{Action: "bakery_stock", Item: "sugar"}); err == nil {
		t.Fatal("stocking from wrong station succeeded")
	}
	p.inCave = true
	nodePosition(w, p, "bakery_plot_0_display")
	if err := w.Act("one", Action{Action: "bakery_unstock", Item: "sugar"}); err == nil {
		t.Fatal("underground coordinates reached display")
	}
	p.inCave = false
	for i := range p.profile.BagSlots {
		p.profile.BagSlots[i] = Stack{Item: "stone", Count: inventoryLimit}
	}
	for i := range p.profile.Hotbar {
		p.profile.Hotbar[i] = Stack{Item: "stone", Count: inventoryLimit}
	}
	syncInventory(&p.profile)
	before = cloneProfile(p.profile)
	if err := w.Act("one", Action{Action: "bakery_unstock", Item: "sugar", Amount: 20}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("failed withdrawal lost display goods")
	}
}
func TestBakeryBusinessPersistenceDetachmentAndDisconnectPause(t *testing.T) {
	w, p := businessWorld(t)
	p.profile.BakeryDisplay["sugar"] = 7
	p.profile.BakeryPantry["berry"] = 4
	p.profile.BakeryStaff["chef"] = true
	p.profile.Discovered["berry_cookie"] = true
	p.profile.BakeryChefRecipe = "berry_cookie"
	p.profile.BakeryChefRecipes = nil // A legacy save predates multiple selections.
	saved := cloneProfile(p.profile)
	snapshot := w.Snapshot("one")
	snapshot.Me.BakeryDisplay["sugar"] = 100
	snapshot.Me.BakeryPantry["berry"] = 100
	snapshot.Me.BakeryStaff["chef"] = false
	for _, n := range snapshot.Nodes {
		if n.Kind == "bakery_display" && n.Owner == "one" {
			n.Stock["sugar"] = 99
		}
	}
	if p.profile.BakeryDisplay["sugar"] != 7 || p.profile.BakeryPantry["berry"] != 4 || !p.profile.BakeryStaff["chef"] {
		t.Fatal("snapshot bakery maps alias live state")
	}
	w.ReleaseBakeryPlot("one")
	advance(w, 80)
	if p.bakery != nil || p.profile.BakeryDisplay["sugar"] != 7 || p.profile.BakeryPantry["berry"] != 4 {
		t.Fatal("closed bakery continued operating")
	}
	w.Leave("one")
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	p = restored.players["one"]
	if !reflect.DeepEqual(p.profile.BakeryDisplay, saved.BakeryDisplay) || !reflect.DeepEqual(p.profile.BakeryPantry, saved.BakeryPantry) || !reflect.DeepEqual(p.profile.BakeryStaff, saved.BakeryStaff) || p.profile.BakeryChefRecipe != "berry_cookie" {
		t.Fatal("saved bakery business not restored")
	}
	nodePosition(restored, p, "bakery_plot_1")
	mustAct(t, restored, Action{Action: "buy_land", Target: "bakery_plot_1"})
	if p.profile.Coins != saved.Coins || p.profile.BakeryDisplay["sugar"] != 7 {
		t.Fatal("free reopening lost stock or charged coins")
	}
}
func TestBakeryCustomerSaleAtWalletCapIsAtomic(t *testing.T) {
	w, p := businessWorld(t)
	p.profile.BakeryDisplay["sugar"] = 5
	c := firstWaitingCustomer(t, w, p)
	nodePosition(w, p, "bakery_plot_0_display")
	p.profile.Coins = coinLimit - 1
	if err := w.Act("one", Action{Action: "bakery_serve", Target: c.ID}); err == nil || p.profile.BakeryDisplay["sugar"] != 5 || p.profile.Coins != coinLimit-1 || c.Phase != "waiting" {
		t.Fatal("wallet cap silently clipped sale or lost stock")
	}
}
