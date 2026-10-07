package game

import (
	"reflect"
	"testing"
)

func buildingWorld(t *testing.T) (*World, *player) {
	t.Helper()
	w, p := testWorld()
	buyTestIsland(t, w, p)
	mustAct(t, w, Action{Action: "teleport_home"})
	p.x, p.z = 0, 0
	for _, item := range []string{"wood", "stone", "stick", "berry", "nut", "paint_teal"} {
		setItem(p, item, 100)
	}
	return w, p
}

func addBuilder(t *testing.T, w *World, owner *player) *player {
	t.Helper()
	w.Join("two", "Builder")
	p := w.players["two"]
	owner.profile.Friends[p.profile.ID] = true
	p.profile.Friends[owner.profile.ID] = true
	owner.profile.HomeIsland.Visitors = true
	owner.profile.HomeIsland.Builders[p.profile.ID] = true
	w.setIsland(p, owner.profile.ID)
	p.x, p.z = 0, 0
	if err := w.Act("two", Action{Action: "build_mode", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBuildModeExclusiveControlAndCancel(t *testing.T) {
	w, p := buildingWorld(t)
	before := p.profile.Inventory["wood"]
	if err := w.Act("one", Action{Action: "build", Item: "chair", X: 4}); err == nil {
		t.Fatal("build outside mode accepted")
	}
	if p.profile.Inventory["wood"] != before {
		t.Fatal("rejected build spent materials")
	}
	mustAct(t, w, Action{Action: "build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "build", Item: "chair", X: 4})
	original := p.profile.HomeIsland.Objects[1]
	if err := w.Act("one", Action{Action: "move_build", Target: original.ID, X: 6}); err == nil {
		t.Fatal("move without edit control accepted")
	}
	mustAct(t, w, Action{Action: "edit_build", Target: original.ID})
	addBuilder(t, w, p)
	if err := w.Act("two", Action{Action: "edit_build", Target: original.ID}); err == nil {
		t.Fatal("two players acquired one object")
	}
	if err := w.Act("two", Action{Action: "destroy_build", Target: original.ID}); err == nil {
		t.Fatal("another builder destroyed locked object")
	}
	mustAct(t, w, Action{Action: "cancel_build"})
	if p.profile.HomeIsland.Objects[1] != original {
		t.Fatal("cancel changed original object")
	}
	if err := w.Act("two", Action{Action: "edit_build", Target: original.ID}); err != nil {
		t.Fatal(err)
	}
	w.Leave("two")
	mustAct(t, w, Action{Action: "edit_build", Target: original.ID})
	mustAct(t, w, Action{Action: "build_mode", Enabled: false})
	if p.editingBuildID != "" || p.buildMode {
		t.Fatal("exiting mode retained control")
	}
	if err := w.Act("one", Action{Action: "destroy_build", Target: original.ID}); err == nil {
		t.Fatal("destroy outside mode accepted")
	}
	mustAct(t, w, Action{Action: "build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "edit_build", Target: original.ID})
	mustAct(t, w, Action{Action: "teleport_main"})
	if p.buildMode || p.editingBuildID != "" {
		t.Fatal("teleport retained build mode or lock")
	}
}

func TestSharedChestGuestBuildingDoesNotUsePersonalInventory(t *testing.T) {
	w, owner := buildingWorld(t)
	guest := addBuilder(t, w, owner)
	setItem(guest, "wood", 30)
	setItem(guest, "stone", 20)
	if err := w.Act("two", Action{Action: "build", Item: "wall", X: 4}); err == nil {
		t.Fatal("guest built with personal supplies instead of shared supplies")
	}
	guest.x, guest.z = 4, 20
	for item, amount := range map[string]int{"wood": 4, "stone": 2} {
		if err := w.Act("two", Action{Action: "chest_deposit", Item: item, Amount: amount}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Act("two", Action{Action: "chest_withdraw", Item: "wood"}); err == nil {
		t.Fatal("builder withdrew owner materials")
	}
	guest.x, guest.z = 0, 0
	personal := cloneMap(guest.profile.Inventory)
	if err := w.Act("two", Action{Action: "build", Item: "wall", X: 4}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(guest.profile.Inventory, personal) || len(owner.profile.HomeIsland.Chest) != 0 {
		t.Fatal("guest build did not use exactly the shared materials")
	}
	id := owner.profile.HomeIsland.Objects[1].ID
	if err := w.Act("two", Action{Action: "edit_build", Target: id}); err != nil {
		t.Fatal(err)
	}
	if err := w.Act("two", Action{Action: "pack_build", Target: id}); err == nil {
		t.Fatal("guest packed owner furnishing into personal inventory")
	}
	if err := w.Act("two", Action{Action: "destroy_build", Target: id}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(guest.profile.Inventory, personal) || owner.profile.HomeIsland.Chest["wood"] != 2 || owner.profile.HomeIsland.Chest["stone"] != 1 {
		t.Fatal("guest destruction refund did not return to shared chest")
	}
}

func TestPersonalStoragePrivacyPersistenceAndRemovalSafety(t *testing.T) {
	w, p := buildingWorld(t)
	mustAct(t, w, Action{Action: "build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "build", Item: "storage_chest", X: 4})
	id := p.profile.HomeIsland.Objects[1].ID
	setItem(p, "sugar", 10)
	setItem(p, "protein_drink", 3)
	setItem(p, "build_chair", 2)
	for _, item := range []string{"sugar", "protein_drink", "build_chair"} {
		mustAct(t, w, Action{Action: "storage_deposit", Target: id, Item: item, Amount: 2})
	}
	contents := cloneMap(p.profile.HomeIsland.Storage[id])
	if contents["sugar"] != 2 || contents["protein_drink"] != 2 || contents["build_chair"] != 2 {
		t.Fatal("personal chest rejected general items")
	}
	addBuilder(t, w, p)
	if err := w.Act("two", Action{Action: "storage_withdraw", Target: id, Item: "sugar"}); err == nil {
		t.Fatal("guest opened personal storage")
	}
	if len(w.Snapshot("two").Island.Storage) != 0 {
		t.Fatal("personal contents leaked to guest")
	}
	exported := w.Profiles()
	exported["one"].HomeIsland.Storage[id]["sugar"] = 999
	snapshot := w.Snapshot("one")
	snapshot.Island.Storage[id]["sugar"] = 999
	if p.profile.HomeIsland.Storage[id]["sugar"] != 2 {
		t.Fatal("exported storage aliases live contents")
	}
	mustAct(t, w, Action{Action: "edit_build", Target: id})
	for _, kind := range []string{"pack_build", "destroy_build"} {
		if err := w.Act("one", Action{Action: kind, Target: id}); err == nil {
			t.Fatal("removed nonempty personal chest")
		}
	}
	if !reflect.DeepEqual(p.profile.HomeIsland.Storage[id], contents) {
		t.Fatal("failed removal lost stored items")
	}
	mustAct(t, w, Action{Action: "cancel_build"})
	mustAct(t, w, Action{Action: "storage_withdraw", Target: id, Item: "sugar", Amount: 2})
	if p.profile.Inventory["sugar"] != 10 {
		t.Fatal("stored items were not returned")
	}
	restored := New(w.Profiles()).Profiles()["one"]
	if !reflect.DeepEqual(restored.HomeIsland.Storage, p.profile.HomeIsland.Storage) {
		t.Fatal("personal storage did not survive restart")
	}
}

func TestPackedFurnishingPreservesPaintAndCannotDuplicate(t *testing.T) {
	w, p := buildingWorld(t)
	mustAct(t, w, Action{Action: "build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "build", Item: "storage_chest", X: 4})
	id := p.profile.HomeIsland.Objects[1].ID
	mustAct(t, w, Action{Action: "edit_build", Target: id})
	mustAct(t, w, Action{Action: "paint_build", Target: id, Item: "teal"})
	if p.profile.Inventory["paint_teal"] != 99 {
		t.Fatal("paint was not consumed")
	}
	mustAct(t, w, Action{Action: "pack_build", Target: id})
	item := "build_storage_chest:teal"
	if p.profile.Inventory[item] != 1 || len(p.profile.HomeIsland.Objects) != 1 {
		t.Fatal("packing failed to exchange furnishing for one item")
	}
	if err := w.Act("one", Action{Action: "pack_build", Target: id}); err == nil || p.profile.Inventory[item] != 1 {
		t.Fatal("replayed pack duplicated furnishing")
	}
	if err := w.Act("one", Action{Action: "build", Item: "storage_chest", From: item, X: 200}); err == nil || p.profile.Inventory[item] != 1 {
		t.Fatal("invalid replacement consumed packed item")
	}
	mustAct(t, w, Action{Action: "build", Item: "storage_chest", From: item, X: 6})
	if p.profile.Inventory[item] != 0 || p.profile.HomeIsland.Objects[1].Color != "teal" {
		t.Fatal("replacement lost paint or retained duplicate item")
	}
	if err := w.Act("one", Action{Action: "build", Item: "storage_chest", From: item, X: 8}); err == nil {
		t.Fatal("replayed placement duplicated furnishing")
	}
}

func TestGardenTreesFlowersAndStatefulPacking(t *testing.T) {
	w, p := buildingWorld(t)
	mustAct(t, w, Action{Action: "build_mode", Enabled: true})
	mustAct(t, w, Action{Action: "build", Item: "tree", X: 4})
	mustAct(t, w, Action{Action: "build", Item: "flowers", X: -4})
	mustAct(t, w, Action{Action: "build", Item: "garden_bed", Z: 4})
	id := p.profile.HomeIsland.Objects[3].ID
	mustAct(t, w, Action{Action: "use_build", Target: id, Item: "berry"})
	mustAct(t, w, Action{Action: "edit_build", Target: id})
	if err := w.Act("one", Action{Action: "pack_build", Target: id}); err == nil {
		t.Fatal("packing discarded a growing crop")
	}
	if w.islandBlocked(p, -4, 0) {
		t.Fatal("flowers created an invisible walking wall")
	}
	if !w.islandBlocked(p, 4, 0) {
		t.Fatal("tree trunk has no collision")
	}
	restored := New(w.Profiles()).Profiles()["one"]
	if !reflect.DeepEqual(restored.HomeIsland.Objects, p.profile.HomeIsland.Objects) {
		t.Fatal("garden planting was not persisted")
	}
}

func TestDestructionRefundPartialCapacityIsAtomic(t *testing.T) {
	for _, guest := range []bool{false, true} {
		w, owner := buildingWorld(t)
		p := owner
		owner.profile.HomeIsland.Objects = append(owner.profile.HomeIsland.Objects, BuildObject{ID: "oven", Kind: "oven", X: 4})
		if guest {
			p = addBuilder(t, w, owner)
			owner.profile.HomeIsland.Chest = map[string]int{"wood": inventoryLimit - 2, "stone": inventoryLimit}
		} else {
			for i := range p.profile.BagSlots {
				p.profile.BagSlots[i] = Stack{Item: "sugar", Count: inventoryLimit}
			}
			p.profile.BagSlots[0] = Stack{Item: "wood", Count: inventoryLimit - 2}
			p.profile.BagSlots[1] = Stack{Item: "stone", Count: inventoryLimit}
			for i := range p.profile.Hotbar {
				p.profile.Hotbar[i] = Stack{Item: "sugar", Count: inventoryLimit}
			}
			syncInventory(&p.profile)
			mustAct(t, w, Action{Action: "build_mode", Enabled: true})
		}
		if err := w.Act(p.profile.ID, Action{Action: "edit_build", Target: "oven"}); err != nil {
			t.Fatal(err)
		}
		beforeInventory := cloneMap(p.profile.Inventory)
		beforeChest := cloneMap(owner.profile.HomeIsland.Chest)
		if err := w.Act(p.profile.ID, Action{Action: "destroy_build", Target: "oven"}); err == nil {
			t.Fatal("partial refund capacity accepted")
		}
		if !reflect.DeepEqual(beforeInventory, p.profile.Inventory) || !reflect.DeepEqual(beforeChest, owner.profile.HomeIsland.Chest) || len(owner.profile.HomeIsland.Objects) != 2 {
			t.Fatalf("failed guest=%v refund changed items or original object", guest)
		}
	}
}
