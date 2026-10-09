package game

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

const camelCasualPrefix = "camel_casual:"
const camelCasualCountdown = 10.0
const camelCasualCapacity = 30

type CamelCasualLobbyView struct {
	ID           string  `json:"id"`
	HostID       string  `json:"hostId"`
	HostName     string  `json:"hostName"`
	EntrantCount int     `json:"entrantCount"`
	Phase        string  `json:"phase"`
	StartAt      float64 `json:"startAt"`
}

func isCamelRaceIsland(island string) bool {
	return island == CamelRaceIsland || strings.HasPrefix(island, camelCasualPrefix)
}

func camelRaceIsland(r *camelRace) string {
	if r.island != "" {
		return r.island
	}
	return CamelRaceIsland
}

func casualRaceID(r *camelRace) string { return strings.TrimPrefix(r.island, camelCasualPrefix) }

func (w *World) camelRaceForPlayer(p *player) *camelRace {
	if p.island == CamelRaceIsland {
		return w.camelRace
	}
	return w.casualRaces[p.island]
}

func camelRacePhase(r *camelRace, now float64) string {
	if r.casual && r.startAt == 0 {
		return "lobby"
	}
	if r.returnAt != 0 {
		return "finished"
	}
	if now < r.startAt {
		return "gathering"
	}
	return "racing"
}

func (w *World) casualCamelRaceAction(p *player, a Action) error {
	if p.disconnected || p.fallen {
		return errors.New("Stand up and connect before joining a friendly race.")
	}
	if a.Action == "race_casual_start" {
		r := w.camelRaceForPlayer(p)
		if r == nil || !r.casual || r.hostID != p.profile.ID {
			return errors.New("Only this friendly race's host can start it.")
		}
		if r.startAt != 0 {
			return errors.New("This race has already started its countdown.")
		}
		r.startAt, r.entrantCount = w.time+camelCasualCountdown, len(r.entries)
		for id := range r.entries {
			entrant := w.players[id]
			if entrant != nil {
				w.eventActor("Friendly camel race starts in 10 seconds! Follow all checkpoints for three laps. No entry fee or prizes.", "notification", entrant.x, entrant.z, id)
			}
		}
		return nil
	}
	if isCamelRaceIsland(p.island) {
		return errors.New("Leave your current race before joining another one.")
	}
	var r *camelRace
	switch a.Action {
	case "race_casual_host":
		if w.casualRaces == nil {
			w.casualRaces = map[string]*camelRace{}
		}
		w.sequence++
		island := fmt.Sprintf("%s%d", camelCasualPrefix, w.sequence)
		r = &camelRace{island: island, casual: true, hostID: p.profile.ID, entries: map[string]*camelRaceEntry{}}
		w.casualRaces[island] = r
	case "race_casual_join":
		r = w.casualRaces[camelCasualPrefix+a.Target]
		if r == nil || r.startAt != 0 {
			return errors.New("Choose a friendly race that is still waiting in its lobby.")
		}
		if len(r.entries) >= camelCasualCapacity {
			return errors.New("This friendly race has a full starting grid. Host another race to play now.")
		}
	default:
		return errors.New("Choose Host, Join, or Start a friendly camel race.")
	}
	w.gatherCamelRacer(r, p, len(r.entries))
	r.entrantCount = len(r.entries)
	w.eventActor("Friendly camel race lobby: invite friends through Teleport, or start a solo practice race. Your original location and camel are saved.", "notification", p.x, p.z, p.profile.ID)
	return nil
}

// A departure frees a waiting-grid slot and transfers hosting deterministically.
// Finished results remain until every participant has returned home.
func (w *World) refreshCasualRace(r *camelRace) {
	ids := []string{}
	for id, entry := range r.entries {
		if entry.returned {
			if r.startAt == 0 {
				delete(r.entries, id)
			}
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		delete(w.casualRaces, r.island)
		return
	}
	sort.Strings(ids)
	if host := r.entries[r.hostID]; host == nil || host.returned {
		r.hostID = ids[0]
	}
	if r.startAt == 0 {
		r.entrantCount = len(ids)
		// Re-form the waiting grid after departures, avoiding reused occupied slots.
		for i, id := range ids {
			if p := w.players[id]; p != nil && p.island == r.island {
				p.x, p.z = 47+float64(i%5)*2, -1.5-float64((i/5)%6)*1.7
				r.entries[id].progress = camelRaceAngle(p.x, p.z)
			}
		}
	}
}

func (w *World) casualRaceLobbies() []CamelCasualLobbyView {
	views := make([]CamelCasualLobbyView, 0, len(w.casualRaces))
	for _, r := range w.casualRaces {
		hostName := "Racer"
		if host := r.entries[r.hostID]; host != nil {
			hostName = host.Name
		}
		views = append(views, CamelCasualLobbyView{ID: casualRaceID(r), HostID: r.hostID, HostName: hostName, EntrantCount: r.entrantCount, Phase: camelRacePhase(r, w.time), StartAt: r.startAt})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].ID < views[j].ID })
	return views
}

// Unlike signup deadlines, reminders continue through the final 20-second grid
// countdown. A late join sees the imminent start without being promised entry.
func camelRaceReminder(now float64) (start, remaining float64) {
	cycle := max(0, math.Floor((now-CamelRaceFirstStart)/CamelRaceInterval)+1)
	start = CamelRaceFirstStart + cycle*CamelRaceInterval
	remaining = start - now
	if remaining <= 0 || remaining > 120 {
		return 0, 0
	}
	return start, remaining
}

func (w *World) remindCamelRace(p *player) {
	start, remaining := camelRaceReminder(w.time)
	if p.disconnected || start == 0 || p.raceReminderStart == start {
		return
	}
	p.raceReminderStart = start
	message := fmt.Sprintf("Scheduled camel race starts in %.0f seconds! Enter at the Camel Cove lollipop before the grid gathers 20 seconds before the start.", math.Ceil(remaining))
	if remaining <= CamelRaceGather {
		message = fmt.Sprintf("Scheduled camel race starts in %.0f seconds! Its starting grid has gathered; signups at Camel Cove now enter the following race.", math.Ceil(remaining))
	} else if remaining >= 120 {
		message = "Scheduled camel race starts in 2 minutes! Enter at the Camel Cove lollipop before the grid gathers 20 seconds before the start."
	}
	w.sequence++
	// A private event cannot be displaced by a busy shared event buffer, and it
	// follows its recipient between worlds during the notification's lifetime.
	p.raceReminder = Event{ID: w.sequence, Text: message, Kind: "notification", X: p.x, Z: p.z, Time: w.time, Actor: p.profile.ID, Island: p.island}
}
