package game

import (
	"errors"
	"fmt"
	"math"
)

const bakeryDisplayCapacity = 100
const bakeryPantryCapacity = 1000
const bakeryCustomerInterval = 20.0
const bakeryChefInterval = .5

type BakeryCustomerView struct {
	ID     string `json:"id"`
	Item   string `json:"item"`
	Amount int    `json:"amount"`
	Price  int    `json:"price"`
	Phase  string `json:"phase"`
}
type BakeryBusinessView struct {
	Waiting         int                  `json:"waiting"`
	NextCustomerIn  float64              `json:"nextCustomerIn"`
	ChefStatus      string               `json:"chefStatus"`
	ChefReadyIn     float64              `json:"chefReadyIn"`
	DisplayCapacity int                  `json:"displayCapacity"`
	PantryCapacity  int                  `json:"pantryCapacity"`
	Customers       []BakeryCustomerView `json:"customers"`
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
	customers                             []*bakeryCustomer
	workers                               map[string]*bakeryWorker
	routesDirty                           bool
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
	for item, n := range p.BakeryDisplay {
		if _, ok := recipeFor(item); !ok || n <= 0 {
			delete(p.BakeryDisplay, item)
		} else {
			p.BakeryDisplay[item] = min(n, bakeryDisplayCapacity)
		}
	}
	// Stable recipe order makes malformed over-capacity saves deterministic.
	remaining := bakeryDisplayCapacity
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
	for role, owned := range p.BakeryStaff {
		if !owned || (role != "chef" && role != "cashier") {
			delete(p.BakeryStaff, role)
		}
	}
	if _, ok := recipeFor(p.BakeryChefRecipe); !ok || p.BakeryChefRecipe != "sugar" && !p.Discovered[p.BakeryChefRecipe] {
		p.BakeryChefRecipe = "sugar"
	}
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
		price := map[string]int{"chef": 300, "cashier": 200}[a.Item]
		if price == 0 {
			return errors.New("Choose a chef or cashier.")
		}
		if p.profile.BakeryStaff[a.Item] {
			return errors.New("You have already hired this member of staff.")
		}
		if p.profile.Coins < price {
			return fmt.Errorf("Hiring a %s costs %d coins.", a.Item, price)
		}
		p.profile.Coins -= price
		p.profile.BakeryStaff[a.Item] = true
		if a.Item == "chef" && p.bakery != nil {
			p.bakery.chefStatus = "Starting shift"
		}
		return nil
	case "bakery_chef_recipe":
		if err := w.requireBakeryStation(p, "oven", a.Target); err != nil {
			return err
		}
		if _, ok := recipeFor(a.Item); !ok || a.Item != "sugar" && !p.profile.Discovered[a.Item] {
			return errors.New("Bake this recipe yourself to teach it to your chef first.")
		}
		p.profile.BakeryChefRecipe = a.Item
		if p.profile.BakeryStaff["chef"] && p.bakery != nil {
			p.bakery.chefStatus = "Preparing recipe"
		}
		return nil
	case "bakery_stock", "bakery_unstock":
		if err := w.requireBakeryStation(p, "display", a.Target); err != nil {
			return err
		}
		if _, ok := recipeFor(a.Item); !ok {
			return errors.New("Stock cookies and cakes in the display.")
		}
		return transferBakeryStock(p, p.profile.BakeryDisplay, a.Item, a.Amount, a.Action == "bakery_stock", bakeryDisplayCapacity)
	case "bakery_pantry_deposit", "bakery_pantry_withdraw":
		if err := w.requireBakeryStation(p, "mixer", a.Target); err != nil {
			return err
		}
		if !bakeryPantryItem(a.Item) {
			return errors.New("The bakery pantry holds dough and baking ingredients.")
		}
		return transferBakeryStock(p, p.profile.BakeryPantry, a.Item, a.Amount, a.Action == "bakery_pantry_deposit", bakeryPantryCapacity)
	case "bakery_serve":
		if err := w.requireBakeryStation(p, "display", ""); err != nil {
			return err
		}
		if p.bakery != nil {
			for _, customer := range p.bakery.customers {
				if customer.ID == a.Target {
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
	if _, _, ok := w.bakeryStationPosition(p, "display"); !ok || p.bakery != nil && p.bakery.routesDirty {
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
	p.profile.Coins += price
	w.leaveBakeryCustomer(p, c, true)
	recipe, _ := recipeFor(c.Item)
	w.eventActor(fmt.Sprintf("A customer bought %d %s for %d coins.", c.Amount, recipe.Name, price), "craft", c.x, c.z, p.profile.ID)
	return nil
}

func (w *World) tickBakeries(dt float64) {
	for _, p := range w.players {
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
					center, _, _ := w.bakeryStationPosition(p, "display")
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
			_, _, displayPlaced := w.bakeryStationPosition(p, "display")
			if displayPlaced && len(b.customers) < 3 && bakeryStockCount(p.profile.BakeryDisplay) > 0 {
				choices := []string{}
				for _, r := range recipes {
					if p.profile.BakeryDisplay[r.ID] > 0 {
						choices = append(choices, r.ID)
					}
				}
				w.sequence++
				item := choices[int(w.sequence%uint64(len(choices)))]
				used := [3]bool{}
				for _, other := range b.customers {
					used[other.slot] = true
				}
				slot := 0
				for slot < len(used)-1 && used[slot] {
					slot++
				}
				c := &bakeryCustomer{BakeryCustomerView: BakeryCustomerView{ID: fmt.Sprintf("customer_%d", w.sequence), Item: item, Amount: min(5, p.profile.BakeryDisplay[item]), Phase: "arriving"}, x: plot.X, z: plot.Z + 9.5}
				c.slot = slot
				w.routeBakeryCustomer(p, c)
				if c.hasRoute && c.Phase == "arriving" {
					b.customers = append(b.customers, c)
				}
			}
		}
		if p.profile.BakeryStaff["cashier"] && w.bakeryWorkerReady(p, "cashier") && w.time >= b.cashierReady {
			b.cashierReady = w.time + 3
			for _, c := range b.customers {
				if c.Phase == "waiting" && p.profile.BakeryDisplay[c.Item] >= c.Amount {
					if w.serveBakeryCustomer(p, c) == nil {
						break
					}
				}
			}
		}
	}
}

func (w *World) workBakeryChef(p *player) {
	b := p.bakery
	if !p.profile.BakeryStaff["chef"] {
		b.chefStatus = "Not hired"
		return
	}
	if reason := w.bakeryChefStationStatus(p); reason != "" {
		b.chefStatus = reason
		return
	}
	if !w.bakeryWorkerReady(p, "chef") {
		b.chefStatus = "Walking to the oven"
		return
	}
	mixer := productionState(&p.profile, "bakery_mixer")
	oven := productionState(&p.profile, "bakery_oven")
	cooling := productionState(&p.profile, "bakery_cooling_table")
	for _, r := range recipes {
		n := min(cooling.Output[r.ID], bakeryDisplayCapacity-bakeryStockCount(p.profile.BakeryDisplay))
		if n > 0 {
			p.profile.BakeryDisplay[r.ID] += n
			cooling.Output[r.ID] -= n
			if cooling.Output[r.ID] == 0 {
				delete(cooling.Output, r.ID)
			}
		}
	}
	for item, n := range mixer.Output {
		n = min(n, bakeryPantryCapacity-bakeryStockCount(p.profile.BakeryPantry))
		if n > 0 {
			p.profile.BakeryPantry[item] += n
			mixer.Output[item] -= n
			if mixer.Output[item] == 0 {
				delete(mixer.Output, item)
			}
		}
	}
	if oven.Job != nil {
		b.chefStatus = "Baking"
		return
	}
	if bakeryStockCount(p.profile.BakeryDisplay) >= bakeryDisplayCapacity {
		b.chefStatus = "Display full"
		return
	}
	r, ok := recipeFor(p.profile.BakeryChefRecipe)
	if !ok {
		b.chefStatus = "Choose a recipe"
		return
	}
	input := "prepared_" + r.ID
	if r.ID == "sugar" {
		input = "dough"
	}
	if p.profile.BakeryPantry[input] > 0 && bakeryStockCount(cooling.Output) < productionCapacity {
		p.profile.BakeryPantry[input]--
		if p.profile.BakeryPantry[input] == 0 {
			delete(p.profile.BakeryPantry, input)
		}
		startProduction(oven, r.ID, 1, 5)
		oven.Job.Destination = "bakery_cooling_table"
		b.chefStatus = "Baking " + r.Name
		return
	}
	if mixer.Job != nil {
		b.chefStatus = "Mixing dough"
		return
	}
	if !emptyProductionMixer(mixer) {
		b.chefStatus = "Pantry full"
		return
	}
	cost := productionAddinCost(r, 1, p.profile.BakeryPantry)
	plain := p.profile.BakeryPantry["dough"] < cost["dough"]
	if plain {
		cost = map[string]int{"baking_sugar": 1, "butter": 1, "flour": 1, "egg": 1}
	}
	for item, n := range cost {
		if p.profile.BakeryPantry[item] < n {
			b.chefStatus = "Needs ingredients"
			return
		}
	}
	for item, n := range cost {
		p.profile.BakeryPantry[item] -= n
		if p.profile.BakeryPantry[item] == 0 {
			delete(p.profile.BakeryPantry, item)
		}
	}
	if plain {
		startProduction(mixer, "dough", 40, 40)
	} else {
		startProduction(mixer, "prepared_"+r.ID, 1, float64(cost["dough"]))
	}
	b.chefStatus = "Mixing dough"
}

func (w *World) bakeryBusinessSnapshot(viewer *player, out *Snapshot) {
	if viewer != nil && viewer.profile.Bakery {
		v := &BakeryBusinessView{DisplayCapacity: bakeryDisplayCapacity, PantryCapacity: bakeryPantryCapacity, Customers: []BakeryCustomerView{}, ChefStatus: "Bakery closed"}
		if b := viewer.bakery; b != nil {
			v.NextCustomerIn = max(0, b.nextCustomer-w.time)
			v.ChefReadyIn = max(0, b.chefReady-w.time)
			v.ChefStatus = b.chefStatus
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
		for _, role := range []string{"chef", "cashier"} {
			if !p.profile.BakeryStaff[role] {
				continue
			}
			x, z, yaw := plot.X, plot.Z+9.5, 0.0
			phase := "Starting shift"
			if p.bakery != nil && p.bakery.workers[role] != nil {
				worker := p.bakery.workers[role]
				x, z, yaw, phase = worker.x, worker.z, worker.yaw, worker.status
			}
			if role == "chef" && p.bakery != nil {
				phase = p.bakery.chefStatus
			}
			out.Nodes = append(out.Nodes, Node{ID: plot.ID + "_" + role, Kind: "bakery_" + role, Owner: p.profile.ID, PlotID: plot.ID, Label: "Bakery " + role, X: x, Z: z, Yaw: yaw, Phase: phase})
		}
		if p.bakery == nil {
			continue
		}
		for _, c := range p.bakery.customers {
			out.Nodes = append(out.Nodes, Node{ID: c.ID, Kind: "bakery_customer", Owner: p.profile.ID, PlotID: plot.ID, Label: "Bakery customer", Item: c.Item, Amount: c.Amount, X: c.x, Z: c.z, Yaw: c.yaw, Phase: c.Phase})
		}
	}
}
