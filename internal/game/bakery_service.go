package game

import (
	"errors"
	"math"
)

// Serving follows the customer, so the baker need not open the display panel.
// Stock and payment still belong to the owner's display and actual order.
func (w *World) requireBakeryCustomer(p *player, c *bakeryCustomer) error {
	if p.island != "" || p.inCave || p.ridingCamel || p.bakeryBuildMode ||
		math.Abs(playerY(p)) > 2.5 || distance(p.x, p.z, c.x, c.z) > 4 {
		return errors.New("Leave build mode and stand within four metres of your customer to serve them.")
	}
	for _, colliders := range [][]Collider{w.colliders, w.propertyColliders} {
		for _, collider := range colliders {
			if collider.resource != nil && !collider.resource.Available {
				continue
			}
			if t, hit := colliderSegmentHit(collider, p.x, playerY(p)+1.55, p.z, c.x, 1.3, c.z); hit && t < .995 {
				return errors.New("Move where you can see your customer before serving them.")
			}
		}
	}
	return nil
}
