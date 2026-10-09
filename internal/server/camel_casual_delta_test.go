package server

import (
	"bytes"
	"encoding/json"
	"testing"

	"cookiekill/internal/game"
)

func TestCasualRaceWorldSnapshotsAndDeltaBaselinesStayDetached(t *testing.T) {
	w := game.New(nil)
	for _, id := range []string{"host", "guest", "other", "observer"} {
		w.Join(id, id)
	}
	encoders := map[string]*stateEncoder{}
	receivers := map[string]*testDeltaReceiver{}
	for _, id := range []string{"host", "observer"} {
		encoders[id] = &stateEncoder{delta: true}
		receivers[id] = &testDeltaReceiver{}
	}
	check := func(id string) game.Snapshot {
		t.Helper()
		want := w.SnapshotUpdate(id)
		got := receivers[id].apply(t, encodeDeltaTest(t, encoders[id], want))
		assertDeltaSnapshot(t, got, want)
		encoders[id].commit(want)
		return got
	}
	act := func(id, action, target string) {
		t.Helper()
		if err := w.Act(id, game.Action{Action: action, Target: target}); err != nil {
			t.Fatalf("%s %s: %v", id, action, err)
		}
	}
	check("host")
	initial := check("observer")
	if len(initial.CamelRace.CasualLobbies) != 0 || initial.CamelRace.ReminderStartAt != 120 || initial.CamelRace.ReminderSeconds != 120 {
		t.Fatal("initial snapshot omitted the empty lobby list or two-minute reminder")
	}
	act("host", "race_casual_host", "")
	host := check("host")
	observer := check("observer")
	lobbyID := host.CamelRace.LobbyID
	if lobbyID == "" || host.CamelRace.Mode != "casual" || host.CamelRace.Phase != "lobby" || observer.CamelRace.Mode != "scheduled" || len(observer.CamelRace.CasualLobbies) != 1 {
		t.Fatal("host and public lobby views were not distinct")
	}
	retained := encoders["host"].previous
	baseline := marshalDeltaTest(t, retained.CamelRace)
	detached := w.SnapshotUpdate("host")
	detached.CamelRace.CasualLobbies[0].HostName = "tampered lobby"
	detached.CamelRace.Entrants[0].Name = "tampered entrant"
	detached.CamelRace.OwnCamel.Speed = 999
	fresh := w.SnapshotUpdate("host")
	if !bytes.Equal(baseline, marshalDeltaTest(t, fresh.CamelRace)) || !bytes.Equal(baseline, marshalDeltaTest(t, encoders["host"].previous.CamelRace)) {
		t.Fatal("a later snapshot mutated the world or committed race baseline")
	}
	var quiet map[string]json.RawMessage
	decodeDeltaTest(t, encodeDeltaTest(t, encoders["host"], fresh), &quiet)
	if _, changed := quiet["camelRace"]; changed {
		t.Fatal("unchanged race was resent after mutation of an unrelated snapshot")
	}

	act("guest", "race_casual_join", lobbyID)
	act("other", "race_casual_host", "")
	host = check("host")
	observer = check("observer")
	if len(host.Players) != 1 || host.Players[0].ID != "guest" || len(host.CamelRace.Entrants) != 2 || len(observer.CamelRace.CasualLobbies) != 2 {
		t.Fatal("wire snapshot mixed independent race participants or missed a lobby")
	}
	if len(initial.CamelRace.CasualLobbies) != 0 || !bytes.Equal(baseline, marshalDeltaTest(t, retained.CamelRace)) {
		t.Fatal("earlier snapshot changed as players joined")
	}
	// The committed snapshot has since advanced; the previously returned public
	// view must still describe the original one-person lobby.
	if hostBefore := detached.CamelRace; hostBefore.CasualLobbies[0].EntrantCount != 1 || len(hostBefore.Entrants) != 1 {
		t.Fatal("later lobby joins mutated an earlier snapshot")
	}
	act("host", "race_casual_start", "")
	started := check("host")
	check("observer")
	if started.CamelRace.Phase != "gathering" || started.CamelRace.StartAt-started.Time != 10 {
		t.Fatal("countdown was not transmitted")
	}
	act("host", "race_leave", "")
	returned := check("host")
	check("observer")
	if returned.CamelRace.Mode != "scheduled" || returned.CamelRace.InArena || returned.CamelRace.LobbyID != "" || returned.CamelRace.HostID != "" || returned.CamelRace.OwnCamel != nil || len(returned.CamelRace.Entrants) != 0 {
		t.Fatal("return delta retained the previous race, host, or loan camel")
	}
	act("guest", "race_leave", "")
	check("host")
	remaining := check("observer")
	if len(remaining.CamelRace.CasualLobbies) != 1 || remaining.CamelRace.CasualLobbies[0].HostID != "other" {
		t.Fatal("leaving one lobby removed another lobby or retained the empty one")
	}
	act("other", "race_leave", "")
	check("host")
	empty := check("observer")
	if empty.CamelRace.CasualLobbies == nil || len(empty.CamelRace.CasualLobbies) != 0 {
		t.Fatal("last departure did not replace the lobby list with an empty array")
	}
	for i := 0; i < 121; i++ {
		w.Tick(1)
	}
	afterStart := w.SnapshotUpdate("observer")
	var frame map[string]json.RawMessage
	data := encodeDeltaTest(t, encoders["observer"], afterStart)
	decodeDeltaTest(t, data, &frame)
	var race map[string]json.RawMessage
	decodeDeltaTest(t, frame["camelRace"], &race)
	for _, field := range []string{"reminderStartAt", "reminderSeconds"} {
		if string(race[field]) != "0" {
			t.Fatalf("expired %s was not explicitly reset to zero: %s", field, race[field])
		}
	}
	if string(race["casualLobbies"]) != "[]" {
		t.Fatalf("cleared lobby list = %s, want []", race["casualLobbies"])
	}
	assertDeltaSnapshot(t, receivers["observer"].apply(t, data), afterStart)
}
