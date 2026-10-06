package game

import (
	"errors"
	"fmt"
	"strings"
)

func bakeryBoost(p *player) float64 { l := float64(p.profile.BakeryLevel); return l / (l + 3) }
func (w *World) buyLand(p *player, target string) error {
	if p.profile.Bakery {
		return errors.New("You already own a bakery.")
	}
	if target == "" {
		for _, n := range w.nodes {
			if n.Kind == "bakery_plot" && distance(p.x, p.z, n.X, n.Z) <= 8 {
				target = n.ID
				break
			}
		}
	}
	if !w.isPlot(target) || !w.near(p, target, 8) {
		return errors.New("Visit an unclaimed bakery plot in the city.")
	}
	if w.plotOwner(target) != "" {
		return errors.New("Another player owns this bakery plot.")
	}
	if p.profile.Coins < 250 {
		return errors.New("A bakery plot costs 250 coins.")
	}
	p.profile.Coins -= 250
	p.profile.Bakery = true
	p.profile.BakeryPlot = target
	w.rebuildPropertyColliders()
	awardHats(&p.profile)
	w.event(p.profile.Name+" opened a bakery!", "bakery", p.x, p.z)
	return nil
}
func (w *World) upgradeBakery(p *player) error {
	if err := w.requireBakery(p); err != nil {
		return err
	}
	cost := 50 + 25*p.profile.BakeryLevel
	if p.profile.Coins < cost {
		return fmt.Errorf("This bakery upgrade costs %d coins.", cost)
	}
	p.profile.Coins -= cost
	p.profile.BakeryLevel++
	w.event("Bakery upgraded: faster baking and mixing, better batch yields and sales.", "bakery", p.x, p.z)
	return nil
}
func (w *World) kitchenTrade(p *player, item string, amount int) error {
	prices := map[string]int{"mixer": 40, "oven": 60, "display": 35}
	if cost, ok := prices[item]; ok {
		if !p.profile.Bakery {
			return errors.New("Buy a bakery before furnishing its kitchen.")
		}
		if p.profile.Equipment[item] {
			return errors.New("You already own that kitchen equipment.")
		}
		if p.profile.Coins < cost {
			return errors.New("You need more coins for that equipment.")
		}
		p.profile.Coins -= cost
		p.profile.Equipment[item] = true
		if item == "display" {
			w.rebuildPropertyColliders()
		}
		return nil
	}
	color := strings.TrimPrefix(item, "paint_")
	if !strings.HasPrefix(item, "paint_") || !validColor(color) {
		return errors.New("Choose kitchen equipment or paint.")
	}
	if !p.profile.Equipment["mixer"] || !p.profile.Equipment["oven"] || !p.profile.Equipment["display"] {
		return errors.New("Complete your kitchen equipment collection to unlock paint.")
	}
	if p.profile.Coins < 12*amount {
		return errors.New("Paint costs 12 coins per tin.")
	}
	p.profile.Coins -= 12 * amount
	if !gain(p, item, amount) {
		return errors.New("Your inventory is full. Move or use a stack first.")
	}
	return nil
}
func (w *World) paintBakery(p *player, target, color string) error {
	if err := w.requireBakery(p); err != nil {
		return err
	}
	if !validChoice(target, "walls", "oven", "mixer", "display") || !validColor(color) {
		return errors.New("Choose a bakery surface and a paint color.")
	}
	if target != "walls" && !p.profile.Equipment[target] {
		return errors.New("Buy this equipment before painting it.")
	}
	if !take(p, "paint_"+color, 1) {
		return errors.New("Buy a tin of that paint at the kitchen shop.")
	}
	p.profile.Paint[target] = color
	return nil
}
func (w *World) generalTrade(p *player, item string, amount int) error {
	prices := map[string]int{"dough": 3, "berry": 2, "nut": 2, "wood": 2, "stick": 1, "stone": 1, "sea_salt": 3}
	price, ok := prices[item]
	if !ok {
		return errors.New("This city shop sells dough, ingredients and building supplies.")
	}
	if p.profile.Coins < price*amount {
		return errors.New("You need more coins for those supplies.")
	}
	p.profile.Coins -= price * amount
	if !gain(p, item, amount) {
		return errors.New("Your inventory is full. Move or use a stack first.")
	}
	return nil
}
