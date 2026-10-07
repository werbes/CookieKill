package game

import (
	"fmt"
	"math/rand/v2"
)

const (
	fallTravelDistance = 50.0
	fallRecoveryTime   = 2.0
)

// Rolls are in thousandths: 0.1% initially, plus one percentage point per
// alcoholic drink. Runtime state survives reconnects but a respawn clears it.
func fallOdds(p *player) int { return min(1000, 1+10*p.alcoholDrinks) }

func resetFall(p *player) {
	p.fallen, p.standAt, p.fallDistance, p.alcoholDrinks = false, 0, 0, 0
}

func (w *World) stand(p *player) error {
	if !p.fallen {
		return nil
	}
	if p.standAt > w.time {
		return fmt.Errorf("Rest for %.1f more seconds before standing up.", p.standAt-w.time)
	}
	p.fallen, p.posture = false, ""
	p.jumpQueued = false // Space stands up without also launching a jump.
	return nil
}

// Only the collision-resolved grounded movement path calls this function.
// Teleports, sitting, swimming, riding, and airborne travel never contribute.
func (w *World) recordFootTravel(p *player, metres float64) {
	if !finite(metres) || metres <= 0 || p.fallen {
		return
	}
	p.fallDistance += metres
	for p.fallDistance+1e-9 >= fallTravelDistance {
		p.fallDistance = max(0, p.fallDistance-fallTravelDistance)
		draw := w.fallRandom
		if draw == nil {
			draw = func() int { return rand.IntN(1000) }
		}
		if draw() < fallOdds(p) {
			w.releaseBakerySeat(p)
			resetMovement(p)
			p.fallen, p.standAt, p.posture = true, w.time+fallRecoveryTime, "lying"
			p.buildMode, p.editingBuildID = false, ""
			p.bakeryBuildMode, p.editingBakeryFurnitureID = false, ""
			w.eventActor(p.profile.Name+" stumbled and fell over.", "fall", p.x, p.z, p.profile.ID)
			return
		}
	}
}
