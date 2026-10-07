package game

import (
	"strings"
	"testing"
)

func TestBakeryNamePersistsAndAppearsOnClaimedPlot(t *testing.T) {
	w, p := testWorld()
	p.profile.Coins = 250
	nodePosition(w, p, "bakery_plot_0")
	mustAct(t, w, Action{Action: "buy_land", Target: "bakery_plot_0"})
	mustAct(t, w, Action{Action: "bakery_rename", Name: "  Crumbs  & Co.  "})
	if p.profile.BakeryName != "Crumbs & Co." || p.profile.Coins != 0 {
		t.Fatal("bakery name must be trimmed and saved without a coin charge")
	}
	for _, n := range w.Snapshot("one").Nodes {
		if n.ID == "bakery_plot_0" && n.Label != "Crumbs & Co." {
			t.Fatal("bakery sign does not show its chosen name")
		}
	}
	restored := New(w.Profiles())
	restored.Join("one", "Baker")
	rp := restored.players["one"]
	nodePosition(restored, rp, "bakery_plot_1")
	mustAct(t, restored, Action{Action: "buy_land", Target: "bakery_plot_1"})
	if rp.profile.BakeryName != "Crumbs & Co." {
		t.Fatal("bakery name was lost when reopening on another plot")
	}
	rp.x, rp.z = -35, -35
	if err := restored.Act("one", Action{Action: "bakery_rename", Name: "Remote"}); err == nil {
		t.Fatal("bakery naming should use the same proximity rule as painting")
	}
}

func TestCamelNamesArePerCamelOwnedAndPersisted(t *testing.T) {
	w, p := testWorld()
	atCamelKeeper(p)
	p.profile.Coins = 700
	mustAct(t, w, Action{Action: "camel_buy"})
	mustAct(t, w, Action{Action: "camel_buy"})
	first, second := p.profile.Camels[0].ID, p.profile.Camels[1].ID
	p.x, p.z = -35, -35 // The Friends panel can rename a camel from anywhere.
	mustAct(t, w, Action{Action: "camel_rename", Target: first, Name: "Sandy"})
	mustAct(t, w, Action{Action: "camel_rename", Target: second, Name: "Åse"})
	w.Join("two", "Other")
	if err := w.Act("two", Action{Action: "camel_rename", Target: first, Name: "Stolen"}); err == nil {
		t.Fatal("renaming another player's camel succeeded")
	}
	for _, name := range []string{" ", strings.Repeat("x", 25), "Bad\nname", "Hidden\u200bname"} {
		if err := w.Act("one", Action{Action: "camel_rename", Target: first, Name: name}); err == nil {
			t.Fatalf("invalid name accepted: %q", name)
		}
	}
	profile := New(w.Profiles()).Profiles()["one"]
	if profile.Camels[0].Name != "Sandy" || profile.Camels[1].Name != "Åse" {
		t.Fatal("individual camel names were not preserved")
	}
	for _, n := range w.camelNodes(p) {
		if n.ID == first && n.Label != "Sandy" {
			t.Fatal("parked camel label does not use its name")
		}
	}
}
