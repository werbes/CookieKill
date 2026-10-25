package game

import (
	"math"
	"reflect"
	"testing"
)

func TestLeaveUnloadsRuntimeAndReconnectPreservesDetachedProgress(t *testing.T) {
	w, old := buildingWorld(t)
	old.profile.Coins, old.profile.PlaySeconds = 1234, 42
	old.profile.Levels["legs"] = 3
	old.profile.CallNameChangedAt = 100
	old.profile.Bakery, old.profile.BakeryPlot = true, "bakery_plot_0"
	old.profile.HomeIsland.Objects = append(old.profile.HomeIsland.Objects, BuildObject{ID: "mixer", Kind: "mixer", X: 4})
	productionState(&old.profile, "home:mixer").Job = &ProductionJob{Item: "prepared_sugar", Amount: 2, Remaining: 1, Total: 2}
	old.profile = sanitize(old.profile)
	want := cloneProfile(old.profile)
	want.BakeryPlot = ""

	// These values belong to a session and must never be restored from it,
	// including a corrupted coordinate that could poison the browser view.
	old.x, old.y, old.z = math.NaN(), math.Inf(1), 200
	old.island, old.inCave, old.posture = "abandoned", true, "lying"
	old.health, old.stamina, old.alcoholDrinks = 35, 0, 7
	old.fallen, old.sprintExhausted, old.jumpHeld = true, true, true
	old.standAt, old.jumpVelocity = 1000, 20
	old.buffs["slowed"], old.cooldowns["train"] = 1000, 1000
	old.bakery = &bakeryBusiness{customers: []*bakeryCustomer{{}}}
	old.practice = &practiceSession{station: "practice_bench_press"}
	old.clubSeatID, old.bakerySeatID = "club_seat_1", "bakery_plot_0_seat_1"
	old.buildMode, old.editingBuildID = true, "mixer"
	old.input = Input{X: 1, Jump: true, Sprint: true}
	w.projectiles = []*Projectile{{ID: "departing", Owner: "one"}, {ID: "other", Owner: "other"}}
	w.Leave("one")
	if len(w.players) != 0 || w.plotOwner("bakery_plot_0") != "" || w.SnapshotUpdate("one").Me.ID != "" {
		t.Fatal("logout retained an active player or occupied bakery plot")
	}
	if len(w.projectiles) != 1 || w.projectiles[0].Owner != "other" {
		t.Fatal("logout did not remove only the departing player's projectiles")
	}
	if got := w.Profiles()["one"]; !reflect.DeepEqual(got, want) {
		t.Fatal("unloading lost persistent account progress")
	}
	w.Input("one", Input{X: -1})
	if err := w.Act("one", Action{Action: "eat", Item: "sugar"}); err == nil || old.input != (Input{}) {
		t.Fatal("an unloaded player still accepts input or actions")
	}
	w.Tick(1)
	w.Leave("one") // Duplicate disconnects cannot replace or erase the save.
	if !reflect.DeepEqual(w.Profiles()["one"], want) {
		t.Fatal("offline simulation or duplicate logout changed saved progress")
	}

	old.profile.Inventory["sugar"] = 999
	old.profile.Levels["legs"] = 10
	old.profile.HomeIsland.Objects[0].X = 999
	old.profile.Production["home:mixer"].Job.Remaining = 999
	w.Join("one", "A login must not rename the account")
	p := w.players["one"]
	if p == old || !reflect.DeepEqual(cloneProfile(p.profile), want) {
		t.Fatal("reconnect reused the runtime or aliased discarded profile data")
	}
	if !finite(p.x) || !finite(p.y) || !finite(p.z) || p.island != "" || p.inCave || p.health != 100 || p.stamina != staminaCapacity(p) {
		t.Fatal("reconnect did not create a healthy, finite mainland spawn")
	}
	if p.practice != nil || p.bakery != nil || p.posture != "" || p.clubSeatID != "" || p.bakerySeatID != "" || p.buildMode || p.editingBuildID != "" || p.fallen || p.alcoholDrinks != 0 || p.jumpHeld || p.sprintExhausted || p.input != (Input{}) || p.cooldowns["train"] != 0 || w.active(p, "slowed") {
		t.Fatal("reconnect restored abandoned activity, seat, movement, or effect state")
	}
	if !w.active(p, "spawn_shield") {
		t.Fatal("fresh spawn lacks its normal protection")
	}
}

func TestTransientDisconnectRetainsSessionForResume(t *testing.T) {
	w, player := buildingWorld(t)
	player.x, player.z, player.yaw, player.pitch = 12.5, -8.25, 1.1, -.35
	player.input = Input{X: 1, Jump: true, Sprint: true}
	player.jumpQueued, player.jumpHeld, player.sprintHeld, player.sprinting = true, true, true, true

	w.Disconnect("one")
	if !player.disconnected || player.input != (Input{}) || player.jumpQueued || player.jumpHeld || player.sprintHeld || player.sprinting {
		t.Fatal("transient disconnect retained live controls")
	}
	view := w.Snapshot("one")
	if view.Me.ID != "one" || view.Me.X != 12.5 || view.Me.Z != -8.25 || view.Me.Yaw != 1.1 || view.Me.Pitch != -.35 {
		t.Fatal("transient disconnect lost resumable session state")
	}

	w.Join("one", "Alice")
	if w.players["one"] != player || player.disconnected || player.x != 12.5 || player.z != -8.25 {
		t.Fatal("reconnect did not resume the retained runtime")
	}
}

func TestOfflineOwnerKeepsSharedIslandAndProductionUpdates(t *testing.T) {
	w, owner := buildingWorld(t)
	owner.profile.HomeIsland.Objects = append(owner.profile.HomeIsland.Objects,
		BuildObject{ID: "busy", Kind: "mixer", X: -4},
		BuildObject{ID: "empty", Kind: "mixer", X: 4},
	)
	productionState(&owner.profile, "home:busy").Output["dough"] = 3
	productionState(&owner.profile, "home:empty")
	guest := addBuilder(t, w, owner)
	w.Leave("one")
	guest.x, guest.z = -3, 0
	if err := w.Act("two", Action{Action: "edit_build", Target: "busy"}); err != nil {
		t.Fatal(err)
	}
	if err := w.Act("two", Action{Action: "destroy_build", Target: "busy"}); err == nil {
		t.Fatal("offline owner's paid production was removed by a builder")
	}
	if err := w.Act("two", Action{Action: "cancel_build"}); err != nil {
		t.Fatal(err)
	}
	guest.x = 3
	if err := w.Act("two", Action{Action: "edit_build", Target: "empty"}); err != nil {
		t.Fatal(err)
	}
	if err := w.Act("two", Action{Action: "destroy_build", Target: "empty"}); err != nil {
		t.Fatal(err)
	}
	saved := w.Profiles()["one"]
	if w.players["one"] != nil || len(saved.HomeIsland.Objects) != 2 || saved.Production["home:empty"] != nil || saved.Production["home:busy"].Output["dough"] != 3 {
		t.Fatal("builder update reloaded owner or lost persistent property/production changes")
	}
	w.Join("one", "Returning owner")
	if got := w.players["one"].profile; !reflect.DeepEqual(got.HomeIsland, saved.HomeIsland) || !reflect.DeepEqual(got.Production, saved.Production) {
		t.Fatal("reconnect lost changes made while owner was unloaded")
	}
}
