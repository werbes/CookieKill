package game

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
)

// Keep all coin balances exactly representable by the browser's JSON numbers.
// Inventory stacks retain their separate, much smaller capacity.
const coinLimit = 9007199254740991

const (
	ClubX          = -202.0
	ClubMouthZ     = -204.0
	ClubStairTopZ  = -207.0
	ClubFloorY     = -12.0
	ClubStairCount = 48
	ClubStairRun   = 20.0 / ClubStairCount
	ClubStairRise  = .25
)

type ClubDrink struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Price     int     `json:"price"`
	Alcoholic bool    `json:"alcoholic"`
	Heal      float64 `json:"heal"`
	Stamina   float64 `json:"stamina"`
}

var clubDrinks = []ClubDrink{
	{"smirnoff_ice", "Smirnoff Ice", 8, true, 0, 0},
	{"beer", "Beer", 5, true, 0, 0},
	{"cider", "Cider", 6, true, 0, 0},
	{"cocktail", "House cocktail", 10, true, 0, 0},
	{"cola", "Cola", 3, false, 0, 12},
	{"lemonade", "Lemonade", 3, false, 0, 12},
	{"orange_juice", "Orange juice", 4, false, 3, 12},
	{"chocolate_shake", "Chocolate shake", 7, false, 8, 18},
	{"strawberry_shake", "Strawberry shake", 7, false, 8, 18},
	{"water", "Water", 2, false, 0, 15},
}

type ClubResult struct {
	ID         uint64  `json:"id"`
	Stake      int     `json:"stake"`
	Sector     int     `json:"sector"`
	Multiplier float64 `json:"multiplier"`
	Payout     int     `json:"payout"`
}

// Sector widths match their exact percentage chances. Integer arithmetic
// floors fractional coin payouts without floating-point rounding.
var clubSectorTenths = [...]int{50, 30, 20, 10, 5, 1, 100}
var clubSectorWeights = [...]int{7, 10, 10, 10, 30, 30, 3}

func clubSectorForRoll(roll int) int {
	if roll < 0 || roll >= 100 {
		return -1
	}
	for sector, weight := range clubSectorWeights {
		if roll < weight {
			return sector
		}
		roll -= weight
	}
	return -1
}

func randomClubSector() (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		return 0, err
	}
	return clubSectorForRoll(int(n.Int64())), nil
}

func clubDrinkFor(item string) (ClubDrink, bool) {
	for _, drink := range clubDrinks {
		if drink.ID == item {
			return drink, true
		}
	}
	return ClubDrink{}, false
}

func clubHalfWidth(z float64) float64 { return 3 + 1.4*max(0, min(1, (z-ClubStairTopZ)/3)) }
func clubMouth(x, z float64) bool {
	return math.Abs(x-ClubX) <= clubHalfWidth(z) && z < ClubMouthZ && z > ClubStairTopZ-2
}
func clubFootprint(x, z float64) bool {
	return math.Abs(x-ClubX) <= clubHalfWidth(z) && z <= ClubMouthZ && z >= -227 ||
		x >= -218 && x <= -186 && z <= -226 && z >= -250
}
func clubGroundY(x, z float64) float64 {
	if math.Abs(x-ClubX) <= 3 && z <= ClubMouthZ && z >= -227 {
		return -ClubStairRise * math.Ceil(max(0, (ClubStairTopZ-z)/ClubStairRun-1e-9))
	}
	if x >= -218 && x <= -186 && z <= -226 && z >= -250 {
		return ClubFloorY
	}
	return 0
}

func caveExitAtTop(p *player, x, z float64) bool {
	if clubFootprint(p.x, p.z) {
		return math.Abs(x-ClubX) <= clubHalfWidth(ClubMouthZ) && z >= ClubMouthZ && p.z >= ClubMouthZ-.5
	}
	return math.Abs(x-120) <= caveHalfWidth(CaveMouthZ) && z >= CaveMouthZ && p.z >= CaveMouthZ-.5
}

func (w *World) seedClub() {
	w.layout.Paths = append(w.layout.Paths, MapRect{"forest_club_trail", -118.5, -192, 167, 5}, MapRect{"forest_club_approach", ClubX, -198, 5, 12})
	w.layout.Props = append(w.layout.Props, WorldProp{ID: "forest_club", Kind: "club_entrance", Label: "The Root Room club", X: ClubX, Z: ClubMouthZ, Width: 10, Depth: 8, Height: 4.8, Scale: 1})
	addNode := func(id, kind, label string, x, z float64) {
		w.nodes = append(w.nodes, &Node{ID: id, Kind: kind, Label: label, X: x, Y: ClubFloorY, Z: z, Available: true, InCave: true})
	}
	addNode("club_bar", "club_bar", "The Root Room bar", -214, -232)
	addNode("club_casino", "club_casino", "Seven-sector coin wheel", -191, -237)
	addNode("club_cake_seller", "club_cake_seller", "Disco Duke · funny cakes", -191, -245)
	w.nodes[len(w.nodes)-1].Available = dayCycleAt(w.time).Phase == "night"
	for row, z := range []float64{-245, -248} {
		for col, x := range []float64{-213, -209, -205, -201} {
			addNode(fmt.Sprintf("club_seat_%d", row*4+col+1), "club_seat", "Club chair", x, z)
		}
	}
	wall := func(id string, x, z, width, depth, y, height float64) {
		w.colliders = append(w.colliders, Collider{ID: id, X: x, Z: z, Width: width, Depth: depth, Y: y, Height: height})
	}
	for i := 0; i < 6; i++ {
		z := ClubMouthZ - (float64(i)+.5)*.5
		for side, sign := range []float64{-1, 1} {
			wall(fmt.Sprintf("club_entry_wall_%d_%d", side, i), ClubX+sign*(clubHalfWidth(z)+.4), z, .8, .51, -.25, 4.8)
		}
	}
	for i := 0; i < ClubStairCount; i++ {
		z := ClubStairTopZ - (float64(i)+.5)*ClubStairRun
		y := -ClubStairRise * float64(i+1)
		for side, x := range []float64{ClubX - 3.4, ClubX + 3.4} {
			wall(fmt.Sprintf("club_stair_wall_%d_%d", side, i), x, z, .8, ClubStairRun+.02, y-.25, 5.05)
		}
		wall(fmt.Sprintf("club_stair_ceiling_%d", i), ClubX, z, 6, ClubStairRun+.01, y+4.4, .6)
	}
	for i, d := range [][4]float64{{-218.4, -238, .8, 24.8}, {-185.6, -238, .8, 24.8}, {-202, -250.4, 33.6, .8}, {-211.5, -226, 13, .8}, {-192.5, -226, 13, .8}} {
		wall(fmt.Sprintf("club_room_wall_%d", i), d[0], d[1], d[2], d[3], ClubFloorY, 6)
	}
	wall("club_room_ceiling", ClubX, -238, 32, 24, ClubFloorY+5.5, 6.5)
	wall("club_bar_counter", -215.5, -232, 2, 8, ClubFloorY, 1.15)
	wall("club_wheel_console", -188.5, -237, 2, 2, ClubFloorY, 1.2)
	// Match the four round cocktail tabletops in club-scenery.js. Their small
	// footprints leave both rows of chairs and the aisles approachable.
	for i, x := range []float64{-213, -209, -205, -201} {
		w.colliders = append(w.colliders, Collider{ID: fmt.Sprintf("club_table_%d", i+1), X: x, Z: -246.5, Radius: .55, Y: ClubFloorY, Height: .94})
	}
}

func (w *World) requireClubNode(p *player, target, kind string, radius float64) (*Node, error) {
	if kind == "club_cake_seller" && dayCycleAt(w.time).Phase != "night" {
		return nil, errors.New("Disco Duke sells funny cakes only at night.")
	}
	if p.island != "" || !p.inCave || !clubFootprint(p.x, p.z) {
		return nil, errors.New("Visit the underground forest club first.")
	}
	for _, n := range w.nodes {
		if n.ID == target && n.Kind == kind && sameNodeLayer(p, n) && w.clubNodeAvailable(n) && distance(p.x, p.z, n.X, n.Z) <= radius && math.Abs(playerY(p)-n.Y) <= 2 {
			return n, nil
		}
	}
	return nil, errors.New("Move closer to the club counter or chair.")
}

func (w *World) buyClubDrink(p *player, a Action) error {
	if _, err := w.requireClubNode(p, a.Target, "club_bar", 5); err != nil {
		return err
	}
	drink, ok := clubDrinkFor(a.Item)
	if !ok {
		return errors.New("Choose a drink from the bar menu.")
	}
	if p.profile.Coins < drink.Price*a.Amount {
		return errors.New("You do not have enough coins for that drink.")
	}
	if !gain(p, drink.ID, a.Amount) {
		return errors.New("Make room in your inventory before buying a drink.")
	}
	p.profile.Coins -= drink.Price * a.Amount
	return nil
}

func (w *World) drink(p *player, item string) error {
	if item == "" {
		item = p.profile.Selected
	}
	drink, ok := clubDrinkFor(item)
	if !ok {
		return errors.New("Choose a drink from your inventory.")
	}
	if p.cooldowns["eat"] > w.time {
		return errors.New("Finish your current refreshment first.")
	}
	if !take(p, item, 1) {
		return errors.New("You do not have that drink.")
	}
	if drink.Alcoholic {
		p.alcoholDrinks = min(100, p.alcoholDrinks+1)
	}
	p.health = min(100, p.health+drink.Heal)
	p.stamina = min(staminaCapacity(p), p.stamina+drink.Stamina)
	p.cooldowns["eat"] = w.time + 1
	w.eventActor("Enjoyed "+drink.Name+".", "eat", p.x, p.z, p.profile.ID)
	return nil
}

func (w *World) gambleClub(p *player, a Action) error {
	if _, err := w.requireClubNode(p, a.Target, "club_casino", 5); err != nil {
		return err
	}
	if a.Amount <= 0 || a.Amount > p.profile.Coins {
		return errors.New("Choose a whole-coin stake from 1 up to your wallet balance.")
	}
	if p.cooldowns["club_gamble"] > w.time {
		return errors.New("Wait for the current spin to finish.")
	}
	// Preflight the largest possible net gain so every advertised result pays
	// exactly, even after the wallet grows far beyond the old million-coin cap.
	if a.Amount > (coinLimit-p.profile.Coins)/9 {
		return fmt.Errorf("That stake could exceed the exact-number wallet limit. The maximum safe stake is %d coins.", min(p.profile.Coins, (coinLimit-p.profile.Coins)/9))
	}
	draw := w.clubRandom
	if draw == nil {
		draw = randomClubSector
	}
	sector, err := draw()
	if err != nil || sector < 0 || sector >= len(clubSectorTenths) {
		return errors.New("The wheel is unavailable. Your coins have not been spent.")
	}
	payout := a.Amount * clubSectorTenths[sector] / 10
	p.profile.Coins = p.profile.Coins - a.Amount + payout
	p.cooldowns["club_gamble"] = w.time + 1
	w.sequence++
	p.clubResult = &ClubResult{ID: w.sequence, Stake: a.Amount, Sector: sector, Multiplier: float64(clubSectorTenths[sector]) / 10, Payout: payout}
	return nil
}

func (w *World) sitClub(p *player, target string) error {
	n, err := w.requireClubNode(p, target, "club_seat", 2.5)
	if err != nil {
		return err
	}
	if p.ridingCamel {
		return errors.New("Dismount your camel before sitting down.")
	}
	if p.clubSeatID == target && p.posture == "sitting" {
		p.posture, p.clubSeatID = "", ""
		return nil
	}
	for _, other := range w.players {
		if other != p && other.clubSeatID == target && other.posture == "sitting" {
			return errors.New("Someone is already sitting in that chair.")
		}
	}
	p.x, p.z = n.X, n.Z
	resetMovement(p)
	p.posture, p.clubSeatID = "sitting", target
	return nil
}
