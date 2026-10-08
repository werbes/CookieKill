package game

import (
	"errors"
	"fmt"
	"math"
)

const bakeryDisplayCapacity = 300
const bakeryPantryCapacity = 1000
const bakeryCustomerInterval = 20.0
const bakeryChefInterval = .5

type BakeryCustomerView struct {
	ID        string `json:"id"`
	Item      string `json:"item"`
	Amount    int    `json:"amount"`
	Price     int    `json:"price"`
	Phase     string `json:"phase"`
	DisplayID string `json:"displayId"`
}
type BakeryBusinessView struct {
	Waiting               int                  `json:"waiting"`
	NextCustomerIn        float64              `json:"nextCustomerIn"`
	ChefStatus            string               `json:"chefStatus"`
	ChefReadyIn           float64              `json:"chefReadyIn"`
	ServerStatus          string               `json:"serverStatus"`
	AchievementCount      int                  `json:"achievementCount"`
	AchievementMultiplier float64              `json:"achievementMultiplier"`
	AchievementFormula    string               `json:"achievementFormula"`
	DisplayCapacity       int                  `json:"displayCapacity"`
	PantryCapacity        int                  `json:"pantryCapacity"`
	Customers             []BakeryCustomerView `json:"customers"`
	Workers               []BakeryWorkerView   `json:"workers"`
	StaffLimit            int                  `json:"staffLimit"`
}
type bakeryCustomer struct {
	BakeryCustomerView
	x, z, yaw, expires float64
	path               []Point
	slot               int
	hasRoute           bool
	goal               Point
}
type bakeryBusiness struct {
	plot                                  string
	nextCustomer, chefReady, cashierReady float64
	chefStatus                            string
	serverStatus                          string
	fridgeAccessChecked, fridgeAccessible bool
	customers                             []*bakeryCustomer
	workers                               map[string]*bakeryWorker
	routesDirty                           bool
	chefRecipeCursor                      int
	customerDisplayCursor                 int
}

func bakeryPantryItem(item string) bool {
	if validChoice(item, "baking_sugar", "butter", "flour", "egg") {
		return true
	}
	if _, ok := preparedRecipe(item); ok {
		return true
	}
	for _, r := range recipes {
		if r.Cost[item] > 0 {
			return true
		}
	}
	return item == "protein_drink"
}
func bakeryStockCount(stock map[string]int) int {
	n := 0
	for _, count := range stock {
		n += count
	}
	return n
}
func normalizeBakeryBusiness(p *Profile) {
	if p.BakeryDisplay == nil {
		p.BakeryDisplay = map[string]int{}
	}
	if p.BakeryPantry == nil {
		p.BakeryPantry = map[string]int{}
	}
	if p.BakeryStaff == nil {
		p.BakeryStaff = map[string]bool{}
	}
	normalizeBakeryWorkforce(p)
	for item, n := range p.BakeryDisplay {
		if _, ok := recipeFor(item); !ok || n <= 0 {
			delete(p.BakeryDisplay, item)
		} else {
			p.BakeryDisplay[item] = min(n, bakeryDisplayCapacity*2)
		}
	}
	// Stable recipe order makes malformed over-capacity saves deterministic.
	// Packed furniture must not destroy stock when the profile is reloaded.
	remaining := bakeryDisplayCapacity * 2
	for _, r := range recipes {
		n := min(remaining, p.BakeryDisplay[r.ID])
		if n > 0 {
			p.BakeryDisplay[r.ID] = n
			remaining -= n
		} else {
			delete(p.BakeryDisplay, r.ID)
		}
	}
	for item, n := range p.BakeryPantry {
		if !bakeryPantryItem(item) || n <= 0 {
			delete(p.BakeryPantry, item)
		} else {
			p.BakeryPantry[item] = min(n, bakeryPantryCapacity)
		}
	}
	remaining = bakeryPantryCapacity
	pantryItems := []string{"baking_sugar", "butter", "flour", "egg", "dough", "berry", "nut", "cactus", "sea_salt", "protein_powder", "protein_drink"}
	for _, r := range recipes {
		pantryItems = append(pantryItems, "prepared_"+r.ID)
	}
	for _, item := range pantryItems {
		n := min(remaining, p.BakeryPantry[item])
		if n > 0 {
			p.BakeryPantry[item] = n
			remaining -= n
		} else {
			delete(p.BakeryPantry, item)
		}
	}
	normalizeBakeryChefRecipes(p)
}

func (w *World) startBakeryBusiness(p *player) {
	p.bakery = &bakeryBusiness{plot: p.profile.BakeryPlot, nextCustomer: w.time + bakeryCustomerInterval, chefReady: w.time + bakeryChefInterval, cashierReady: w.time + 3, chefStatus: "Not hired", workers: map[string]*bakeryWorker{}, routesDirty: true}
	if p.profile.BakeryStaff["chef"] {
		p.bakery.chefStatus = "Starting shift"
	}
}
func (w *World) bakeryPlot(p *player) (BakeryPlot, bool) {
	if !p.profile.Bakery || p.profile.BakeryPlot == "" {
		return BakeryPlot{}, false
	}
	for _, plot := range w.layout.Plots {
		if plot.ID == p.profile.BakeryPlot {
			return plot, true
		}
	}
	return BakeryPlot{}, false
}

func (w *World) bakeryBusinessAction(p *player, a Action) error {
	switch a.Action {
	case "bakery_hire":
		if err := w.requireBakery(p); err != nil {
			return err
		}
		if a.Target != "" && a.Target != p.profile.BakeryPlot {
			return errors.New("Hire staff at your own bakery sign.")
		}
		price := map[string]int{"chef": 300, "cashier": 200, "server": 250}[a.Item]
		if price == 0 {
			return errors.New("Choose a chef, server, or cashier.")
		}
		workerID := a.Item
		if p.profile.BakeryStaff[workerID] {
			workerID += "_2"
		}
		if p.profile.BakeryStaff[workerID] {
			return errors.New("You can hire at most two of each staff role.")
		}
		if p.profile.Coins < price {
			return fmt.Errorf("Hiring a %s costs %d coins.", a.Item, price)
		}
		p.profile.Coins -= price
		p.profile.BakeryStaff[workerID] = true
		if a.Item == "chef" && p.bakery != nil {
			p.bakery.chefStatus = "Starting shift"
		}
		return nil
	case "bakery_staff_break":
		if err := w.requireBakery(p); err != nil {
			return err
		}
		if a.Target != p.profile.BakeryPlot || bakeryStaffRole(a.Item) == "" || !p.profile.BakeryStaff[a.Item] {
			return errors.New("Manage a hired worker at your own bakery sign.")
		}
		if a.Enabled {
			p.profile.BakeryStaffBreaks[a.Item] = true
		} else {
			delete(p.profile.BakeryStaffBreaks, a.Item)
		}
		if p.bakery != nil {
			if worker := p.bakery.workers[a.Item]; worker != nil {
				if a.Enabled {
					worker.status = "On break"
				} else {
					worker.status = "Returning to work"
					worker.hasRoute = false
				}
			}
		}
		return nil
	case "bakery_chef_recipe":
		return w.bakeryChefRecipeAction(p, a)
	case "bakery_chef_recipe_set":
		return w.bakeryChefRecipeAction(p, a)
	case "bakery_stock", "bakery_unstock":
		if err := w.requireBakeryStation(p, "display", a.Target); err != nil {
			return err
		}
		if _, ok := recipeFor(a.Item); !ok {
			return errors.New("Stock cookies and cakes in the display.")
		}
		return transferBakeryStock(p, p.profile.BakeryDisplay, a.Item, a.Amount, a.Action == "bakery_stock", bakeryTotalDisplayCapacity(&p.profile))
	case "bakery_pantry_deposit", "bakery_pantry_withdraw", "bakery_fridge_deposit", "bakery_fridge_withdraw":
		if err := w.requireBakeryStation(p, "fridge", a.Target); err != nil {
			return err
		}
		if !bakeryPantryItem(a.Item) {
			return errors.New("The kitchen fridge holds dough and baking ingredients.")
		}
		return transferBakeryStock(p, p.profile.BakeryPantry, a.Item, a.Amount, a.Action == "bakery_pantry_deposit" || a.Action == "bakery_fridge_deposit", bakeryPantryCapacity)
	case "bakery_serve":
		if p.bakery != nil {
			for _, customer := range p.bakery.customers {
				if customer.ID == a.Target {
					if err := w.requireBakeryCustomer(p, customer); err != nil {
						return err
					}
					return w.serveBakeryCustomer(p, customer)
				}
			}
		}
		return errors.New("That customer is no longer waiting at your bakery.")
	}
	return errors.New("Unknown bakery business action.")
}
func transferBakeryStock(p *player, stock map[string]int, item string, amount int, deposit bool, capacity int) error {
	if deposit {
		if amount > capacity-bakeryStockCount(stock) {
			return errors.New("This bakery storage is full. Make room first.")
		}
		if !take(p, item, amount) {
			return errors.New("You do not have enough of this item.")
		}
		stock[item] += amount
	} else {
		if stock[item] < amount {
			return errors.New("There is not enough of that item in bakery storage.")
		}
		if !gain(p, item, amount) {
			return errors.New("Make room in your inventory first.")
		}
		stock[item] -= amount
		if stock[item] == 0 {
			delete(stock, item)
		}
	}
	return nil
}
func bakerySalePrice(p *player, item string, amount int) int {
	price := 2
	if item != "sugar" {
		price = 4
	}
	if item == "cake" {
		price = 12
	}
	total := price*amount + int(float64(price*amount)*.5*bakeryBoost(p))
	if p.profile.Equipment["display"] {
		total += amount
	}
	total += achievementPriceBonus(&p.profile, price*amount)
	return total
}
func (w *World) leaveBakeryCustomer(p *player, c *bakeryCustomer, bought bool) {
	c.Phase = "leaving"
	if !bought {
		c.Amount = 0
	}
	w.routeBakeryCustomer(p, c)
}
func (w *World) serveBakeryCustomer(p *player, c *bakeryCustomer) error {
	if c.DisplayID == "" {
		if displays := bakeryDisplays(&p.profile); len(displays) > 0 {
			c.DisplayID = displays[0].ID
		}
	}
	display, ok := bakeryFurnitureByID(&p.profile, c.DisplayID)
	if !ok || display.Kind != "display" || display.ID == p.editingBakeryFurnitureID || p.bakery != nil && p.bakery.routesDirty {
		return errors.New("Place the display and let customers reach its counter before serving.")
	}
	if c.Phase != "waiting" {
		return errors.New("This customer is not waiting to be served.")
	}
	if p.profile.BakeryDisplay[c.Item] < c.Amount {
		return errors.New("Stock enough of the customer's order in the display first.")
	}
	price := bakerySalePrice(p, c.Item, c.Amount)
	if price > coinLimit-p.profile.Coins {
		return errors.New("Your wallet is full. Spend some coins before making more sales.")
	}
	p.profile.BakeryDisplay[c.Item] -= c.Amount
	if p.profile.BakeryDisplay[c.Item] == 0 {
		delete(p.profile.BakeryDisplay, c.Item)
	}
	addCoins(p, price)
	w.leaveBakeryCustomer(p, c, true)
	recipe, _ := recipeFor(c.Item)
	w.eventActor(fmt.Sprintf("A customer bought %d %s for %d coins.", c.Amount, recipe.Name, price), "craft", c.x, c.z, p.profile.ID)
	return nil
}

func (w *World) tickBakeries(dt float64) {
	for _, p := range w.players {
		if p.disconnected {
			continue
		}
		plot, ok := w.bakeryPlot(p)
		if !ok {
			p.bakery = nil
			continue
		}
		if p.bakery == nil || p.bakery.plot != plot.ID {
			w.startBakeryBusiness(p)
		}
		b := p.bakery
		w.updateBakeryWorkers(p, dt)
		w.workBakeryServer(p, dt)
		if b.routesDirty {
			for _, c := range b.customers {
				w.routeBakeryCustomer(p, c)
			}
			b.routesDirty = false
		}
		if w.time >= b.chefReady {
			b.chefReady = w.time + bakeryChefInterval
			w.workBakeryChef(p)
		}
		kept := b.customers[:0]
		for _, c := range b.customers {
			advanceBakeryPath(&c.x, &c.z, &c.yaw, &c.path, dt*1.8)
			if c.hasRoute && len(c.path) == 0 {
				if c.Phase == "leaving" {
					continue
				}
				if c.Phase == "arriving" {
					c.Phase = "waiting"
					display, _ := bakeryFurnitureByID(&p.profile, c.DisplayID)
					center := Point{plot.X + display.X, plot.Z + display.Z}
					c.yaw = math.Atan2(-(center.X - c.x), -(center.Z - c.z))
					if c.expires == 0 {
						c.expires = w.time + 60
					}
				}
			}
			if c.Phase == "waiting" && w.time >= c.expires {
				w.leaveBakeryCustomer(p, c, false)
			}
			kept = append(kept, c)
		}
		b.customers = kept
		if w.time >= b.nextCustomer {
			b.nextCustomer = w.time + bakeryCustomerInterval
			w.admitBakeryCustomer(p)
		}
		w.workBakeryCashiers(p)
	}
}
func (w *World) bakeryBusinessSnapshot(viewer *player, out *Snapshot) {
	if viewer != nil && viewer.profile.Bakery {
		count := hungAchievementCount(&viewer.profile)
		v := &BakeryBusinessView{DisplayCapacity: bakeryTotalDisplayCapacity(&viewer.profile), PantryCapacity: bakeryPantryCapacity, Customers: []BakeryCustomerView{}, Workers: bakeryWorkerViews(viewer), StaffLimit: bakeryStaffLimit, ChefStatus: "Bakery closed", ServerStatus: "Bakery closed", AchievementCount: count, AchievementMultiplier: 1 + .1*float64(count), AchievementFormula: "1 + 0.10 × N"}
		if b := viewer.bakery; b != nil {
			v.NextCustomerIn = max(0, b.nextCustomer-w.time)
			v.ChefReadyIn = max(0, b.chefReady-w.time)
			v.ChefStatus = b.chefStatus
			v.ServerStatus = b.serverStatus
			for _, c := range b.customers {
				customer := c.BakeryCustomerView
				customer.Price = bakerySalePrice(viewer, c.Item, c.Amount)
				v.Customers = append(v.Customers, customer)
				if c.Phase == "waiting" {
					v.Waiting++
				}
			}
		}
		out.Me.BakeryBusiness = v
	}
	if viewer != nil && viewer.island != "" {
		return
	}
	for _, p := range w.players {
		plot, ok := w.bakeryPlot(p)
		if !ok {
			continue
		}
		for _, view := range bakeryWorkerViews(p) {
			role, id := view.Role, view.ID
			x, z, yaw := plot.X, plot.Z+9.5, 0.0
			if p.bakery != nil && p.bakery.workers[id] != nil {
				worker := p.bakery.workers[id]
				x, z, yaw = worker.x, worker.z, worker.yaw
			}
			node := Node{ID: plot.ID + "_" + id, Kind: "bakery_" + role, Owner: p.profile.ID, PlotID: plot.ID, Label: view.Label, X: x, Z: z, Yaw: yaw, Phase: view.Status}
			if role == "server" {
				tray := bakeryServerTray(&p.profile, id)
				for _, r := range recipes {
					if n := tray[r.ID]; n > 0 {
						node.Item, node.Amount = r.ID, n
						break
					}
				}
			}
			out.Nodes = append(out.Nodes, node)
		}
		if p.bakery == nil {
			continue
		}
		for _, c := range p.bakery.customers {
			out.Nodes = append(out.Nodes, Node{ID: c.ID, Kind: "bakery_customer", Owner: p.profile.ID, PlotID: plot.ID, Label: "Bakery customer", Item: c.Item, Amount: c.Amount, X: c.x, Z: c.z, Yaw: c.yaw, Phase: c.Phase})
		}
	}
}
