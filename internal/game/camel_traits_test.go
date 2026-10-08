package game

import (
	"math"
	"reflect"
	"testing"
)

func TestCamelBirthTraitsVaryPersistAndClamp(t *testing.T) {
	a, b := Camel{ID: "one_camel_1"}, Camel{ID: "one_camel_2"}
	camelTraits(&a)
	camelTraits(&b)
	if a.Speed == b.Speed || a.Jump == b.Jump || a.Stamina == b.Stamina || a.Happiness == b.Happiness {
		t.Fatal("camels should have individual traits")
	}
	before := a
	camelTraits(&a)
	if a != before {
		t.Fatal("traits rerolled")
	}
	w, p := testWorld()
	p.profile.Camels = []Camel{a, b}
	normalizeCamels(&p.profile)
	if !reflect.DeepEqual(New(w.Profiles()).Profiles()["one"].Camels, p.profile.Camels) {
		t.Fatal("traits changed on restart")
	}
	a.Speed = math.NaN()
	a.Jump = 999
	a.Stamina = -2
	a.Happiness = math.Inf(1)
	camelTraits(&a)
	if !finite(a.Speed) || a.Jump != 10 || a.Stamina != 90 || !finite(a.Happiness) {
		t.Fatalf("invalid traits not normalized: %+v", a)
	}
}

func TestCamelTreatPurchaseFeedingCapsAndPersistence(t *testing.T) {
	w, p := testWorld()
	p.x, p.z = 198, 153
	p.profile.Coins = 1000
	c := Camel{ID: "one_camel_1", X: 198, Z: 153}
	camelTraits(&c)
	p.profile.Camels = []Camel{c}
	for _, treat := range camelTreats {
		coins := p.profile.Coins
		mustAct(t, w, Action{Action: "buy_camel_treat", Target: "pet_shop", Item: treat.ID, Amount: 2})
		if p.profile.Coins != coins-2*treat.Price || p.profile.Inventory[treat.ID] != 2 {
			t.Fatal("treat purchase incorrect")
		}
		before := p.profile.Camels[0]
		w.time++
		mustAct(t, w, Action{Action: "feed_camel", Target: c.ID, Item: treat.ID, Amount: 20})
		after := p.profile.Camels[0]
		if p.profile.Inventory[treat.ID] != 1 {
			t.Fatal("feeding must consume exactly one")
		}
		change := map[string]float64{"speed": after.Speed - before.Speed, "jump": after.Jump - before.Jump, "stamina": after.Stamina - before.Stamina, "happiness": after.Happiness - before.Happiness}
		for key, n := range change {
			if math.Abs(n) > 1e-8 {
				t.Fatalf("%s changed %s by %v", treat.ID, key, n)
			}
		}
		if *trainingValue(&after.TrainingBoost, treat.Stat) != treat.Work || treat.Multiplier != 2 {
			t.Fatal("treat did not prepare the advertised training boost")
		}
		if err := w.Act("one", Action{Action: "feed_camel", Target: c.ID, Item: treat.ID}); err == nil || p.profile.Inventory[treat.ID] != 1 {
			t.Fatal("feeding cooldown failed")
		}
	}
	w.time++
	p.profile.Camels[0].Happiness = 99
	mustAct(t, w, Action{Action: "feed_camel", Target: c.ID, Item: "happy_apple"})
	if p.profile.Camels[0].Happiness != 99 || p.profile.Camels[0].TrainingBoost.Happiness != 240 {
		t.Fatal("treat directly changed happiness instead of preparing training")
	}
	setItem(p, "happy_apple", 1)
	w.time++
	if err := w.Act("one", Action{Action: "feed_camel", Target: c.ID, Item: "happy_apple"}); err == nil || p.profile.Inventory["happy_apple"] != 1 {
		t.Fatal("treats beyond the training budget must not be wasted")
	}
	p.profile.Camels[0].TrainingBoost.Happiness = 0
	p.profile.Camels[0].Happiness = 100
	if err := w.Act("one", Action{Action: "feed_camel", Target: c.ID, Item: "happy_apple"}); err == nil || p.profile.Inventory["happy_apple"] != 1 {
		t.Fatal("capped-stat treats must not be wasted")
	}
	normalizeCamels(&p.profile)
	if !reflect.DeepEqual(New(w.Profiles()).Profiles()["one"].Camels, p.profile.Camels) {
		t.Fatal("upgrades not persisted")
	}
}

func TestCamelTreatRejectsDistantAndOtherCamelsAndImprovesMovement(t *testing.T) {
	w, p := testWorld()
	p.x, p.z = -35, -35
	c := Camel{ID: "one_camel_1", X: p.x, Z: p.z, TraitsVersion: 1, Speed: 10, Jump: 6, Stamina: 120, Happiness: 50}
	p.profile.Camels = []Camel{c}
	setItem(p, "swift_dates", 4)
	for _, target := range []string{"two_camel_1", "missing"} {
		if err := w.Act("one", Action{Action: "feed_camel", Target: target, Item: "swift_dates"}); err == nil {
			t.Fatal("invalid camel accepted")
		}
	}
	p.profile.Camels[0].X = 20
	if err := w.Act("one", Action{Action: "feed_camel", Target: c.ID, Item: "swift_dates"}); err == nil || p.profile.Inventory["swift_dates"] != 4 {
		t.Fatal("distance not enforced atomically")
	}
	p.profile.Camels[0].X = p.x
	p.ridingCamel, p.mountedCamelID = true, c.ID
	before := w.playerMovementSpeed(p)
	mustAct(t, w, Action{Action: "feed_camel", Target: c.ID, Item: "swift_dates"})
	if w.playerMovementSpeed(p) != before || camelJumpSpeed(p) != 6 || staminaCapacity(p) != 120 {
		t.Fatal("feeding bypassed exercise to improve movement")
	}
	p.profile.Camels[0].Happiness = 100
	if math.Abs(w.playerMovementSpeed(p)-p.profile.Camels[0].Speed) > 1e-8 {
		t.Fatal("happy camel should reach natural speed")
	}
	p.profile.Camels[0].Jump = 8
	p.profile.Camels[0].Stamina = 170
	if camelJumpSpeed(p) != 8 || staminaCapacity(p) != 170 {
		t.Fatal("jump/stamina upgrades ignored")
	}
	coins := p.profile.Coins
	if err := w.Act("one", Action{Action: "buy_camel_treat", Target: "pet_shop", Item: "happy_apple"}); err == nil || p.profile.Coins != coins {
		t.Fatal("remote treat purchase accepted")
	}
}
