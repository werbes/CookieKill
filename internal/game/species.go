package game

import (
	"math"
	"strconv"
	"strings"
)

func reputationSpecies(key string) string {
	for _, spec := range animalSpecs {
		if key == spec.species || strings.HasPrefix(key, "animal_"+spec.species+"_") {
			return spec.species
		}
	}
	// The first two members of each original species used numeric IDs.
	if n, err := strconv.Atoi(strings.TrimPrefix(key, "animal_")); strings.HasPrefix(key, "animal_") && err == nil && n >= 0 && n < 12 {
		return animalSpecs[n%6].species
	}
	return ""
}

// Old individual opinions migrate once to the average for each species. A
// saved species score takes precedence, so repeated loads cannot change it.
func normalizeSpeciesReputation(p *Profile) {
	reputation := map[string]int{}
	sums, counts := map[string]int{}, map[string]int{}
	for key, value := range p.Reputation {
		species := reputationSpecies(key)
		if species == "" {
			continue
		}
		value = max(-10, min(10, value))
		if key == species {
			reputation[species] = value
		} else {
			sums[species] += value
			counts[species]++
		}
	}
	for _, spec := range animalSpecs {
		if _, exists := reputation[spec.species]; !exists && counts[spec.species] > 0 {
			reputation[spec.species] = int(math.Round(float64(sums[spec.species]) / float64(counts[spec.species])))
		}
	}
	p.Reputation = reputation
}

func changeSpeciesReputation(p *player, species string, amount int) {
	p.profile.Reputation[species] = max(-10, min(10, p.profile.Reputation[species]+amount))
}

func (w *World) greetAnimal(p *player, a *Animal) bool {
	key := "ocean_greeting:" + a.Species
	if p.profile.Reputation[a.Species] <= 0 || p.cooldowns[key] > w.time {
		return false
	}
	p.cooldowns[key] = w.time + 60
	visits := "ocean_visits:" + a.Species
	p.cooldowns[visits]++
	w.eventActor("A "+strings.ReplaceAll(a.Species, "_", " ")+" swims up to greet you.", "animal", a.X, a.Z, p.profile.ID)
	// Only some greetings bring a gift. The species shares the timer, so meeting
	// a whole pod cannot turn one greeting into a pile of rewards.
	if int(p.cooldowns[visits])%3 == 0 && p.cooldowns["ocean_gift:"+a.Species] <= w.time {
		items := []string{"shell", "pearl", "treasure"}
		item := items[(int(p.cooldowns[visits])/3-1)%len(items)]
		if gain(p, item, 1) {
			p.cooldowns["ocean_gift:"+a.Species] = w.time + 120
			w.eventActor("Your "+strings.ReplaceAll(a.Species, "_", " ")+" friend brought you a "+item+"!", "animal", a.X, a.Z, p.profile.ID)
		}
	}
	return true
}
