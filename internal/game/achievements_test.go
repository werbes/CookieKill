package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestAchievementGoalsAndPermanentUnlocks(t *testing.T) {
	cases := []struct {
		id            string
		before, after Profile
	}{
		{"chef", Profile{Discovered: map[string]bool{"sugar": false, "unknown": true}}, Profile{Discovered: map[string]bool{"sugar": true}}},
		{"recycler", Profile{Cleaned: 9}, Profile{Cleaned: 10}},
		{"ocean", Profile{Rescues: 2}, Profile{Rescues: 3}},
		{"champion", Profile{Kills: 4}, Profile{Kills: 5}},
		{"builder", Profile{}, Profile{HomeIsland: &Island{}}},
		{"green_guardian", Profile{Cleaned: 99}, Profile{Cleaned: 100}},
		{"marine_warden", Profile{Rescues: 19}, Profile{Rescues: 20}},
		{"rancher", Profile{Camels: []Camel{{}, {}, {RentalUntil: 1}}}, Profile{Camels: []Camel{{}, {}, {}}}},
		{"racer", Profile{}, Profile{CamelCoins: 1}},
		{"trailblazer", Profile{PlaySeconds: 3599.99}, Profile{PlaySeconds: 3600}},
		{"merchant", Profile{Coins: 100000, CoinsEarned: 999}, Profile{CoinsEarned: 1000}},
		{"architect", Profile{HomeIsland: &Island{UpgradeLevel: 2}}, Profile{HomeIsland: &Island{UpgradeLevel: 3}}},
		{"restaurateur", Profile{BakeryStaff: map[string]bool{"chef": true}}, Profile{BakeryStaff: map[string]bool{"chef": true, "cashier": true}}},
		{"artisan", Profile{BakeryLevel: 2}, Profile{BakeryLevel: 3}},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			awardAchievements(&tc.before)
			if tc.before.Achievements[tc.id] || tc.before.Hats[tc.id] {
				t.Fatal("awarded before the goal")
			}
			awardAchievements(&tc.after)
			if !tc.after.Achievements[tc.id] || !tc.after.Hats[tc.id] {
				t.Fatal("goal must award both a permanent achievement and its hat")
			}
			retired := Profile{Achievements: tc.after.Achievements}
			awardAchievements(&retired)
			if !retired.Achievements[tc.id] || !retired.Hats[tc.id] {
				t.Fatal("unlock disappeared when an old condition stopped being true")
			}
		})
	}
	p := Profile{Discovered: map[string]bool{}}
	for i, recipe := range recipes {
		p.Discovered[recipe.ID] = true
		awardAchievements(&p)
		if p.Achievements["master_chef"] != (i == len(recipes)-1) {
			t.Fatal("complete cookbook must require every original recipe")
		}
	}
}

func TestAchievementLegacyMigrationCloneAndSave(t *testing.T) {
	p := Profile{ID: "one", Hats: map[string]bool{"chef": true, "ocean": true, "fake": true}, Achievements: map[string]bool{"merchant": true, "unknown": true, "artisan": false}, Avatar: Avatar{Skin: 2, Shirt: "tee", ShirtColor: "teal", Pants: "trousers", PantsColor: "sand", Hat: "ocean"}, HungAchievements: []string{"chef", "chef", "ocean", "locked", "artisan"}}
	p = sanitize(p)
	if !p.Achievements["chef"] || !p.Achievements["ocean"] || !p.Hats["merchant"] || p.Avatar.Hat != "ocean" {
		t.Fatal("legacy hats or independent earned achievements lost")
	}
	if p.Hats["fake"] || p.Achievements["unknown"] || len(p.HungAchievements) != 2 {
		t.Fatal("invalid or duplicate frames survived normalization")
	}
	copy := cloneProfile(p)
	copy.Achievements["chef"] = false
	copy.HungAchievements[0] = "merchant"
	if !p.Achievements["chef"] || p.HungAchievements[0] != "chef" {
		t.Fatal("profile clone shares achievement storage")
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var restored Profile
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	restored = sanitize(restored)
	if !reflect.DeepEqual(restored.Achievements, p.Achievements) || !reflect.DeepEqual(restored.HungAchievements, p.HungAchievements) {
		t.Fatal("earned and hanging achievements did not survive profile serialization")
	}
}

func TestAchievementHangOwnershipReplaySnapshotsAndHatIndependence(t *testing.T) {
	w, p := businessWorld(t)
	p.profile.Hats["chef"] = true
	awardAchievements(&p.profile)
	mustAct(t, w, Action{Action: "hang_achievement", Item: "chef"})
	for _, action := range []Action{{Action: "hang_achievement", Item: "chef"}, {Action: "hang_achievement", Item: "racer"}, {Action: "hang_achievement", Item: "unknown"}} {
		before := cloneProfile(p.profile)
		if err := w.Act("one", action); err == nil || !reflect.DeepEqual(p.profile, before) {
			t.Fatal("invalid hanging must be rejected atomically")
		}
	}
	avatar := p.profile.Avatar
	avatar.Hat = "chef"
	mustAct(t, w, Action{Action: "customize", Avatar: &avatar})
	avatar.Hat = ""
	mustAct(t, w, Action{Action: "customize", Avatar: &avatar})
	if !p.profile.Achievements["chef"] || hungAchievementCount(&p.profile) != 1 {
		t.Fatal("taking off a hat removed its achievement or frame")
	}
	s := w.Snapshot("one")
	if len(s.AchievementCatalog) != 15 || len(s.HatCatalog) != 15 {
		t.Fatal("catalogs missing")
	}
	found := false
	for i := range s.Nodes {
		if s.Nodes[i].ID == p.profile.BakeryPlot {
			found = reflect.DeepEqual(s.Nodes[i].HungAchievements, []string{"chef"})
			s.Nodes[i].HungAchievements[0] = "mutated"
		}
	}
	if !found || p.profile.HungAchievements[0] != "chef" {
		t.Fatal("wall display snapshot missing or aliases profile")
	}
	s.AchievementCatalog[0].Name = "mutated"
	if w.Snapshot("one").AchievementCatalog[0].Name == "mutated" {
		t.Fatal("catalog snapshot aliases definitions")
	}
	before := cloneProfile(p.profile)
	p.x += 30
	if err := w.Act("one", Action{Action: "unhang_achievement", Item: "chef"}); err == nil || !reflect.DeepEqual(before, p.profile) {
		t.Fatal("remote frame removal allowed")
	}
	nodePosition(w, p, p.profile.BakeryPlot)
	p.inCave = true
	if err := w.Act("one", Action{Action: "unhang_achievement", Item: "chef"}); err == nil {
		t.Fatal("underground frame removal allowed")
	}
	p.inCave = false
	mustAct(t, w, Action{Action: "unhang_achievement", Item: "chef"})
	if len(p.profile.HungAchievements) != 0 || !p.profile.Achievements["chef"] {
		t.Fatal("taking down frame removed permanent achievement")
	}
}

func TestAchievementOriginalPriceAdditiveAndDuplicateSafe(t *testing.T) {
	p := Profile{Achievements: map[string]bool{"chef": true, "ocean": true, "builder": true, "merchant": true}, HungAchievements: []string{"chef", "ocean", "builder", "merchant", "chef", "fake", "racer"}}
	if hungAchievementCount(&p) != 4 || 100+achievementPriceBonus(&p, 100) != 140 || 10+achievementPriceBonus(&p, 10) != 14 {
		t.Fatal("four distinct earned frames must give 1.4x original price without compounding or duplicate stacking")
	}
	if achievementPriceBonus(&p, 2) != 0 || achievementPriceBonus(&p, 3) != 1 || achievementPriceBonus(&p, -1) != 0 {
		t.Fatal("combined bonus rounding is wrong")
	}
	actor := &player{profile: p}
	actor.profile.BakeryLevel = 3
	actor.profile.Equipment = map[string]bool{"display": true}
	without := cloneProfile(actor.profile)
	without.HungAchievements = nil
	if bakerySalePrice(actor, "sugar", 5)-bakerySalePrice(&player{profile: without}, "sugar", 5) != 4 {
		t.Fatal("achievement bonus was compounded with existing bakery upgrades")
	}
}

func TestAchievementPlaytimeAwardOccursDuringPlay(t *testing.T) {
	w, p := testWorld()
	p.profile.PlaySeconds = 3599.9
	w.Tick(.2)
	if !p.profile.Achievements["trailblazer"] || !p.profile.Hats["trailblazer"] {
		t.Fatal("playtime goal waits for reconnect or an unrelated action")
	}
}

func TestAchievementPaintingsPriceManualAndBothCashierSales(t *testing.T) {
	w, p, _ := workforceWorld(t)
	p.profile.Cleaned, p.profile.Rescues = 10, 3
	p.profile.Discovered["sugar"] = true
	for _, id := range []string{"chef", "builder", "recycler", "ocean"} {
		mustAct(t, w, Action{Action: "hang_achievement", Item: id})
	}
	if hungAchievementCount(&p.profile) != 4 {
		t.Fatal("fixture did not hang four distinct earned paintings")
	}
	p.profile.BakeryDisplay["sugar"] = 50
	p.bakery.nextCustomer = w.time + 1000
	w.admitBakeryCustomer(p)
	advance(w, 12)
	manual := p.bakery.customers[0]
	if manual.Phase != "waiting" || manual.Amount != 5 {
		t.Fatal("manual-sale customer did not reach the counter")
	}
	p.x, p.z = manual.x, manual.z+1
	before := p.profile.Coins
	mustAct(t, w, Action{Action: "bakery_serve", Target: manual.ID})
	if p.profile.Coins != before+14 {
		t.Fatal("manual five-cookie sale must pay original10 plus four additive bonuses=14")
	}
	plot, _ := w.bakeryPlot(p)
	p.x, p.z = plot.X, plot.Z
	for _, id := range []string{"cashier", "cashier_2"} {
		mustAct(t, w, Action{Action: "bakery_hire", Item: "cashier"})
		mustAct(t, w, Action{Action: "bakery_staff_break", Target: plot.ID, Item: id, Enabled: true})
	}
	w.admitBakeryCustomer(p)
	w.admitBakeryCustomer(p)
	advance(w, 12)
	if len(p.bakery.customers) != 2 || p.bakery.customers[0].DisplayID == p.bakery.customers[1].DisplayID {
		t.Fatal("cashier-sale customers did not queue at separate displays")
	}
	for _, id := range []string{"cashier", "cashier_2"} {
		mustAct(t, w, Action{Action: "bakery_staff_break", Target: plot.ID, Item: id, Enabled: false})
	}
	before = p.profile.Coins
	advance(w, 12)
	if p.profile.Coins != before+28 || p.profile.BakeryDisplay["sugar"] != 35 {
		t.Fatal("both physical cashiers must each pay14 and consume exactly five cookies")
	}
	for _, id := range []string{"cashier", "cashier_2"} {
		mustAct(t, w, Action{Action: "bakery_staff_break", Target: plot.ID, Item: id, Enabled: true})
	}
	mustAct(t, w, Action{Action: "unhang_achievement", Item: "recycler"})
	w.admitBakeryCustomer(p)
	advance(w, 12)
	manual = p.bakery.customers[0]
	p.x, p.z = manual.x, manual.z+1
	before = p.profile.Coins
	mustAct(t, w, Action{Action: "bakery_serve", Target: manual.ID})
	if p.profile.Coins != before+13 || !p.profile.Achievements["recycler"] {
		t.Fatal("removing one painting must lower the next payment without revoking the achievement")
	}
	w.Join("visitor", "Painting visitor")
	visible := false
	for _, node := range w.Snapshot("visitor").Nodes {
		if node.ID != plot.ID {
			continue
		}
		visible = reflect.DeepEqual(node.HungAchievements, []string{"chef", "builder", "ocean"})
		if len(node.HungAchievements) > 0 {
			node.HungAchievements[0] = "changed by visitor"
		}
	}
	if !visible || p.profile.HungAchievements[0] != "chef" {
		t.Fatal("visitor snapshot omitted paintings or exposed mutable saved state")
	}
	restored := New(w.Profiles()).Profiles()["one"]
	if !reflect.DeepEqual(restored.HungAchievements, p.profile.HungAchievements) || 10+achievementPriceBonus(&restored, 10) != 13 {
		t.Fatal("saved paintings did not preserve the adjusted price after reload")
	}
}
