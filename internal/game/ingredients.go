package game

import (
	"errors"
	"fmt"
	"math"
)

var desertIngredientPrices = map[string]int{
	"berry": 2, "nut": 2, "cactus": 2, "sea_salt": 3,
	"protein_powder": 8, "protein_drink": 6, "wood": 2, "stick": 1, "stone": 1,
}

type BakingIngredient struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}

var bakingIngredients = []BakingIngredient{
	{ID: "baking_sugar", Name: "Sugar", Price: 4},
	{ID: "butter", Name: "Butter", Price: 5},
	{ID: "flour", Name: "Flour", Price: 3},
	{ID: "egg", Name: "Egg", Price: 3},
}

func (w *World) insideIngredientShop(p *player, target string) bool {
	if target != "ingredient_shop" || p.island != "" || p.inCave || !w.near(p, target, 7) {
		return false
	}
	for _, shop := range w.layout.Props {
		if shop.Kind == "ingredient_shop" && math.Abs(p.x-shop.X) <= shop.Width/2-.5 && math.Abs(p.z-shop.Z) <= shop.Depth/2-.5 && playerY(p) >= -.1 && playerY(p)+1.7 <= shop.Height {
			return true
		}
	}
	return false
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
	if target == "ingredient_shop" {
		if !w.insideIngredientShop(p, target) {
			return errors.New("Step inside Baker's Pantry in Crumb City to buy sugar, butter, flour, and eggs.")
		}
		for _, ingredient := range bakingIngredients {
			if ingredient.ID != item {
				continue
			}
			cost := ingredient.Price * amount
			if p.profile.Coins < cost {
				return fmt.Errorf("That purchase costs %d coins.", cost)
			}
			if !gain(p, item, amount) {
				return errors.New("Make room in your inventory before buying supplies.")
			}
			p.profile.Coins -= cost
			w.eventActor(fmt.Sprintf("Bought %d %s for %d coins.", amount, ingredient.Name, cost), "trade", p.x, p.z, p.profile.ID)
			return nil
		}
		return errors.New("Choose sugar, butter, flour, or eggs from Baker's Pantry.")
	}
	if !w.insideDesertTrader(p, target) {
		return errors.New("Go inside a desert trader's house to buy baking ingredients and supplies.")
	}
	price, ok := desertIngredientPrices[item]
	if !ok {
		return errors.New("Choose berries, nuts, cactus, salt, protein, or building supplies. Find dough in chests or mix it at your bakery.")
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
