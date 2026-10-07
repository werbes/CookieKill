package game

import (
	"fmt"
	"testing"
	"time"
)

var benchmarkSnapshot Snapshot

// Keep the complete procedural world. Accounts include saved inventories,
// friendships, private islands, and bakery upgrades, as on a running server.
func benchmarkWorld(players int, islandPlayers bool) (*World, []string) {
	profiles := make(map[string]Profile, 256)
	for i := 0; i < 256; i++ {
		id := fmt.Sprintf("bench_%03d", i)
		p := Profile{ID: id, Name: id, CallName: id, FriendCode: fmt.Sprintf("FERN%04d", i), Coins: 500, Inventory: map[string]int{"sugar": 10, "dough": 20, "wood": 25, "stick": 12, "stone": 18, "berry": 8, "nut": 8}, Bakery: i%3 == 0, BakeryLevel: i % 6, Equipment: map[string]bool{"oven": true, "mixer": true, "display": true}, Paint: map[string]string{"walls": "teal"}, Friends: map[string]bool{}, Discovered: map[string]bool{"sugar": true, "berry_cookie": true}, Reputation: map[string]int{}}
		for j := 1; j <= 8; j++ {
			p.Friends[fmt.Sprintf("bench_%03d", (i+j)%256)] = true
		}
		if i%4 == 0 {
			p.HomeIsland = &Island{Owner: id, Visitors: true, Builders: map[string]bool{}, Chest: map[string]int{"wood": 50, "stone": 25}}
			for j := 0; j < 64; j++ {
				p.HomeIsland.Objects = append(p.HomeIsland.Objects, BuildObject{ID: fmt.Sprintf("object_%d", j), Kind: "chair", X: float64(j%8)*3 - 12, Z: float64(j/8)*3 - 12})
			}
		}
		profiles[id] = p
	}
	w := New(profiles)
	w.now = func() time.Time { return time.Unix(2_000_000_000, 0) }
	ids := make([]string, players)
	for i := range ids {
		id := fmt.Sprintf("bench_%03d", i)
		ids[i] = id
		w.Join(id, id)
		p := w.players[id]
		p.x, p.z = 8, 35+float64(i%64)
		p.cooldowns["last_input"] = 1e15
		p.input = Input{X: .1, Z: .1}
		if i < len(w.layout.Plots) {
			p.profile.Bakery, p.profile.BakeryPlot = true, w.layout.Plots[i].ID
		}
		if islandPlayers {
			p.island = fmt.Sprintf("bench_%03d", (i/4)*4)
			p.x, p.z = 0, 20+float64(i%4)
		}
		if len(w.animals) > 0 {
			p.profile.Reputation[w.animals[i%len(w.animals)].Species] = i%3 - 1
		}
	}
	w.rebuildPropertyColliders()
	for i := 0; i < 12; i++ {
		w.eventActor("A private crafting result", "craft", 8, 40, ids[i%len(ids)])
	}
	return w, ids
}

func BenchmarkWorldTick(b *testing.B) {
	for _, players := range []int{1, 16, 64} {
		b.Run(fmt.Sprintf("players=%d", players), func(b *testing.B) {
			w, _ := benchmarkWorld(players, false)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				w.Tick(.05)
			}
		})
	}
}

func BenchmarkWorldSnapshotBatch(b *testing.B) {
	benchmarkSnapshots(b, false)
}

func BenchmarkWorldSnapshotUpdateBatch(b *testing.B) {
	benchmarkSnapshots(b, true)
}

func benchmarkSnapshots(b *testing.B, update bool) {
	for _, islands := range []bool{false, true} {
		for _, players := range []int{1, 16, 64} {
			b.Run(fmt.Sprintf("islands=%t/players=%d", islands, players), func(b *testing.B) {
				w, ids := benchmarkWorld(players, islands)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					for _, id := range ids {
						if update {
							benchmarkSnapshot = w.SnapshotUpdate(id)
						} else {
							benchmarkSnapshot = w.Snapshot(id)
						}
					}
				}
			})
		}
	}
}
