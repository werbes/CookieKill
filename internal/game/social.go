package game

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

type FriendView struct {
	ID         string `json:"id"`
	CallName   string `json:"callName"`
	Username   string `json:"username"`
	FriendCode string `json:"friendCode,omitempty"`
	Online     bool   `json:"online"`
	OwnsIsland bool   `json:"ownsIsland"`
	Visitors   bool   `json:"visitors"`
	CanBuild   bool   `json:"canBuild"`
	Request    bool   `json:"request"`
}

var friendWords = []string{"BIRD", "CAKE", "CLAY", "DUNE", "FERN", "FISH", "FROG", "GLOW", "GOLD", "HOME", "JADE", "KITE", "LAKE", "LEAF", "LIME", "MOON", "MOSS", "NEST", "NUTS", "PALM", "PEAR", "PINE", "PINK", "PLUM", "POND", "RAIN", "REEF", "ROSE", "SAGE", "SAND", "SEED", "STAR", "TIDE", "TREE", "WAVE", "WIND"}

func (w *World) ensureFriendCode(p *Profile) {
	if p.FriendCode != "" {
		return
	}
	used := map[string]bool{}
	for _, other := range w.profileViews() {
		if other.ID != p.ID {
			used[other.FriendCode] = true
		}
	}
	for {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(friendWords)*10000)))
		if err != nil {
			panic("secure randomness unavailable for friend codes")
		}
		v := int(n.Int64())
		code := fmt.Sprintf("%s%04d", friendWords[v/10000], v%10000)
		if !used[code] {
			p.FriendCode = code
			return
		}
	}
}

// Assign codes in sorted order so legacy accounts receive stable persisted IDs.
func (w *World) restoreSocial() {
	ids := make([]string, 0, len(w.profiles))
	for id := range w.profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := w.profiles[id]
		w.ensureFriendCode(&p)
		w.profiles[id] = p
	}
}

func (w *World) profileByID(id string) (Profile, bool) {
	if p := w.players[id]; p != nil {
		return p.profile, true
	}
	if p := w.offline[id]; p != nil {
		return p.profile, true
	}
	p, ok := w.profiles[id]
	return p, ok
}

func (w *World) storeProfile(p Profile) {
	if online := w.players[p.ID]; online != nil {
		online.profile = p
		return
	}
	if offline := w.offline[p.ID]; offline != nil {
		offline.profile = p
	}
	w.profiles[p.ID] = cloneProfile(p)
}

func (w *World) socialAction(p *player, a Action) error {
	switch a.Action {
	case "friend_public":
		p.profile.PublicFriendCode = a.Enabled
		return nil
	case "friend_request":
		code := strings.ToUpper(strings.TrimSpace(a.Target))
		if code == p.profile.FriendCode {
			return errors.New("That is your own friend code.")
		}
		if p.cooldowns["friend_request"] > w.time {
			return errors.New("Wait a moment before sending another friend request.")
		}
		for id, target := range w.profileViews() {
			if target.FriendCode != code {
				continue
			}
			if p.profile.Friends[id] {
				return errors.New("You are already friends.")
			}
			if len(target.FriendRequests) >= 100 {
				return errors.New("This player has too many pending friend requests.")
			}
			if target.FriendRequests[p.profile.ID] {
				return errors.New("Your friend request is already waiting for a reply.")
			}
			target = cloneProfile(target)
			target.FriendRequests[p.profile.ID] = true
			w.storeProfile(target)
			p.cooldowns["friend_request"] = w.time + 2
			w.eventActor(p.profile.CallName+" sent you a friend request.", "notification", 0, 0, id)
			return nil
		}
		return errors.New("No player has that friend code.")
	case "friend_accept", "friend_decline":
		if !p.profile.FriendRequests[a.Target] {
			return errors.New("That friend request is no longer pending.")
		}
		target, ok := w.profileByID(a.Target)
		if !ok {
			return errors.New("This player is no longer available.")
		}
		if a.Action == "friend_accept" {
			if len(p.profile.Friends) >= 100 || len(target.Friends) >= 100 {
				return errors.New("The friends list is full (100 friends).")
			}
			target = cloneProfile(target)
			p.profile.Friends[a.Target] = true
			target.Friends[p.profile.ID] = true
			delete(target.FriendRequests, p.profile.ID)
			w.storeProfile(target)
			w.eventActor(p.profile.CallName+" accepted your friend request.", "notification", 0, 0, target.ID)
		}
		delete(p.profile.FriendRequests, a.Target)
		return nil
	case "friend_remove":
		delete(p.profile.Friends, a.Target)
		if p.profile.HomeIsland != nil {
			delete(p.profile.HomeIsland.Builders, a.Target)
		}
		if target, ok := w.profileByID(a.Target); ok {
			target = cloneProfile(target)
			delete(target.Friends, p.profile.ID)
			if target.HomeIsland != nil {
				delete(target.HomeIsland.Builders, p.profile.ID)
			}
			w.storeProfile(target)
		}
		return nil
	}
	return errors.New("Unknown friend action.")
}

func (w *World) friendViews(p *player) []FriendView {
	result := []FriendView{}
	for id, other := range w.profileViews() {
		if !p.profile.Friends[id] && !p.profile.FriendRequests[id] {
			continue
		}
		f := FriendView{ID: id, CallName: other.CallName, Username: other.Username, Online: w.players[id] != nil, Request: p.profile.FriendRequests[id]}
		if other.PublicFriendCode {
			f.FriendCode = other.FriendCode
		}
		if h := other.HomeIsland; h != nil {
			f.OwnsIsland = true
			f.Visitors = h.Visitors
			f.CanBuild = h.Builders[p.profile.ID]
		}
		result = append(result, f)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
