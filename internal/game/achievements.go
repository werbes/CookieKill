package game

import (
	"errors"
	"math"
)

type AchievementDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Hat         string `json:"hat"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
}

type HatDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Achievement string `json:"achievement"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
}

var achievementCatalog = []AchievementDefinition{
	{"chef", "First batch", "Discover your first cookie or cake recipe.", "chef", "♧", "#e7d3a2"},
	{"recycler", "Beach helper", "Trade 10 kg of trash with Peace.", "recycler", "♻", "#789c69"},
	{"ocean", "Ocean friend", "Help three ocean animals.", "ocean", "♆", "#6eacc5"},
	{"champion", "Cookie champion", "Defeat five players.", "champion", "★", "#d3ad4e"},
	{"builder", "A place of your own", "Own a bakery or a home.", "builder", "⌂", "#e0b554"},
	{"master_chef", "Complete cookbook", "Discover all eight original cookie and cake recipes.", "master_chef", "♛", "#b4747c"},
	{"green_guardian", "Shoreline guardian", "Trade 100 kg of trash with Peace.", "green_guardian", "❧", "#548a62"},
	{"marine_warden", "Marine rescuer", "Help 20 ocean animals.", "marine_warden", "⚓", "#547caa"},
	{"rancher", "A growing herd", "Own three permanent camels.", "rancher", "♞", "#bd8b58"},
	{"racer", "Dune Dash medalist", "Earn your first camel coin in a race.", "racer", "⚑", "#c8644d"},
	{"trailblazer", "Time well spent", "Play for one hour in total.", "trailblazer", "◷", "#8b9470"},
	{"merchant", "Honest earnings", "Earn 1,000 coins, excluding gambling payouts.", "merchant", "◆", "#a789c1"},
	{"architect", "Room to grow", "Buy three home island expansions.", "architect", "▥", "#749dac"},
	{"restaurateur", "A working kitchen", "Hire both a chef and a cashier for your bakery.", "restaurateur", "☕", "#986e83"},
	{"artisan", "Local institution", "Upgrade your bakery to level three.", "artisan", "✿", "#c99b67"},
}

var hatNames = []string{"Chef hat", "Recycler cap", "Ocean crown", "Champion crown", "Builder hat", "Master chef toque", "Leaf beret", "Captain's hat", "Rancher's hat", "Racing helmet", "Explorer hat", "Merchant's top hat", "Architect's cap", "Cafe beret", "Baker's boater"}

func achievementFor(id string) (AchievementDefinition, bool) {
	for _, achievement := range achievementCatalog {
		if achievement.ID == id {
			return achievement, true
		}
	}
	return AchievementDefinition{}, false
}

func achievementEarned(p *Profile, id string) bool {
	switch id {
	case "chef", "master_chef":
		count := 0
		for _, recipe := range recipes {
			if p.Discovered[recipe.ID] {
				count++
			}
		}
		return count > 0 && (id == "chef" || count == len(recipes))
	case "recycler":
		return p.Cleaned >= 10
	case "ocean":
		return p.Rescues >= 3
	case "champion":
		return p.Kills >= 5
	case "builder":
		return p.Bakery || p.Home != nil || p.HomeIsland != nil
	case "green_guardian":
		return p.Cleaned >= 100
	case "marine_warden":
		return p.Rescues >= 20
	case "rancher":
		owned := 0
		for _, camel := range p.Camels {
			if camel.RentalUntil == 0 {
				owned++
			}
		}
		return owned >= 3
	case "racer":
		return p.CamelCoins >= 1
	case "trailblazer":
		return p.PlaySeconds >= 3600
	case "merchant":
		return p.CoinsEarned >= 1000
	case "architect":
		return p.HomeIsland != nil && p.HomeIsland.UpgradeLevel >= 3
	case "restaurateur":
		return p.BakeryStaff["chef"] && p.BakeryStaff["cashier"]
	case "artisan":
		return p.BakeryLevel >= 3
	}
	return false
}

// Earned achievements and unlocks are permanent; wearing a hat is independent.
// Existing hat unlocks become earned achievements without rerunning old goals.
func awardAchievements(p *Profile) {
	if p.Achievements == nil {
		p.Achievements = map[string]bool{}
	}
	if p.Hats == nil {
		p.Hats = map[string]bool{}
	}
	for _, achievement := range achievementCatalog {
		if p.Achievements[achievement.ID] || p.Hats[achievement.Hat] || achievementEarned(p, achievement.ID) {
			p.Achievements[achievement.ID], p.Hats[achievement.Hat] = true, true
		}
	}
}

func normalizeAchievements(p *Profile) {
	for id, earned := range p.Achievements {
		if _, ok := achievementFor(id); !ok || !earned {
			delete(p.Achievements, id)
		}
	}
	for id, unlocked := range p.Hats {
		if _, ok := achievementFor(id); !ok || !unlocked {
			delete(p.Hats, id)
		}
	}
	awardAchievements(p)
	seen, hung := map[string]bool{}, []string{}
	for _, id := range p.HungAchievements {
		if _, valid := achievementFor(id); valid && p.Achievements[id] && !seen[id] {
			seen[id] = true
			hung = append(hung, id)
		}
	}
	p.HungAchievements = hung
	if !p.Hats[p.Avatar.Hat] {
		p.Avatar.Hat = ""
	}
}

func hungAchievementCount(p *Profile) int {
	seen := map[string]bool{}
	for _, id := range p.HungAchievements {
		if _, valid := achievementFor(id); valid && p.Achievements[id] {
			seen[id] = true
		}
	}
	return len(seen)
}

// Every distinct earned frame adds 10% of the ORIGINAL order price. Round the
// combined bonus once, so small orders do not lose each individual 10% bonus.
func achievementPriceBonus(p *Profile, originalBase int) int {
	return max(0, originalBase) * hungAchievementCount(p) / 10
}

func (w *World) achievementAction(p *player, a Action) error {
	if err := w.requireBakery(p); err != nil {
		return err
	}
	if p.island != "" || p.inCave || math.Abs(playerY(p)) > 2 {
		return errors.New("Visit your own bakery to hang or remove an achievement.")
	}
	awardAchievements(&p.profile)
	if _, ok := achievementFor(a.Item); !ok || !p.profile.Achievements[a.Item] {
		return errors.New("Earn that achievement before hanging it in your bakery.")
	}
	for i, id := range p.profile.HungAchievements {
		if id != a.Item {
			continue
		}
		if a.Action == "hang_achievement" {
			return errors.New("That achievement is already hanging. Each achievement counts once.")
		}
		if a.Action == "unhang_achievement" {
			p.profile.HungAchievements = append(p.profile.HungAchievements[:i], p.profile.HungAchievements[i+1:]...)
			return nil
		}
	}
	if a.Action != "hang_achievement" {
		return errors.New("That achievement is not hanging in your bakery.")
	}
	p.profile.HungAchievements = append(p.profile.HungAchievements, a.Item)
	return nil
}

func (w *World) decorateAchievementSnapshot(viewer *player, out *Snapshot) {
	out.AchievementCatalog = append([]AchievementDefinition{}, achievementCatalog...)
	out.HatCatalog = make([]HatDefinition, 0, len(achievementCatalog))
	for i, achievement := range achievementCatalog {
		out.HatCatalog = append(out.HatCatalog, HatDefinition{achievement.Hat, hatNames[i], achievement.ID, achievement.Icon, achievement.Color})
	}
	if viewer == nil || viewer.island != "" {
		return
	}
	for i := range out.Nodes {
		n := &out.Nodes[i]
		if n.Kind != "bakery_plot" || n.Owner == "" {
			continue
		}
		if p := w.players[n.Owner]; p != nil {
			n.HungAchievements = append([]string{}, p.profile.HungAchievements...)
		}
	}
}
