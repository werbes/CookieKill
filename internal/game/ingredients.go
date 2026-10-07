package game

import (
	"errors"
	"fmt"
	"math"
)

var desertIngredientPrices = map[string]int{
	"dough": 3, "berry": 2, "nut": 2, "cactus": 2, "sea_salt": 3,
	"protein_powder": 8, "protein_drink": 6, "wood": 2, "stick": 1, "stone": 1,
}

func (w *World) insideDesertTrader(p *player, target string) bool {
	if p.island != "" || p.inCave || !w.near(p, target, 7) {
		return false
	}
	for _, n := range w.nodes {
		if n.ID != target || n.Kind != "desert_trader" {
			continue
		}
		for _, house := range w.layout.Props {
			if house.Kind == "desert_building" && house.X == n.X && house.Z == n.Z &&
				math.Abs(p.x-house.X) <= house.Width/2-.5 && math.Abs(p.z-house.Z) <= house.Depth/2-.5 &&
				playerY(p) >= -.1 && playerY(p)+1.7 <= house.Height {
				return true
			}
		}
	}
	return false
}

func (w *World) buyIngredients(p *player, target, item string, amount int) error {
	if !w.insideDesertTrader(p, target) {
		return errors.New("Go inside a desert trader's house to buy baking ingredients and supplies.")
	}
	price, ok := desertIngredientPrices[item]
	if !ok {
		return errors.New("Choose dough, berries, nuts, cactus, salt, protein, or building supplies from the trader.")
	}
	if p.profile.Coins < price*amount {
		return fmt.Errorf("That purchase costs %d coins.", price*amount)
	}
	if !gain(p, item, amount) {
		return errors.New("Make room in your inventory before buying supplies.")
	}
	p.profile.Coins -= price * amount
	w.eventActor(fmt.Sprintf("Bought %d %s for %d coins.", amount, item, price*amount), "trade", p.x, p.z, p.profile.ID)
	return nil
}
