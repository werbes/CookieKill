package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestTownBouquetVarietiesBuyGiftAndPersistExactType(t *testing.T) {
	for _, bouquet := range townBouquets {
		t.Run(bouquet.ID, func(t *testing.T) {
			w, p := townWorld()
			w.Join("friend", "Mia")
			q := w.players["friend"]
			q.x, q.z = p.x+1, p.z
			mustAct(t, w, Action{Action: "buy_bouquet", Target: "flower_shop", Item: bouquet.ID, Amount: 2})
			if p.profile.Coins != 100-2*bouquet.Price || p.profile.Inventory[bouquet.ID] != 2 {
				t.Fatal("wrong variety price or inventory")
			}
			mustAct(t, w, Action{Action: "give_bouquet", Target: "friend", Item: bouquet.ID})
			if p.profile.Inventory[bouquet.ID] != 1 || q.profile.Inventory[bouquet.ID] != 1 {
				t.Fatal("gift changed bouquet type")
			}
			for _, other := range townBouquets {
				if other.ID != bouquet.ID && q.profile.Inventory[other.ID] != 0 {
					t.Fatal("gift created a different bouquet")
				}
			}
			encoded, _ := json.Marshal(p.profile)
			var decoded Profile
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if sanitize(decoded).Inventory[bouquet.ID] != 1 {
				t.Fatal("bouquet did not survive save normalization")
			}
			beforeP, beforeQ := cloneProfile(p.profile), cloneProfile(q.profile)
			if err := w.Act("one", Action{Action: "give_bouquet", Target: "friend", Item: "cake"}); err == nil || !reflect.DeepEqual(beforeP, cloneProfile(p.profile)) || !reflect.DeepEqual(beforeQ, cloneProfile(q.profile)) {
				t.Fatal("unknown gift was not atomic")
			}
			fillTownBag(q)
			beforeP, beforeQ = cloneProfile(p.profile), cloneProfile(q.profile)
			if err := w.Act("one", Action{Action: "give_bouquet", Target: "friend", Item: bouquet.ID}); err == nil || !reflect.DeepEqual(beforeP, cloneProfile(p.profile)) || !reflect.DeepEqual(beforeQ, cloneProfile(q.profile)) {
				t.Fatal("full receiver lost or duplicated a gift")
			}
		})
	}
}

func TestTownBouquetCatalogIsCopiedAndLegacyPurchaseDefaults(t *testing.T) {
	w, p := townWorld()
	mustAct(t, w, Action{Action: "buy_bouquet", Target: "flower_shop"})
	if p.profile.Inventory["flower_bouquet"] != 1 || p.profile.Coins != 80 {
		t.Fatal("legacy request default changed")
	}
	s := w.Snapshot("one")
	if len(s.Town.Bouquets) != 4 || s.Town.Bouquet.ID != "flower_bouquet" {
		t.Fatal("bouquet catalogs missing")
	}
	s.Town.Bouquets[0].Price = 999
	if w.Snapshot("one").Town.Bouquets[0].Price != 20 {
		t.Fatal("snapshot aliases bouquet catalog")
	}
}
