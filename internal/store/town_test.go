package store

import (
	"math"
	"testing"

	"cookiekill/internal/game"
)

func TestTownStatsAndGiftsSurvivePlayerFileRestart(t *testing.T) {
	db, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := game.Profile{ID: "town_player", Name: "Mia", Coins: 150, CoinsEarned: 444, CamelCoins: 90, PlaySeconds: 3725.25, Kills: 7, CookieRingKills: 3, InventoryVersion: 3}
	p.BagSlots[0] = game.Stack{Item: "flower_daisy", Count: 2}
	p.BagSlots[1] = game.Stack{Item: "flower_bouquet", Count: 1}
	if err := db.Save(map[string]game.Profile{p.ID: p}); err != nil {
		t.Fatal(err)
	}
	loaded, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	world := game.New(loaded)
	world.Join(p.ID, p.Name)
	got := world.Snapshot(p.ID)
	if got.Me.CookieRingKills != 3 || got.Me.Kills != 7 || got.Me.CoinsEarned != 444 || got.Me.CamelCoins != 90 || math.Abs(got.Me.PlaySeconds-3725.25) > 1e-9 || got.Me.Inventory["flower_daisy"] != 2 || got.Me.Inventory["flower_bouquet"] != 1 {
		t.Fatalf("town progress lost on restart: %+v", got.Me)
	}
	if len(got.Town.Leaderboards) != 5 || got.Town.Leaderboards[0].Entries[0].Value != 90 {
		t.Fatal("persisted player absent from records")
	}
}
