package server

import (
	"encoding/json"
	"testing"

	"cookiekill/internal/game"
)

func TestTrainingAndAchievementChangesSurviveDeltaReconstruction(t *testing.T) {
	encoder := &stateEncoder{delta: true}
	receiver := &testDeltaReceiver{}
	before := game.Snapshot{
		CamelTraining:      game.CamelTrainingView{Courses: []game.CamelTrainingCourse{{ID: "speed", Checkpoints: []game.Point{{X: -34, Z: 24}}}}},
		AchievementCatalog: []game.AchievementDefinition{{ID: "chef", Name: "First batch", Hat: "chef"}},
		HatCatalog:         []game.HatDefinition{{ID: "chef", Name: "Chef hat", Achievement: "chef"}},
		Nodes:              []game.Node{{ID: "plot", Kind: "bakery_plot"}},
	}
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, before)), before)
	encoder.commit(before)
	// A quiet update must not resend the static catalogs or idle course state.
	var unchanged map[string]json.RawMessage
	if err := json.Unmarshal(encodeDeltaTest(t, encoder, before), &unchanged); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"camelTraining", "achievementCatalog", "hatCatalog"} {
		if _, ok := unchanged[key]; ok {
			t.Fatalf("unchanged %s resent", key)
		}
	}
	after := before
	after.CamelTraining = game.CamelTrainingView{InArena: true, Exercise: "speed", Checkpoint: 2, CompletedLaps: 1, OwnCamel: &game.Camel{ID: "one_camel_1", Speed: 10, Training: game.CamelTrainingProgress{Speed: 50}, TrainingBoost: game.CamelTrainingProgress{Speed: 95}}, Courses: before.CamelTraining.Courses}
	after.Me.Island = game.CamelTrainingIsland
	after.Me.Achievements = map[string]bool{"chef": true}
	after.Me.HungAchievements = []string{"chef"}
	after.Nodes = []game.Node{{ID: "plot", Kind: "bakery_plot", HungAchievements: []string{"chef"}}}
	after.AchievementCatalog = append([]game.AchievementDefinition{}, before.AchievementCatalog...)
	after.AchievementCatalog = append(after.AchievementCatalog, game.AchievementDefinition{ID: "racer", Hat: "racer"})
	after.HatCatalog = append([]game.HatDefinition{}, before.HatCatalog...)
	after.HatCatalog = append(after.HatCatalog, game.HatDefinition{ID: "racer", Achievement: "racer"})
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, after)), after)
	encoder.commit(after)
	finished := after
	finished.CamelTraining = game.CamelTrainingView{Courses: before.CamelTraining.Courses}
	finished.Me.Island = ""
	finished.Me.HungAchievements = []string{}
	finished.Nodes = []game.Node{{ID: "plot", Kind: "bakery_plot"}}
	finished.AchievementCatalog = []game.AchievementDefinition{}
	finished.HatCatalog = nil
	assertDeltaSnapshot(t, receiver.apply(t, encodeDeltaTest(t, encoder, finished)), finished)
}
