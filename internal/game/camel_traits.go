package game

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
)

// Stable birth traits also migrate old camels without rerolling on reconnect.
func camelTraits(c *Camel) {
	if c.TraitsVersion < 1 {
		seed := sha256.Sum256([]byte("cookiekill-camel-birth:" + c.ID))
		value := func(offset int, low, high float64) float64 {
			n := float64(binary.LittleEndian.Uint32(seed[offset:offset+4])) / float64(math.MaxUint32)
			return math.Round((low+n*(high-low))*100) / 100
		}
		c.Speed, c.Jump = value(0, 9, 12), value(4, 5.5, 7.5)
		c.Stamina, c.Happiness = value(8, 90, 140), value(12, 40, 90)
		c.TraitsVersion = 1
	}
	limit := func(value, low, high float64) float64 {
		if !finite(value) {
			return low
		}
		return max(low, min(high, value))
	}
	c.Speed, c.Jump = limit(c.Speed, 9, 16), limit(c.Jump, 5.5, 10)
	c.Stamina, c.Happiness = limit(c.Stamina, 90, 200), limit(c.Happiness, 0, 100)
	for _, stat := range []string{"speed", "jump", "stamina", "happiness"} {
		progress, boost := trainingValue(&c.Training, stat), trainingValue(&c.TrainingBoost, stat)
		*progress, *boost = limit(*progress, 0, 99.999), limit(*boost, 0, 240)
	}
}

func camelBaseSpeed(p *player) float64 {
	c := mountedCamel(p)
	if c == nil {
		return 0
	}
	camelTraits(c)
	return c.Speed * (.9 + .1*c.Happiness/100)
}

func camelJumpSpeed(p *player) float64 {
	if c := mountedCamel(p); c != nil {
		camelTraits(c)
		return c.Jump
	}
	return jumpSpeed
}

type CamelTreat struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Price      int     `json:"price"`
	Stat       string  `json:"stat"`
	Gain       float64 `json:"gain"`
	Cap        float64 `json:"cap"`
	Multiplier float64 `json:"multiplier"`
	Work       float64 `json:"work"`
	MaxWork    float64 `json:"maxWork"`
}

var camelTreats = []CamelTreat{
	{"swift_dates", "Swift dates", 30, "speed", 0, 16, 2, 120, 240},
	{"spring_carrot", "Spring carrot", 25, "jump", 0, 10, 2, 120, 240},
	{"stamina_oats", "Stamina oats", 25, "stamina", 0, 200, 2, 120, 240},
	{"happy_apple", "Happy apple", 10, "happiness", 0, 100, 2, 120, 240},
}

func camelTreatFor(item string) (CamelTreat, bool) {
	for _, treat := range camelTreats {
		if treat.ID == item {
			return treat, true
		}
	}
	return CamelTreat{}, false
}

func (w *World) camelTreatAction(p *player, a Action) error {
	treat, ok := camelTreatFor(a.Item)
	if !ok {
		return errors.New("Choose a camel treat from the pet store.")
	}
	if a.Action == "buy_camel_treat" {
		if _, err := w.requireTownNode(p, a.Target, "pet_shop", 5); err != nil {
			return err
		}
		if p.profile.Coins < treat.Price*a.Amount {
			return errors.New("You do not have enough coins for those treats.")
		}
		if !gain(p, treat.ID, a.Amount) {
			return errors.New("Make room in your bag for the treats first.")
		}
		p.profile.Coins -= treat.Price * a.Amount
		return nil
	}
	if a.Action != "feed_camel" {
		return errors.New("Choose a treat to buy or feed to your camel.")
	}
	if p.island == CamelRaceIsland {
		return errors.New("Save treats for after the race.")
	}
	w.expireCamels(p)
	var camel *Camel
	for i := range p.profile.Camels {
		if p.profile.Camels[i].ID == a.Target {
			camel = &p.profile.Camels[i]
			break
		}
	}
	if camel == nil {
		return errors.New("Choose one of your own camels.")
	}
	training := p.island == CamelTrainingIsland && p.training != nil && p.training.camelID == camel.ID
	if !training && (camel.Island != p.island || camel.InCave != p.inCave || math.Abs(camel.Y-groundY(p)) > 2 || distance(camel.X, camel.Z, p.x, p.z) > 5) {
		return errors.New("Come within five metres of your camel to give it a treat.")
	}
	if !training && camel.ID != p.mountedCamelID && !w.reachableDrop(p, camel.X, camel.Y, camel.Z) {
		return errors.New("Walk around the obstacle to reach your camel.")
	}
	if p.cooldowns["camel_treat"] > w.time {
		return errors.New("Let your camel finish its treat first.")
	}
	camelTraits(camel)
	stat := camelStat(camel, treat.Stat)
	if stat == nil || *stat >= treat.Cap {
		return errors.New("Your camel has already reached the maximum for that treat.")
	}
	boost := trainingValue(&camel.TrainingBoost, treat.Stat)
	if *boost+treat.Work > treat.MaxWork {
		return errors.New("Train with the existing treats first. Your camel already has plenty of training energy.")
	}
	consumed := false
	if a.From == "hotbar" {
		consumed = heldCamelTreat(p) == treat.ID && takeHotbar(p, treat.ID)
	} else {
		consumed = take(p, treat.ID, 1)
	}
	if !consumed {
		return errors.New("You do not have that treat in your inventory.")
	}
	*boost += treat.Work
	p.cooldowns["camel_treat"] = w.time + 1
	p.cooldowns["camel_eating:"+camel.ID] = w.time + 2
	w.eventActor("Your camel enjoyed "+treat.Name+"! Its next "+treat.Stat+" training earns progress twice as fast.", "camel", p.x, p.z, p.profile.ID)
	return nil
}
