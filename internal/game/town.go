package game

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

type TownRank struct {
	Rank  int     `json:"rank"`
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}
type TownLeaderboard struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Unit    string     `json:"unit"`
	Entries []TownRank `json:"entries"`
	Self    *TownRank  `json:"self,omitempty"`
	Total   int        `json:"total"`
}
type TownFlower struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SellPrice int    `json:"sellPrice"`
	Color     string `json:"color"`
}
type TownBouquet struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}
type BettyLine struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Reply string `json:"reply"`
}
type BettyView struct {
	Name     string      `json:"name"`
	Greeting string      `json:"greeting"`
	Lines    []BettyLine `json:"lines"`
}
type TownView struct {
	Leaderboards  []TownLeaderboard `json:"leaderboards"`
	FlowerCatalog []TownFlower      `json:"flowerCatalog"`
	Bouquet       TownBouquet       `json:"bouquet"`
	Bouquets      []TownBouquet     `json:"bouquets"`
	TreatCatalog  []CamelTreat      `json:"treatCatalog"`
	Betty         BettyView         `json:"betty"`
}

var townFlowers = []TownFlower{
	{"flower_daisy", "Daisy", 3, "#f9eac2"},
	{"flower_tulip", "Pink tulip", 5, "#e89eb4"},
	{"flower_lavender", "Lavender", 4, "#aa91d2"},
}
var townBouquet = TownBouquet{"flower_bouquet", "A little sunshine", 20}
var townBouquets = []TownBouquet{
	townBouquet,
	{"bouquet_daisies", "Sunny daisies", 15},
	{"bouquet_tulips", "Rosy tulips", 30},
	{"bouquet_lavender", "Lavender wishes", 25},
}
var bettyLines = []BettyLine{
	{"day", "How is your day, Betty?", "Lovely, dear. The daisies opened this morning, and now you've stopped for a chat. That's two good things already."},
	{"flowers", "Which flowers are your favorites?", "Daisies, I think. They turn up with such cheerful little faces. You can pick a stem in the garden outside, and I'll pay you for it at the counter."},
	{"advice", "Any advice for a busy day?", "Leave a little room for a rest, dear. Even the flowers don't bloom all at once. You're welcome to sit with your thoughts here."},
	{"gift", "Who should I give a bouquet to?", "Someone you want to make smile. It needn't be a special occasion. Buy a bouquet, stand near your friend, and give it from your inventory."},
	{"city", "What do you like about Crumb City?", "The small kindnesses. A baker saving the last bun, a neighbor watering a pot, someone remembering your name. Those are my favorite city records."},
}

func townFlowerFor(item string) (TownFlower, bool) {
	for _, flower := range townFlowers {
		if flower.ID == item {
			return flower, true
		}
	}
	return TownFlower{}, false
}
func isTownItem(item string) bool {
	_, flower := townFlowerFor(item)
	_, treat := camelTreatFor(item)
	_, bouquet := townBouquetFor(item)
	return flower || treat || bouquet
}
func townBouquetFor(item string) (TownBouquet, bool) {
	for _, bouquet := range townBouquets {
		if bouquet.ID == item {
			return bouquet, true
		}
	}
	return TownBouquet{}, false
}

// Old saves start at zero: a current wallet cannot reconstruct lifetime income.
func normalizeTownStats(p *Profile) {
	p.CamelCoins = max(0, min(coinLimit, p.CamelCoins))
	p.CookieRingKills = max(0, min(coinLimit, p.CookieRingKills))
	p.CoinsEarned = max(0, min(coinLimit, p.CoinsEarned))
	if !finite(p.PlaySeconds) || p.PlaySeconds < 0 {
		p.PlaySeconds = 0
	}
	p.PlaySeconds = min(float64(coinLimit), p.PlaySeconds)
}
func recordCoinsEarned(p *player, count int) {
	if count > 0 {
		p.profile.CoinsEarned += min(count, max(0, coinLimit-p.profile.CoinsEarned))
	}
}
func (w *World) tickTownStats(dt float64) {
	if !finite(dt) || dt <= 0 {
		return
	}
	for _, p := range w.players {
		if !p.disconnected {
			p.profile.PlaySeconds = min(float64(coinLimit), p.profile.PlaySeconds+dt)
		}
	}
}

func (w *World) townSnapshot(self string) TownView {
	// Copy scalar ranking data only, with live state taking precedence over saves.
	profiles := make(map[string]Profile, len(w.profiles)+len(w.players))
	for id, p := range w.profiles {
		profiles[id] = p
	}
	for id, p := range w.players {
		profiles[id] = p.profile
	}
	town := TownView{Leaderboards: []TownLeaderboard{}, FlowerCatalog: append([]TownFlower{}, townFlowers...), Bouquet: townBouquet, Bouquets: append([]TownBouquet{}, townBouquets...), TreatCatalog: append([]CamelTreat{}, camelTreats...),
		Betty: BettyView{"Betty", "Hello, dear. Welcome to Petal & Posy. Stay for a little chat if you like.", append([]BettyLine{}, bettyLines...)}}
	boards := []struct {
		id, name, unit string
		score          func(Profile) float64
	}{
		{"camel_coins", "Most camel coins", "coins", func(p Profile) float64 { return float64(p.CamelCoins) }},
		{"play_time", "Most time played", "seconds", func(p Profile) float64 { return math.Floor(p.PlaySeconds + 1e-7) }},
		{"coins_earned", "Most coins earned", "coins", func(p Profile) float64 { return float64(p.CoinsEarned) }},
		{"kills", "Most kills", "kills", func(p Profile) float64 { return float64(p.Kills) }},
		{"cookie_ring_kills", "Cookie ring kills", "kills", func(p Profile) float64 { return float64(p.CookieRingKills) }},
	}
	for _, definition := range boards {
		entries := make([]TownRank, 0, len(profiles))
		for id, p := range profiles {
			name := p.CallName
			if name == "" {
				name = p.Name
			}
			if name == "" {
				name = "Cookie explorer"
			}
			entries = append(entries, TownRank{ID: id, Name: name, Value: definition.score(p)})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Value != entries[j].Value {
				return entries[i].Value > entries[j].Value
			}
			left, right := strings.ToLower(entries[i].Name), strings.ToLower(entries[j].Name)
			if left != right {
				return left < right
			}
			return entries[i].ID < entries[j].ID
		})
		board := TownLeaderboard{ID: definition.id, Name: definition.name, Unit: definition.unit, Entries: []TownRank{}, Total: len(entries)}
		for i := range entries {
			entries[i].Rank = i + 1
			if i > 0 && entries[i].Value == entries[i-1].Value {
				entries[i].Rank = entries[i-1].Rank
			}
			if entries[i].ID == self {
				own := entries[i]
				board.Self = &own
			}
			if i < 10 {
				board.Entries = append(board.Entries, entries[i])
			}
		}
		town.Leaderboards = append(town.Leaderboards, board)
	}
	return town
}

func (w *World) seedTown() {
	building := func(id, kind, label string, x, z, width, depth float64) {
		prop := WorldProp{ID: id, Kind: kind, Label: label, X: x, Z: z, Width: width, Depth: depth, Height: 4.8, Scale: 1}
		w.layout.Props = append(w.layout.Props, prop)
		w.buildingColliders(prop)
		w.colliders = append(w.colliders, Collider{ID: id + "_ceiling", X: x, Z: z, Width: width, Depth: depth, Y: 4.8, Height: .3})
	}
	building("town_hall", "town_hall", "CRUMB CITY RECORDS", 130, 153, 18, 16)
	building("town_flower_shop", "town_flower_shop", "PETAL & POSY", 168, 153, 14, 12)
	building("town_pet_shop", "town_pet_shop", "PAWS & PALMS", 198, 153, 16, 12)
	w.layout.Paths = append(w.layout.Paths,
		MapRect{"town_lane", 164, 168, 90, 4}, MapRect{"town_market_link", 180, 148, 4, 44},
		MapRect{"town_records_walk", 130, 164, 3.2, 8}, MapRect{"town_flowers_walk", 168, 162.5, 3.2, 11}, MapRect{"town_pets_walk", 198, 162.5, 3.2, 11})
	add := func(id, kind, label string, x, z float64) {
		w.nodes = append(w.nodes, &Node{ID: id, Kind: kind, Label: label, X: x, Z: z, Available: true})
	}
	for i, board := range []struct{ id, name string }{{"camel_coins", "Most camel coins"}, {"play_time", "Most time played"}, {"coins_earned", "Most coins earned"}, {"kills", "Most kills"}, {"cookie_ring_kills", "Cookie ring kills"}} {
		add("board_"+board.id, "leaderboard_board", board.name, 123.6+float64(i)*3.2, 146)
	}
	add("flower_shop", "flower_shop", "Petal & Posy flower counter", 168, 153)
	add("betty", "betty", "Betty · a friendly chat", 171.8, 154.5)
	add("pet_shop", "pet_shop", "Paws & Palms pet store", 198, 153)
	for row := 0; row < 3; row++ {
		for column, flower := range townFlowers {
			x, z := 145+float64(column)*4, 145+float64(row)*4
			w.nodes = append(w.nodes, &Node{ID: fmt.Sprintf("town_flower_%d_%d", row, column), Kind: flower.ID, Label: flower.Name + " · pick one stem", X: x, Z: z, Available: true, Amount: 1, respawn: 45})
		}
	}
	// Counters have exactly matching scene geometry; their front aisles stay open.
	w.colliders = append(w.colliders,
		Collider{ID: "town_flower_counter", X: 168, Z: 152, Width: 4, Depth: 1, Height: 1},
		Collider{ID: "town_pet_counter", X: 198, Z: 152, Width: 4, Depth: 1, Height: 1})
}

func (w *World) requireTownNode(p *player, target, kind string, radius float64) (*Node, error) {
	if target != kind {
		return nil, errors.New("Visit the matching town shop counter.")
	}
	for _, n := range w.nodes {
		if n.ID == target && n.Kind == kind && n.Available && sameNodeLayer(p, n) &&
			distance(p.x, p.z, n.X, n.Z) <= radius && math.Abs(playerY(p)-n.Y) <= 2 {
			return n, nil
		}
	}
	return nil, errors.New("Move closer to the town shop counter, on the same floor.")
}

func (w *World) townAction(p *player, a Action) error {
	if a.Action == "give_bouquet" {
		return w.giveBouquet(p, a.Target, a.Item)
	}
	if a.Amount < 1 || a.Amount > 20 {
		return errors.New("Choose between 1 and 20 flowers or bouquets.")
	}
	if _, err := w.requireTownNode(p, a.Target, "flower_shop", 5); err != nil {
		return err
	}
	switch a.Action {
	case "sell_flower":
		flower, ok := townFlowerFor(a.Item)
		if !ok {
			return errors.New("Betty buys single daisies, tulips, and lavender stems from the town garden.")
		}
		price := flower.SellPrice * a.Amount
		if price > coinLimit-p.profile.Coins {
			return errors.New("Your wallet is full. Spend some coins before selling flowers.")
		}
		if !take(p, a.Item, a.Amount) {
			return errors.New("Pick those flower stems in the garden first.")
		}
		addCoins(p, price)
		w.eventActor(fmt.Sprintf("Sold %d %s stems to Betty for %d coins.", a.Amount, flower.Name, price), "trade", p.x, p.z, p.profile.ID)
		return nil
	case "buy_bouquet":
		if a.Item == "" {
			a.Item = townBouquet.ID
		}
		bouquet, ok := townBouquetFor(a.Item)
		if !ok {
			return errors.New("Choose one of Betty's flower bouquets.")
		}
		price := bouquet.Price * a.Amount
		if p.profile.Coins < price {
			return errors.New("You do not have enough coins for that bouquet.")
		}
		if !gain(p, bouquet.ID, a.Amount) {
			return errors.New("Make room in your inventory for the bouquet.")
		}
		p.profile.Coins -= price
		return nil
	}
	return errors.New("Choose a flower shop action.")
}

func (w *World) giveBouquet(p *player, target, item string) error {
	if item == "" {
		item = townBouquet.ID
	}
	bouquet, ok := townBouquetFor(item)
	if !ok {
		return errors.New("Choose a flower bouquet to give.")
	}
	recipient := w.players[target]
	if recipient == nil || recipient == p || recipient.disconnected || p.disconnected {
		return errors.New("Choose another nearby player who is online.")
	}
	if p.island != recipient.island || p.inCave != recipient.inCave || math.Abs(playerY(p)-playerY(recipient)) > 2 || distance(p.x, p.z, recipient.x, recipient.z) > 4 {
		return errors.New("Stand within four metres of your friend on the same floor to give a bouquet.")
	}
	// Prepare both inventories first: neither player loses or gains a partial gift.
	giverNext, receiverNext := &player{profile: cloneProfile(p.profile)}, &player{profile: cloneProfile(recipient.profile)}
	if !take(giverNext, bouquet.ID, 1) {
		return errors.New("Buy a bouquet from Betty before giving one.")
	}
	if !gain(receiverNext, bouquet.ID, 1) {
		return errors.New("Your friend's inventory is full. They need room for the bouquet.")
	}
	p.profile, recipient.profile = giverNext.profile, receiverNext.profile
	w.eventActor(p.profile.CallName+" gave "+bouquet.Name+" to "+recipient.profile.CallName+". A little kindness, just for you!", "gift", recipient.x, recipient.z, p.profile.ID)
	return nil
}
