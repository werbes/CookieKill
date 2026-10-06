package game

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Act validates intent, inventory, proximity and cooldowns on the server.
func (w *World) Act(id string, a Action) (err error) {
	p, ok := w.players[id]
	if !ok {
		return errors.New("Join the game before taking an action.")
	}
	before := cloneProfile(p.profile)
	defer func() {
		if err != nil {
			p.profile = before
		}
	}()
	if a.Action == "move_item" {
		return moveItem(p, a)
	}
	if a.Amount == 0 {
		a.Amount = 1
	}
	if a.Amount < 1 || a.Amount > 20 {
		return errors.New("Choose an amount between 1 and 20.")
	}
	switch a.Action {
	case "friend_public", "friend_request", "friend_accept", "friend_decline", "friend_remove":
		return w.socialAction(p, a)
	case "eat_lollipop", "buy_home", "teleport_home", "teleport_main", "teleport_cave", "visit_island", "island_visitors", "island_builder", "chest_deposit", "chest_withdraw", "build", "move_build", "destroy_build", "use_build":
		return w.islandAction(p, a)
	case "adopt_animal":
		return w.adoptAnimal(p, a.Target)
	case "camel_rent", "camel_buy", "camel_gift", "camel_ride":
		return w.camelAction(p, a)
	case "throw":
		return w.throw(p, a.Item)
	case "eat":
		return w.eat(p, a.Item)
	case "select":
		if a.Slot == 5 && p.profile.HomeIsland != nil {
			p.profile.SelectedSlot = 5
			syncInventory(&p.profile)
			return nil
		}
		if a.Slot < 0 || a.Slot >= 5 {
			return errors.New("Choose one of your five hotbar slots.")
		}
		p.profile.SelectedSlot = a.Slot
		syncInventory(&p.profile)
		return nil
	case "claim_recovery":
		return claimRecovery(p, a.Item)
	case "customize":
		return w.customize(p, a)
	case "build_home":
		return w.buildHome(p, a.Item)
	case "create_safezone":
		return w.createSafeZone(p)
	case "paint":
		return w.paintBakery(p, a.Target, a.Item)
	case "interact":
		return w.interact(p, a.Target)
	case "craft":
		return w.craft(p, a.Item, a.Amount)
	case "trade":
		return w.trade(p, a.Target, a.Item, a.Amount)
	case "train":
		return w.train(p, a.Item)
	case "buy_land":
		return w.buyLand(p, a.Target)
	case "upgrade":
		return w.upgradeBakery(p)
	case "make_dough":
		if err := w.requireBakery(p); err != nil {
			return err
		}
		if p.cooldowns["dough"] > w.time {
			return errors.New("Your dough mixer is still working.")
		}
		if !gain(p, "dough", 2+int(4*bakeryBoost(p))) {
			return errors.New("Your inventory is full. Move or use a stack first.")
		}
		rest := 8 * (1 - .4*bakeryBoost(p))
		if p.profile.Equipment["mixer"] {
			rest *= .85
		}
		p.cooldowns["dough"] = w.time + rest
		w.event("Fresh dough is ready.", "craft", p.x, p.z)
		return nil
	case "order_dough":
		if err := w.requireBakery(p); err != nil {
			return err
		}
		cost := 5 * a.Amount
		if p.profile.Coins < cost {
			return fmt.Errorf("Dough delivery costs %d coins.", cost)
		}
		p.profile.Coins -= cost
		if !gain(p, "dough", 8*a.Amount) {
			return errors.New("Your inventory is full. Move or use a stack first.")
		}
		w.event("A dough delivery arrived.", "craft", p.x, p.z)
		return nil
	case "consume_protein":
		if a.Item != "protein_powder" && a.Item != "protein_drink" {
			return errors.New("Choose protein powder or a protein drink.")
		}
		if p.profile.Inventory[a.Item] < 1 {
			return errors.New("You do not have that protein item.")
		}
		take(p, a.Item, 1)
		duration := 90.0
		if a.Item == "protein_powder" {
			duration = 120
		}
		p.buffs["protein"] = w.time + duration
		w.event("Protein boost: gym training recovers faster.", "buff", p.x, p.z)
		return nil
	default:
		return errors.New("Unknown action.")
	}
}

func addCoins(p *player, count int) { p.profile.Coins = min(inventoryLimit, p.profile.Coins+count) }

func (w *World) near(p *player, id string, radius float64) bool {
	for _, n := range w.nodes {
		if n.ID == id && n.Island == p.island {
			return n.Available && distance(p.x, p.z, n.X, n.Z) <= radius
		}
	}
	return false
}

func (w *World) requireBakery(p *player) error {
	if !p.profile.Bakery {
		return errors.New("Buy your bakery land in the city first.")
	}
	if !w.near(p, p.profile.BakeryPlot, 10) {
		return errors.New("Visit your city bakery to do that.")
	}
	return nil
}

func (w *World) throw(p *player, item string) error {
	if p.island != "" {
		return errors.New("Home islands are peaceful. Return to the main island to throw cookies.")
	}
	if w.insideSafe(p.x, p.z) {
		return errors.New("Leave the safe zone before throwing cookies.")
	}
	if item == "" {
		item = p.profile.Selected
	}
	r, ok := recipeFor(item)
	if !ok {
		return errors.New("Choose a cookie to throw.")
	}
	if p.cooldowns["throw"] > w.time {
		return errors.New("Your next throw is not ready yet.")
	}
	if !hasHotbar(p, item) {
		return errors.New("Move that cookie into your hotbar before throwing it.")
	}
	arms := float64(p.profile.Levels["arms"])
	speed := 22 + arms*.8
	damage := r.Damage * (1 + arms*.045)
	if w.active(p, "range") {
		speed *= 1.5
	}
	if w.active(p, "weakened") {
		speed *= .65
		damage *= .65
	}
	dx := -math.Sin(p.yaw) * math.Cos(p.pitch)
	dz := -math.Cos(p.yaw) * math.Cos(p.pitch)
	dy := math.Sin(p.pitch)
	// Validate the short muzzle offset too; otherwise a player pressed against
	// a thin wall could spawn a projectile on its far side.
	if _, hit := w.solidShotHit(p.x, playerY(p)+1.45, p.z, p.x+dx*.8, playerY(p)+1.45+dy*.8, p.z+dz*.8); hit {
		return errors.New("Move back from the obstacle before throwing.")
	}
	takeHotbar(p, item)
	delete(p.buffs, "spawn_shield")
	w.sequence++
	w.projectiles = append(w.projectiles, &Projectile{ID: strconv.FormatUint(w.sequence, 10), Owner: p.profile.ID, Item: item, X: p.x + dx*.8, Y: playerY(p) + 1.45 + dy*.8, Z: p.z + dz*.8, VX: dx * speed, VY: dy * speed, VZ: dz * speed, damage: damage, born: w.time, super: w.active(p, "superstrength")})
	p.cooldowns["throw"] = w.time + .6/(1+arms*.08)
	return nil
}

func (w *World) eat(p *player, item string) error {
	if item == "" {
		item = p.profile.Selected
	}
	r, ok := recipeFor(item)
	if !ok {
		return errors.New("Choose a cookie to eat.")
	}
	if p.cooldowns["eat"] > w.time {
		return errors.New("Finish your cookie before eating another.")
	}
	if !hasHotbar(p, item) {
		return errors.New("Move that cookie into your hotbar before eating it.")
	}
	takeHotbar(p, item)
	p.health = min(100, p.health+r.Heal)
	p.cooldowns["eat"] = w.time + 1
	switch item {
	case "cactus_cookie":
		p.buffs["speed"] = w.time + 12
	case "sun_cookie":
		p.buffs["range"] = w.time + 15
	case "salt_cookie":
		p.buffs["swim"] = w.time + 15
	case "protein_cookie":
		p.buffs["superstrength"] = w.time + 10
	}
	w.event(p.profile.Name+" ate a cookie.", "eat", p.x, p.z)
	return nil
}

func (w *World) interact(p *player, target string) error {
	if target == "island_portal" {
		return w.islandAction(p, Action{Action: "teleport_cave"})
	}
	if target == "pink_lollipop" {
		return w.islandAction(p, Action{Action: "eat_lollipop", Item: "pink"})
	}
	if target == "blue_lollipop" {
		return w.islandAction(p, Action{Action: "eat_lollipop", Item: "blue"})
	}
	if p.island != "" {
		return w.islandAction(p, Action{Action: "use_build", Target: target})
	}
	if p.cooldowns["interact"] > w.time {
		return errors.New("Wait a moment before gathering again.")
	}
	for _, a := range w.animals {
		if a.ID == target {
			return w.helpAnimal(p, a)
		}
	}
	for _, n := range w.nodes {
		if n.ID != target || n.Island != p.island {
			continue
		}
		if distance(p.x, p.z, n.X, n.Z) > 5 {
			return errors.New("Move closer to interact (within 5 metres).")
		}
		if !n.Available {
			return errors.New("This resource is regrowing. Come back shortly.")
		}
		if n.Kind == "wood" {
			return w.chopTree(p, n)
		}
		if n.respawn <= 0 {
			switch n.Kind {
			case "fire":
				w.event("Use the recipe book to bake cookies by this fire.", "hint", p.x, p.z)
			case "dummy":
				w.event("Aim and left-click to practice throwing at the dummy.", "hint", p.x, p.z)
			case "peace":
				w.event("Peace pays 3 coins for every kilogram of beach trash.", "hint", p.x, p.z)
			case "gym":
				w.event("Train legs, stamina, arms, or swimming for 2 coins. Protein speeds recovery.", "hint", p.x, p.z)
			case "bakery_plot":
				w.event("Buy this land for 250 coins to open your own bakery.", "hint", p.x, p.z)
			case "desert_trader":
				w.event("Saffron trades special cookies for berries, nuts, shells, or treasure.", "hint", p.x, p.z)
			default:
				w.event("Open your inventory to see the actions available here.", "hint", p.x, p.z)
			}
			p.cooldowns["interact"] = w.time + .4
			return nil
		}
		if n.Kind == "chest" {
			if !gain(p, "dough", 3) {
				return errors.New("Your inventory is full. Move or use a stack first.")
			}
			if !gain(p, "berry", 1) {
				return errors.New("Your inventory is full. Move or use a stack first.")
			}
			addCoins(p, 2)
			w.event("Chest found: 3 dough, a berry, and 2 coins.", "gather", p.x, p.z)
		} else {
			if !gain(p, n.Kind, n.Amount) {
				return errors.New("Your inventory is full. Move or use a stack first.")
			}
			w.event(fmt.Sprintf("Gathered %d %s.", n.Amount, n.Kind), "gather", p.x, p.z)
		}
		n.Available = false
		n.readyAt = w.time + n.respawn
		p.cooldowns["interact"] = w.time + .4
		return nil
	}
	return errors.New("There is nothing to interact with here.")
}

func (w *World) craft(p *player, item string, amount int) error {
	if p.cooldowns["craft"] > w.time {
		return errors.New("Your cookies are still baking.")
	}
	if item == "fire" {
		if p.island != "" {
			return w.islandAction(p, Action{Action: "build", Item: "fire", X: p.x + 2, Y: p.y, Z: p.z})
		}
		if inWater(p) {
			return errors.New("You need dry land to build a fire.")
		}
		fireX, fireZ := min(MaxX-1, p.x+2), p.z
		if waterAt(fireX, fireZ) || w.solidBlocked(fireX, fireZ, .7) || w.homeBlocked(p, fireX, fireZ) {
			return errors.New("Choose clear, dry ground for your campfire.")
		}
		for _, n := range w.nodes {
			if n.Kind == "fire" && n.Island == p.island && distance(p.x, p.z, n.X, n.Z) < 7 {
				return errors.New("There is already a cooking fire nearby.")
			}
		}
		if !afford(p, map[string]int{"wood": 2, "stick": 2, "stone": 3}, 1) {
			return errors.New("A fire needs 2 wood, 2 sticks, and 3 stones.")
		}
		fires := 0
		for _, n := range w.nodes {
			if n.Kind == "fire" {
				fires++
			}
		}
		if fires >= 80 {
			return errors.New("There are plenty of campfires already. Use an existing one.")
		}
		spend(p, map[string]int{"wood": 2, "stick": 2, "stone": 3}, 1)
		w.sequence++
		w.nodes = append(w.nodes, &Node{ID: "fire_" + strconv.FormatUint(w.sequence, 10), Kind: "fire", Label: "Campfire", X: fireX, Z: fireZ, Available: true, Owner: p.profile.ID, ExpiresAt: w.time + 25*60})
		p.cooldowns["craft"] = w.time + 1
		w.event("A cooking fire is ready. Time to bake!", "craft", p.x, p.z)
		return nil
	}
	r, ok := recipeFor(item)
	if !ok {
		return errors.New("That recipe is unknown.")
	}
	atBakery := p.profile.Bakery && w.near(p, p.profile.BakeryPlot, 10)
	atHomeOven := w.nearIslandAppliance(p, "oven")
	if item == "cake" && !atBakery && !atHomeOven {
		return errors.New("This recipe needs the oven in your own bakery.")
	}
	inDesert := p.island == "" && area(p.x, p.z) == "desert"
	if item == "sun_cookie" && !inDesert && !atBakery && !atHomeOven {
		return errors.New("This recipe needs desert heat or your bakery oven.")
	}
	canBake := atBakery || atHomeOven || w.nearIslandAppliance(p, "fire")
	if h := p.profile.Home; h != nil && distance(p.x, p.z, h.X, h.Z) <= 7 {
		canBake = true
	}
	for _, n := range w.nodes {
		if n.Kind == "fire" && n.Island == p.island && n.Available && distance(p.x, p.z, n.X, n.Z) <= 7 {
			canBake = true
			break
		}
	}
	if !canBake {
		return errors.New("Use your cookbook beside a campfire, home hearth, or your bakery oven.")
	}
	cost := cloneMap(r.Cost)
	if item == "protein_cookie" {
		// Either protein product can be baked; consume available powder first.
		powder := min(amount, p.profile.Inventory["protein_powder"])
		cost = map[string]int{"dough": amount, "protein_powder": powder, "protein_drink": amount - powder}
	} else {
		for key, count := range cost {
			cost[key] = count * amount
		}
	}
	if !afford(p, cost, 1) {
		return errors.New("You need more ingredients for this recipe.")
	}
	spend(p, cost, 1)
	produced := amount
	if atBakery {
		produced += int(float64(amount) * .35 * bakeryBoost(p))
	}
	if !gain(p, item, produced) {
		return errors.New("Your inventory is full. Move or use a stack first.")
	}
	p.profile.Discovered[item] = true
	awardHats(&p.profile)
	rest := 1.0
	if atBakery {
		rest *= 1 - .4*bakeryBoost(p)
		if p.profile.Equipment["oven"] {
			rest *= .8
		}
	}
	p.cooldowns["craft"] = w.time + rest
	w.eventActor(fmt.Sprintf("Baked %d %s. Saved in your cookbook.", produced, r.Name), "craft", p.x, p.z, p.profile.ID)
	return nil
}

func afford(p *player, cost map[string]int, amount int) bool {
	for item, count := range cost {
		if p.profile.Inventory[item] < count*amount {
			return false
		}
	}
	return true
}
func spend(p *player, cost map[string]int, amount int) {
	for item, count := range cost {
		take(p, item, count*amount)
	}
}

func (w *World) trade(p *player, target, item string, amount int) error {
	if target == "bakery" {
		target = p.profile.BakeryPlot
	}
	if !w.near(p, target, 7) {
		return errors.New("Move closer to the trader or shop.")
	}
	kind := target
	if strings.HasPrefix(target, "desert_trader") {
		kind = "desert_trader"
	}
	if w.isPlot(target) {
		kind = "bakery"
	}
	switch kind {
	case "kitchen_shop":
		return w.kitchenTrade(p, item, amount)
	case "paint_shop":
		if !strings.HasPrefix(item, "paint_") {
			return errors.New("The paint shop sells paint for your bakery.")
		}
		return w.kitchenTrade(p, item, amount)
	case "general_shop":
		return w.generalTrade(p, item, amount)
	case "peace":
		prices := map[string]int{"trash": 3, "shell": 2, "stone": 1, "pearl": 12, "treasure": 25, "old_coin": 5, "rare_item": 60}
		price, ok := prices[item]
		if !ok {
			return errors.New("Peace buys trash, seashells, stones, pearls, old coins, treasure, and rare finds.")
		}
		if p.profile.Inventory[item] < amount {
			return errors.New("You do not have enough to trade.")
		}
		take(p, item, amount)
		if item == "trash" {
			p.profile.Cleaned += amount
			awardHats(&p.profile)
		}
		addCoins(p, price*amount)
		w.event(fmt.Sprintf("Peace paid %d coins. Keep the coast beautiful!", price*amount), "trade", p.x, p.z)
		return nil
	case "desert_trader":
		switch item {
		case "berry":
			if p.profile.Inventory["berry"] < 3*amount {
				return errors.New("Saffron wants 3 berries for 2 mysterious desert cookies.")
			}
			take(p, "berry", 3*amount)
			if !gain(p, "sun_cookie", 2*amount) {
				return errors.New("Your inventory is full. Move or use a stack first.")
			}
		case "nut", "shell":
			cost := 3
			if item == "shell" {
				cost = 4
			}
			if p.profile.Inventory[item] < cost*amount {
				return fmt.Errorf("Offer %d %s for two mysterious desert cookies.", cost, item)
			}
			take(p, item, cost*amount)
			if !gain(p, "cactus_cookie", 2*amount) {
				return errors.New("Your inventory is full. Move or use a stack first.")
			}
		case "treasure":
			if p.profile.Inventory["treasure"] < amount {
				return errors.New("Saffron wants an ocean treasure for a special cookie box.")
			}
			take(p, "treasure", amount)
			if !gain(p, "sun_cookie", 3*amount) {
				return errors.New("Your inventory is full. Move or use a stack first.")
			}
			if !gain(p, "cactus_cookie", 3*amount) {
				return errors.New("Your inventory is full. Move or use a stack first.")
			}
		default:
			return errors.New("Desert villagers barter for berries, nuts, shells or ocean treasure. They do not use coins.")
		}
		w.event("Saffron shared a box of special desert cookies.", "trade", p.x, p.z)
		return nil
	case "vending":
		price := 0
		if item == "protein_powder" {
			price = 8
		}
		if item == "protein_drink" {
			price = 6
		}
		if price == 0 {
			return errors.New("The vending machine sells protein powder and protein drinks.")
		}
		if p.profile.Coins < price*amount {
			return errors.New("You need more coins for that protein purchase.")
		}
		p.profile.Coins -= price * amount
		if !gain(p, item, amount) {
			return errors.New("Your inventory is full. Move or use a stack first.")
		}
		return nil
	case "bakery":
		if target != p.profile.BakeryPlot {
			return errors.New("You can only sell at your own bakery.")
		}
		if err := w.requireBakery(p); err != nil {
			return err
		}
		if _, ok := recipeFor(item); !ok {
			return errors.New("Your bakery sells cookies and cakes.")
		}
		if p.profile.Inventory[item] < amount {
			return errors.New("You do not have enough baked goods to sell.")
		}
		price := 2
		if item != "sugar" {
			price = 4
		}
		if item == "cake" {
			price = 12
		}
		// Round once for the whole sale, so upgrades also benefit inexpensive
		// sugar cookies when customers buy a batch.
		total := price*amount + int(float64(price*amount)*.5*bakeryBoost(p))
		if p.profile.Equipment["display"] {
			total += amount
		}
		take(p, item, amount)
		addCoins(p, total)
		w.event(fmt.Sprintf("Bakery customers paid %d coins.", total), "trade", p.x, p.z)
		return nil
	default:
		return errors.New("This place does not offer trades.")
	}
}

func (w *World) train(p *player, kind string) error {
	if kind != "legs" && kind != "stamina" && kind != "arms" && kind != "swim" {
		return errors.New("Train legs, stamina, arms, or swimming.")
	}
	if !w.near(p, "gym", 8) {
		return errors.New("Visit the gym in the city to train.")
	}
	if p.profile.Levels[kind] >= 10 {
		return errors.New("You have reached the maximum level in that workout.")
	}
	if p.cooldowns["train"] > w.time {
		return errors.New("Catch your breath before your next workout.")
	}
	if p.profile.Coins < 2 {
		return errors.New("A gym workout costs 2 coins.")
	}
	p.profile.Coins -= 2
	p.profile.Levels[kind]++
	rest := 4.0
	if w.active(p, "protein") {
		rest = 1.5
	}
	p.cooldowns["train"] = w.time + rest
	w.event(fmt.Sprintf("%s training reached level %d.", kind, p.profile.Levels[kind]), "train", p.x, p.z)
	return nil
}

func (w *World) helpAnimal(p *player, a *Animal) error {
	if p.island != "" {
		return errors.New("Visit the ocean to help wild animals.")
	}
	if a.Egg {
		return w.disturbEgg(p, a)
	}
	if a.Health <= 0 || distance(p.x, p.z, a.X, a.Z) > 5 {
		return errors.New("Swim closer to help that animal.")
	}
	if a.Need != "" {
		if a.Need == "wounded" {
			if p.profile.Inventory["sugar"] < 1 {
				return errors.New("A wounded animal needs one plain starter cookie to recover.")
			}
			take(p, "sugar", 1)
		}
		if !gain(p, "pearl", 1) {
			return errors.New("Make room for the animal's pearl before helping.")
		}
		a.Health = a.MaxHealth
		a.Need = ""
		a.needAt = w.time + 120
		p.profile.Reputation[a.ID] = min(10, p.profile.Reputation[a.ID]+3)
		p.profile.Rescues++
		awardHats(&p.profile)
		for _, witness := range w.animals {
			if witness.ID != a.ID && witness.Health > 0 && distance(a.X, a.Z, witness.X, witness.Z) <= 30 {
				p.profile.Reputation[witness.ID] = min(10, p.profile.Reputation[witness.ID]+1)
			}
		}
		a.rewardAt[p.profile.ID] = w.time + 45
		p.cooldowns["interact"] = w.time + .5
		w.event("You helped a "+a.Species+". Nearby animals remember your kindness. You found a pearl!", "animal", a.X, a.Z)
		return nil
	}
	if p.profile.Reputation[a.ID] < 2 {
		return errors.New("This animal does not trust you yet. Help trapped or wounded ocean animals.")
	}
	if a.rewardAt[p.profile.ID] > w.time {
		return errors.New("Your ocean friend is still looking for another treasure.")
	}
	items := []string{"pearl", "old_coin", "treasure", "rare_item"}
	item := items[int(w.sequence)%len(items)]
	if !gain(p, item, 1) {
		return errors.New("Your inventory is full. Move or use a stack first.")
	}
	a.rewardAt[p.profile.ID] = w.time + 60
	p.cooldowns["interact"] = w.time + .5
	w.event("Your "+a.Species+" friend led you to a "+item+"!", "animal", a.X, a.Z)
	return nil
}
