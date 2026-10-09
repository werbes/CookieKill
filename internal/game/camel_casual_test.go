package game

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func hostCasualRace(t *testing.T, w *World, p *player) *camelRace {
	t.Helper()
	if err := w.Act(p.profile.ID, Action{Action: "race_casual_host"}); err != nil {
		t.Fatal(err)
	}
	r := w.camelRaceForPlayer(p)
	if r == nil || !r.casual || r.hostID != p.profile.ID || !strings.HasPrefix(p.island, camelCasualPrefix) {
		t.Fatal("hosting did not create an isolated friendly lobby")
	}
	return r
}

func TestCamelCasualLobbyActionsConcurrentIsolationAndHostTransfer(t *testing.T) {
	w, p := testWorld()
	w.time = 500
	p.x, p.z = 25, 35
	w.Join("two", "Second")
	w.Join("three", "Third")
	w.Join("four", "Fourth")
	r := hostCasualRace(t, w, p)
	other := hostCasualRace(t, w, w.players["three"])
	if r.island == other.island || len(w.casualRaces) != 2 || w.camelRace != nil {
		t.Fatal("casual instances interfered with one another or the schedule")
	}
	if err := w.Act("two", Action{Action: "race_casual_join", Target: casualRaceID(r)}); err != nil {
		t.Fatal(err)
	}
	if err := w.Act("two", Action{Action: "race_casual_start"}); err == nil {
		t.Fatal("non-host started the race")
	}
	for _, action := range []string{"teleport_main", "camel_dismount", "throw", "eat", "race_casual_host", "race_enter"} {
		if err := w.Act("one", Action{Action: action, Item: "sugar", Target: "camel_race_lollipop"}); err == nil {
			t.Fatalf("friendly race allowed action %s", action)
		}
	}
	x, z := p.x, p.z
	w.Input("one", Input{X: -1, Z: 1, Jump: true, Sprint: true})
	w.Tick(1)
	if p.x != x || p.z != z || !grounded(p) {
		t.Fatal("waiting lobby did not freeze its grid")
	}
	s := w.Snapshot("one")
	if s.CamelRace.Mode != "casual" || s.CamelRace.Phase != "lobby" || s.CamelRace.HostID != "one" || s.CamelRace.EndsAt != 0 || len(s.CamelRace.CasualLobbies) != 2 || s.Island != nil || len(s.Nodes) != 0 {
		t.Fatalf("bad lobby snapshot: %+v", s.CamelRace)
	}
	for _, peer := range s.Players {
		if peer.ID != "one" && peer.ID != "two" {
			t.Fatal("a separate lobby or mainland player leaked into this race")
		}
	}
	s.CamelRace.CasualLobbies[0].HostName = "changed"
	s.CamelRace.OwnCamel.Speed = 999
	if w.Snapshot("one").CamelRace.CasualLobbies[0].HostName == "changed" || p.raceCamel.Speed == 999 {
		t.Fatal("snapshot aliases live lobby or camel data")
	}
	mustAct(t, w, Action{Action: "race_leave"})
	if p.island != "" || p.x != 25 || p.z != 35 || r.hostID != "two" || r.entrantCount != 1 {
		t.Fatal("host departure did not restore origin and transfer hosting")
	}
	if err := w.Act("two", Action{Action: "race_casual_start"}); err != nil {
		t.Fatal(err)
	}
	if r.startAt != w.time+10 || w.Snapshot("two").CamelRace.Phase != "gathering" {
		t.Fatal("new host did not begin the ten-second countdown")
	}
	if err := w.Act("four", Action{Action: "race_casual_join", Target: casualRaceID(r)}); err == nil {
		t.Fatal("late join changed the frozen field")
	}
	if err := w.Act("two", Action{Action: "race_casual_start"}); err == nil {
		t.Fatal("repeated start reset countdown")
	}
	w.Leave("two")
	if len(w.casualRaces) != 1 || w.casualRaces[other.island] != other || w.offline["two"].island != "" {
		t.Fatal("logout did not clean only its own race and restore the player")
	}
}

func TestCamelCasualActualThreeLapsNoRewardsAndAutomaticReturn(t *testing.T) {
	w, p := testWorld()
	w.time = 500
	p.x, p.z = 23, 34
	p.profile.Camels = []Camel{{ID: "one_camel_1", X: 23, Z: 34, Speed: 15, Jump: 9, Stamina: 180, Happiness: 100, TraitsVersion: 1}}
	p.ridingCamel, p.mountedCamelID, p.stamina = true, "one_camel_1", 72
	syncMountedCamel(p)
	before := cloneProfile(p.profile)
	r := hostCasualRace(t, w, p)
	mustAct(t, w, Action{Action: "race_casual_start"})
	entry := r.entries["one"]
	w.time = r.startAt
	for entry.Status == "racing" && w.time < r.startAt+200 {
		raceSteer(w, p, true, true)
		w.Tick(.05)
	}
	if entry.Status != "finished" || entry.Lap != 3 || entry.checkpoints != 48 || entry.Rank != 1 || entry.Coins != 0 || entry.CamelCoins != 0 {
		t.Fatalf("incorrect casual result: %+v", entry.CamelRaceEntryView)
	}
	if p.profile.Coins != before.Coins || p.profile.CamelCoins != before.CamelCoins || p.profile.CoinsEarned != before.CoinsEarned || !reflect.DeepEqual(p.profile.Inventory, before.Inventory) || !reflect.DeepEqual(p.profile.Camels, before.Camels) {
		t.Fatal("friendly completion changed persistent rewards, goods, or camel")
	}
	w.finishCamelRacer(p, entry, w.time+1)
	if r.finished != 1 {
		t.Fatal("duplicate finish was counted")
	}
	w.time += 21
	w.tickCamelRace(0)
	if p.island != "" || p.x != 23 || p.z != 34 || !p.ridingCamel || p.stamina != 72 || p.raceCamel != nil || len(w.casualRaces) != 0 || !reflect.DeepEqual(p.profile.Camels, before.Camels) {
		t.Fatal("automatic return lost origin, owned mount, stamina, or lobby cleanup")
	}
}

func TestCamelCasualScheduledGatherPreemptsOnlyRegisteredPlayer(t *testing.T) {
	w, p := testWorld()
	p.x, p.z = -227, 20
	mustAct(t, w, Action{Action: "race_enter", Target: "camel_race_lollipop"})
	r := hostCasualRace(t, w, p)
	w.Join("two", "Companion")
	if err := w.Act("two", Action{Action: "race_casual_join", Target: casualRaceID(r)}); err != nil {
		t.Fatal(err)
	}
	w.time = 100
	w.tickCamelRace(0)
	if p.island != CamelRaceIsland || w.camelRace.entrantCount != 1 || r.hostID != "two" || w.players["two"].island != r.island || r.entrantCount != 1 {
		t.Fatal("scheduled gathering interrupted the wrong casual participants")
	}
	if err := w.Act("two", Action{Action: "race_casual_start"}); err != nil {
		t.Fatal(err)
	}
	second := w.players["two"]
	z1, z2 := p.z, second.z
	w.time = 120
	w.Input("one", Input{Z: 1})
	w.Input("two", Input{Z: 1})
	w.Tick(.1)
	if p.z <= z1 || second.z <= z2 || w.Snapshot("one").CamelRace.Mode != "scheduled" || w.Snapshot("two").CamelRace.Mode != "casual" {
		t.Fatal("scheduled and friendly races did not advance independently")
	}
	mustAct(t, w, Action{Action: "race_leave"})
	if p.island != "" || p.x != -227 || p.z != 20 {
		t.Fatal("nested race transition lost the actual return location")
	}
}

func TestCamelCasualHomeReturnDisconnectTimeoutAndUnavailableHome(t *testing.T) {
	for _, leave := range []string{"leave", "disconnect", "timeout", "private"} {
		t.Run(leave, func(t *testing.T) {
			w, p := testWorld()
			w.time = 500
			w.Join("host", "Island host")
			w.players["host"].profile.HomeIsland = &Island{Owner: "host", Visitors: true}
			p.island, p.x, p.z = "host", 9, 11
			r := hostCasualRace(t, w, p)
			switch leave {
			case "leave":
				mustAct(t, w, Action{Action: "race_leave"})
			case "disconnect":
				w.SetConnected("one", false)
				w.tickCamelRace(0)
			case "timeout":
				mustAct(t, w, Action{Action: "race_casual_start"})
				w.time = r.startAt + CamelRaceTimeout
				w.tickCamelRace(0)
			case "private":
				w.players["host"].profile.HomeIsland.Visitors = false
				mustAct(t, w, Action{Action: "race_leave"})
				if p.island != "" || p.x != -227 || p.z != 20 {
					t.Fatal("return entered a home whose access was revoked")
				}
				return
			}
			if p.island != "host" || p.x != 9 || p.z != 11 || p.ridingCamel || p.raceCamel != nil || len(w.casualRaces) != 0 {
				t.Fatal("return did not restore the original visited home or remove the loan")
			}
		})
	}
}

func TestCamelRaceRemindersEveryCycleLateJoinAndAllWorlds(t *testing.T) {
	w, p := testWorld()
	if p.raceReminderStart != 120 || !strings.Contains(p.raceReminder.Text, "2 minutes") {
		t.Fatal("first scheduled race did not notify the joining player")
	}
	first := p.raceReminder.ID
	w.tickCamelRace(0)
	w.time = 90
	w.tickCamelRace(0)
	if p.raceReminder.ID != first {
		t.Fatal("first cycle emitted duplicate reminders")
	}
	w.Join("late", "Late arrival")
	if !strings.Contains(w.players["late"].raceReminder.Text, "30 seconds") {
		t.Fatal("late join did not receive the remaining countdown")
	}
	w.time = 110
	w.Join("grid", "Grid arrival")
	if !strings.Contains(w.players["grid"].raceReminder.Text, "grid has gathered") {
		t.Fatal("late grid notification promised impossible entry")
	}
	w.time = 1079.99
	w.tickCamelRace(0)
	if p.raceReminder.ID != first {
		t.Fatal("next reminder fired before two-minute boundary")
	}
	p.island = CookiePracticeIsland
	w.time = 1080
	w.tickCamelRace(0)
	second := p.raceReminder.ID
	if second == first || p.raceReminderStart != 1200 {
		t.Fatal("second scheduled race did not notify an isolated-world player")
	}
	// More recipients than the global event ring can hold must all receive theirs.
	for i := 0; i < 80; i++ {
		w.Join(fmt.Sprintf("crowd_%d", i), "Racer")
	}
	for _, entrant := range w.players {
		found := 0
		for _, event := range w.Snapshot(entrant.profile.ID).Events {
			if event.ID == entrant.raceReminder.ID {
				found++
			}
		}
		if found != 1 {
			t.Fatalf("%s received %d copies of reminder", entrant.profile.ID, found)
		}
	}
	// World transitions do not discard or duplicate a still-live personal event.
	p.island = "one"
	w.tickCamelRace(0)
	if p.raceReminder.ID != second || w.Snapshot("one").CamelRace.ReminderSeconds != 120 {
		t.Fatal("world transition changed reminder identity or countdown")
	}
	w.time = 1200
	w.tickCamelRace(0)
	if a, b := camelRaceReminder(w.time); a != 0 || b != 0 {
		t.Fatal("start instant retained an expired two-minute reminder")
	}
	w.time = 2160
	w.tickCamelRace(0)
	if p.raceReminderStart != 2280 || p.raceReminder.ID == second {
		t.Fatal("third scheduled race reminder missing")
	}
}

func TestCamelRaceReminderDisconnectedAndReconnectedSessions(t *testing.T) {
	w, p := testWorld()
	w.time = 500
	w.Leave("one")
	w.time = 1080
	w.tickCamelRace(0)
	if p.raceReminderStart != 120 {
		t.Fatal("offline account received an active reminder")
	}
	w.time = 1140
	w.Join("one", "Returned")
	if p.raceReminderStart != 1200 || !strings.Contains(p.raceReminder.Text, "60 seconds") {
		t.Fatal("reconnected player did not receive the current countdown")
	}
	reminder := p.raceReminder.ID
	w.Join("one", "Returned again")
	w.tickCamelRace(0)
	if p.raceReminder.ID != reminder {
		t.Fatal("repeated login emitted a second reminder in the same cycle")
	}
}

func TestCamelRaceEntryClearsOnlyEntrantsProjectiles(t *testing.T) {
	for _, mode := range []string{"casual", "scheduled"} {
		t.Run(mode, func(t *testing.T) {
			w, p := testWorld()
			p.x, p.z = -227, 20
			w.projectiles = []*Projectile{{ID: "entrant", Owner: "one"}, {ID: "other", Owner: "two"}}
			if mode == "casual" {
				hostCasualRace(t, w, p)
			} else {
				mustAct(t, w, Action{Action: "race_enter", Target: "camel_race_lollipop"})
				w.time = 100
				w.tickCamelRace(0)
			}
			if len(w.projectiles) != 1 || w.projectiles[0].ID != "other" {
				t.Fatal("race entry retained the entrant's old-world projectile or removed someone else's")
			}
		})
	}
}
