package game

import "errors"

type ClubCake struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    int     `json:"price"`
	Duration float64 `json:"duration"`
}

var clubCakes = []ClubCake{
	{"funny_cake", "Funny cake", 15, 45},
	{"cosmic_cake", "Cosmic cake", 25, 75},
}

func clubCakeFor(item string) (ClubCake, bool) {
	for _, cake := range clubCakes {
		if cake.ID == item {
			return cake, true
		}
	}
	return ClubCake{}, false
}

// Consult the clock for every request: a menu opened at night must not permit
// a purchase after dawn, even before a node's next simulation update.
func (w *World) clubNodeAvailable(n *Node) bool {
	if n.Kind == "club_cake_seller" {
		return dayCycleAt(w.time).Phase == "night"
	}
	return n.Available
}

func (w *World) buyClubCake(p *player, a Action) error {
	if _, err := w.requireClubNode(p, a.Target, "club_cake_seller", 5); err != nil {
		return err
	}
	cake, ok := clubCakeFor(a.Item)
	if !ok {
		return errors.New("Choose a cake from Disco Duke's menu.")
	}
	if p.profile.Coins < cake.Price*a.Amount {
		return errors.New("You do not have enough coins for that cake.")
	}
	if !gain(p, cake.ID, a.Amount) {
		return errors.New("Make room in your inventory before buying a cake.")
	}
	p.profile.Coins -= cake.Price * a.Amount
	return nil
}

func (w *World) eatClubCake(p *player, cake ClubCake) error {
	if p.cooldowns["eat"] > w.time {
		return errors.New("Finish your current refreshment first.")
	}
	if !take(p, cake.ID, 1) {
		return errors.New("You do not have that cake.")
	}
	// This timed effect is presentation only; movement and aim stay authoritative.
	p.buffs["funny_cake"] = w.time + cake.Duration
	p.cooldowns["eat"] = w.time + 1
	w.eventActor("Enjoyed "+cake.Name+". The colors are getting curious!", "eat", p.x, p.z, p.profile.ID)
	return nil
}
