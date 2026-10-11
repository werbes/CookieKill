package game

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

const (
	CamelRaceIsland      = "camel_race"
	CamelRaceFirstStart  = 120.0
	CamelRaceInterval    = 18 * 60.0
	CamelRaceGather      = 20.0
	CamelRaceTimeout     = 5 * 60.0
	CamelRaceLaps        = 3
	CamelRaceCheckpoints = 16
)

type CamelRaceTrack struct {
	OuterX      float64 `json:"outerX"`
	OuterZ      float64 `json:"outerZ"`
	InnerX      float64 `json:"innerX"`
	InnerZ      float64 `json:"innerZ"`
	Checkpoints int     `json:"checkpoints"`
}

var camelRaceTrack = CamelRaceTrack{60, 42, 43, 25, CamelRaceCheckpoints}

type CamelRaceEntryView struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	Lap        int     `json:"lap"`
	Checkpoint int     `json:"checkpoint"`
	Rank       int     `json:"rank"`
	Coins      int     `json:"coins"`
	CamelCoins int     `json:"camelCoins"`
	FinishTime float64 `json:"finishTime"`
}

type CamelRaceView struct {
	Mode              string                 `json:"mode"`
	LobbyID           string                 `json:"lobbyId"`
	HostID            string                 `json:"hostId"`
	CasualLobbies     []CamelCasualLobbyView `json:"casualLobbies"`
	ReminderStartAt   float64                `json:"reminderStartAt"`
	ReminderSeconds   float64                `json:"reminderSeconds"`
	NextStartAt       float64                `json:"nextStartAt"`
	NextGatherAt      float64                `json:"nextGatherAt"`
	RegisteredStartAt float64                `json:"registeredStartAt"`
	InArena           bool                   `json:"inArena"`
	Phase             string                 `json:"phase"`
	StartAt           float64                `json:"startAt"`
	GatherAt          float64                `json:"gatherAt"`
	EndsAt            float64                `json:"endsAt"`
	Laps              int                    `json:"laps"`
	EntrantCount      int                    `json:"entrantCount"`
	Entrants          []CamelRaceEntryView   `json:"entrants"`
	Track             CamelRaceTrack         `json:"track"`
	OwnCamel          *Camel                 `json:"ownCamel,omitempty"`
}

type camelRaceReturn struct {
	x, y, z, yaw, pitch   float64
	island                string
	inCave, ridingCamel   bool
	mountedCamelID        string
	stamina               float64
	sprintExhausted       bool
	fallen                bool
	standAt, fallDistance float64
}

type camelRaceEntry struct {
	CamelRaceEntryView
	progress    float64
	checkpoints int
	returnTo    camelRaceReturn
	returned    bool
}

type camelRace struct {
	island       string
	casual       bool
	hostID       string
	startAt      float64
	entries      map[string]*camelRaceEntry
	entrantCount int
	finished     int
	returnAt     float64
}

// Registrations close at the gather instant, including actions arriving before
// the next tick. A registration cannot enter a race whose field is already set.
func nextCamelRaceStart(seconds float64) float64 {
	cycle := math.Floor((seconds+CamelRaceGather-CamelRaceFirstStart+1e-8)/CamelRaceInterval) + 1
	return CamelRaceFirstStart + max(0, cycle)*CamelRaceInterval
}

func (w *World) seedRace() {
	w.nodes = append(w.nodes, &Node{ID: "camel_race_lollipop", Kind: "camel_race_lollipop", Label: "Camel lollipop · enter the next race", X: -227, Z: 20, Available: true})
	w.layout.Props = append(w.layout.Props, WorldProp{ID: "camel_race_cave", Kind: "camel_race_cave", Label: "Camel Cove", X: -227.5, Z: 18, Width: 23, Depth: 20, Height: 9, Scale: 1})
	for _, c := range []Collider{
		{ID: "camel_race_cave_back", X: -238.5, Z: 18, Width: 1, Depth: 20, Height: 9},
		{ID: "camel_race_cave_north", X: -227.5, Z: 8.5, Width: 23, Depth: 1, Height: 9},
		{ID: "camel_race_cave_south", X: -227.5, Z: 27.5, Width: 23, Depth: 1, Height: 9},
		{ID: "camel_race_cave_entry_north", X: -216.5, Z: 13, Width: 1, Depth: 10, Height: 9},
		{ID: "camel_race_cave_entry_south", X: -216.5, Z: 26, Width: 1, Depth: 4, Height: 9},
		{ID: "camel_race_cave_lintel", X: -216.5, Z: 21, Width: 1, Depth: 6, Y: 4.5, Height: 4.5},
		{ID: "camel_race_cave_roof", X: -227.5, Z: 18, Width: 23, Depth: 20, Y: 4.5, Height: 4.5},
	} {
		w.colliders = append(w.colliders, c)
	}
}

func (w *World) enterCamelRace(p *player, target string) error {
	if target != "camel_race_lollipop" || p.island != "" || p.inCave || !grounded(p) || distance(p.x, p.z, -227, 20) > 4 {
		return errors.New("Taste the camel lollipop inside Camel Cove on the far western beach.")
	}
	if w.raceRegistrations[p.profile.ID] != 0 {
		return errors.New("You are already registered for the next camel race.")
	}
	if w.raceRegistrations == nil {
		w.raceRegistrations = map[string]float64{}
	}
	start := nextCamelRaceStart(w.time)
	w.raceRegistrations[p.profile.ID] = start
	w.eventActor(fmt.Sprintf("Camel race entered! The starting grid gathers in %.0f seconds. Your camel comes along; a race camel is provided if needed.", start-CamelRaceGather-w.time), "notification", p.x, p.z, p.profile.ID)
	return nil
}

func (w *World) leaveCamelRace(p *player) error {
	if r := w.camelRaceForPlayer(p); r != nil {
		if entry := r.entries[p.profile.ID]; entry != nil && !entry.returned {
			if entry.Status == "racing" {
				entry.Status = "dnf"
			}
			w.returnCamelRacer(p, entry)
			if r.casual {
				w.refreshCasualRace(r)
			}
			return nil
		}
	}
	if w.raceRegistrations[p.profile.ID] != 0 {
		delete(w.raceRegistrations, p.profile.ID)
		return nil
	}
	return errors.New("You are not entered in a camel race.")
}

func (w *World) gatherCamelRace(start float64) {
	if w.camelRace != nil {
		w.returnAllCamelRacers()
	}
	r := &camelRace{startAt: start, entries: map[string]*camelRaceEntry{}}
	ids := []string{}
	for id, registration := range w.raceRegistrations {
		if registration > start {
			continue
		}
		delete(w.raceRegistrations, id)
		if p := w.players[id]; registration == start && p != nil && !p.disconnected && w.time < start+CamelRaceTimeout {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for i, id := range ids {
		p := w.players[id]
		w.gatherCamelRacer(r, p, i)
		w.eventActor("Camel Cove race: follow the ring through each checkpoint. Three laps; the grid opens at the countdown.", "notification", p.x, p.z, id)
	}
	r.entrantCount = len(r.entries)
	if r.entrantCount == 0 {
		w.camelRace = nil
		return
	}
	w.camelRace = r
}

func (w *World) gatherCamelRacer(r *camelRace, p *player, i int) {
	w.leaveRollerCoaster(p)
	id := p.profile.ID
	w.clearCookieArenaShots(id)
	if p.cookieArena != nil {
		_ = w.leaveCookieArena(p)
	}
	if isCamelRaceIsland(p.island) {
		_ = w.leaveCamelRace(p)
	}
	trainingCamelID := ""
	if p.island == CamelTrainingIsland {
		if p.training != nil {
			trainingCamelID = p.training.camelID
		}
		_ = w.leaveCamelTraining(p)
	}
	syncMountedCamel(p)
	w.expireCamels(p)
	loan := Camel{ID: fmt.Sprintf("race_%0.f_%s", r.startAt, id), Name: "Race camel"}
	if c := ownCamel(p, trainingCamelID); c != nil {
		loan = *c
	} else if c := mountedCamel(p); c != nil {
		loan = *c
	} else if len(p.profile.Camels) > 0 {
		loan = p.profile.Camels[0]
	}
	camelTraits(&loan)
	e := &camelRaceEntry{CamelRaceEntryView: CamelRaceEntryView{ID: id, Name: p.profile.CallName, Status: "racing", Checkpoint: 1}, returnTo: camelRaceReturn{
		x: p.x, y: p.y, z: p.z, yaw: p.yaw, pitch: p.pitch, island: p.island, inCave: p.inCave, ridingCamel: p.ridingCamel, mountedCamelID: p.mountedCamelID,
		stamina: p.stamina, sprintExhausted: p.sprintExhausted, fallen: p.fallen, standAt: p.standAt, fallDistance: p.fallDistance,
	}}
	p.island, p.inCave, p.y = camelRaceIsland(r), false, 0
	w.leaveJohnPorkBehind(p)
	p.x, p.z, p.yaw, p.pitch = 47+float64(i%5)*2, -1.5-float64((i/5)%6)*1.7, math.Pi, 0
	p.ridingCamel, p.mountedCamelID, p.raceCamel = true, loan.ID, &loan
	p.posture, p.buildMode, p.editingBuildID, p.bakeryBuildMode, p.editingBakeryFurnitureID = "", false, "", false, ""
	p.bakerySeatID, p.clubSeatID = "", ""
	p.fallen, p.standAt, p.fallDistance = false, 0, 0
	p.stamina, p.sprintExhausted = loan.Stamina, false
	resetMovement(p)
	e.progress = camelRaceAngle(p.x, p.z)
	r.entries[id] = e
}

func (w *World) returnCamelRacer(p *player, entry *camelRaceEntry) {
	if entry.returned {
		return
	}
	entry.returned = true
	saved := entry.returnTo
	p.raceCamel = nil
	p.x, p.y, p.z, p.yaw, p.pitch = saved.x, saved.y, saved.z, saved.yaw, saved.pitch
	p.island, p.inCave, p.ridingCamel, p.mountedCamelID = saved.island, saved.inCave, saved.ridingCamel, saved.mountedCamelID
	p.posture = ""
	p.stamina, p.sprintExhausted = saved.stamina, saved.sprintExhausted
	p.fallen, p.standAt, p.fallDistance = saved.fallen, saved.standAt, saved.fallDistance
	resetMovement(p)
	if p.island != "" && !(p.island == nutIslandID(p.profile.ID) && p.profile.NutIsland != nil) {
		profile, exists := w.profileByID(p.island)
		if !exists || profile.HomeIsland == nil || p.island != p.profile.ID && !profile.HomeIsland.Visitors {
			p.x, p.y, p.z, p.island, p.inCave = -227, 0, 20, "", false
			p.ridingCamel, p.mountedCamelID = false, ""
		}
	}
	w.expireCamels(p)
	if p.ridingCamel && mountedCamel(p) == nil {
		p.ridingCamel, p.mountedCamelID = false, ""
	}
	syncMountedCamel(p)
}

func (w *World) returnAllCamelRacers() {
	w.returnCamelRaceEntrants(w.camelRace)
}

func (w *World) returnCamelRaceEntrants(r *camelRace) {
	if r == nil {
		return
	}
	for id, entry := range r.entries {
		p := w.players[id]
		if p != nil && !entry.returned {
			if entry.Status == "racing" {
				entry.Status = "dnf"
			}
			w.returnCamelRacer(p, entry)
		}
	}
}

func camelRaceAngle(x, z float64) float64 {
	return math.Atan2(z/33.5, x/51.5)
}

func camelRaceTrackContains(x, z float64) bool {
	return finite(x) && finite(z) && math.Pow(x/(camelRaceTrack.OuterX-.6), 2)+math.Pow(z/(camelRaceTrack.OuterZ-.6), 2) <= 1 &&
		math.Pow(x/(camelRaceTrack.InnerX+.6), 2)+math.Pow(z/(camelRaceTrack.InnerZ+.6), 2) >= 1
}

// Every movement segment is collision checked and advances an unwrapped angle.
// Reversing subtracts progress; recrossing the same gate never earns another lap.
func (w *World) moveCamelRacer(p *player, entry *camelRaceEntry, dt float64) float64 {
	if dt <= 0 || entry.Status != "racing" {
		return 0
	}
	if w.time-p.cooldowns["last_input"] > .5 {
		p.input = Input{Yaw: p.yaw, Pitch: p.pitch}
		p.jumpQueued = false
	}
	moving := math.Hypot(p.input.X, p.input.Z) > .01
	if p.stamina <= 0 {
		p.sprintExhausted = true
	}
	if p.sprintExhausted && !p.sprintHeld && p.stamina >= staminaCapacity(p)*.2 {
		p.sprintExhausted = false
	}
	p.sprinting = moving && p.input.Sprint && !p.sprintExhausted
	speed := camelBaseSpeed(p)
	if p.sprinting {
		speed *= 1.65
		p.stamina = max(0, p.stamina-18*dt)
	} else {
		p.stamina = min(staminaCapacity(p), p.stamina+14*dt)
	}
	if p.jumpQueued && grounded(p) && p.cooldowns["jump"] <= w.time {
		p.jumpVelocity = camelJumpSpeed(p)
	}
	p.jumpQueued = false
	if !grounded(p) {
		p.jumpOffset += p.jumpVelocity*dt - jumpGravity*dt*dt/2
		p.jumpVelocity -= jumpGravity * dt
		if p.jumpOffset <= 0 {
			p.jumpOffset, p.jumpVelocity = 0, 0
			p.cooldowns["jump"] = w.time + .12
		}
	}
	dx, dz := p.input.X*speed*dt, p.input.Z*speed*dt
	steps := max(1, int(math.Ceil(math.Hypot(dx, dz)/.2)))
	for i := 0; i < steps; i++ {
		before := camelRaceAngle(p.x, p.z)
		if camelRaceTrackContains(p.x+dx/float64(steps), p.z) {
			p.x += dx / float64(steps)
		}
		if camelRaceTrackContains(p.x, p.z+dz/float64(steps)) {
			p.z += dz / float64(steps)
		}
		delta := math.Remainder(camelRaceAngle(p.x, p.z)-before, 2*math.Pi)
		previous := entry.progress
		entry.progress += delta
		for entry.checkpoints < CamelRaceLaps*CamelRaceCheckpoints && entry.progress+1e-9 >= float64(entry.checkpoints+1)*2*math.Pi/CamelRaceCheckpoints {
			entry.checkpoints++
		}
		entry.Lap = entry.checkpoints / CamelRaceCheckpoints
		entry.Checkpoint = entry.checkpoints%CamelRaceCheckpoints + 1
		if entry.checkpoints == CamelRaceLaps*CamelRaceCheckpoints {
			fraction := max(0, min(1, (CamelRaceLaps*2*math.Pi-previous)/max(1e-12, delta)))
			return w.time - dt + (float64(i)+fraction)*dt/float64(steps)
		}
	}
	return 0
}

func (w *World) finishCamelRacer(p *player, entry *camelRaceEntry, at float64) {
	if entry.Status != "racing" {
		return
	}
	r := w.camelRaceForPlayer(p)
	if r == nil {
		return
	}
	r.finished++
	entry.Status, entry.Rank, entry.FinishTime = "finished", r.finished, max(0, at-r.startAt)
	if !r.casual && entry.Rank <= 3 {
		coins := []int{200, 100, 50}[entry.Rank-1] * max(0, r.entrantCount-1)
		before := p.profile.Coins
		addCoins(p, coins)
		entry.Coins = p.profile.Coins - before
		entry.CamelCoins = min([]int{50, 30, 10}[entry.Rank-1], coinLimit-p.profile.CamelCoins)
		p.profile.CamelCoins += entry.CamelCoins
	}
	resetMovement(p)
	p.sprinting = false
	if r.casual {
		w.eventActor(fmt.Sprintf("Friendly camel race: place %d! Three laps completed in %.1f seconds. No prizes or entry fees.", entry.Rank, entry.FinishTime), "notification", p.x, p.z, p.profile.ID)
		return
	}
	w.eventActor(fmt.Sprintf("Camel race: place %d! Earned %d coins and %d CamelCoins. Leave the track whenever you are ready.", entry.Rank, entry.Coins, entry.CamelCoins), "notification", p.x, p.z, p.profile.ID)
}

func (w *World) tickCamelRace(dt float64) {
	for _, p := range w.players {
		w.remindCamelRace(p)
	}
	latest := CamelRaceFirstStart + math.Floor((w.time+CamelRaceGather-CamelRaceFirstStart+1e-8)/CamelRaceInterval)*CamelRaceInterval
	if latest >= CamelRaceFirstStart && latest > w.raceStartHandled {
		w.raceStartHandled = latest
		w.gatherCamelRace(latest)
	}
	w.tickCamelRaceInstance(w.camelRace, dt)
	for _, r := range w.casualRaces {
		w.tickCamelRaceInstance(r, dt)
		w.refreshCasualRace(r)
	}
}

func (w *World) tickCamelRaceInstance(r *camelRace, dt float64) {
	if r == nil {
		return
	}
	type finisher struct {
		p     *player
		entry *camelRaceEntry
		at    float64
	}
	finished := []finisher{}
	for id, entry := range r.entries {
		if entry.returned {
			continue
		}
		p := w.players[id]
		if p == nil || p.disconnected || p.island != camelRaceIsland(r) {
			if entry.Status == "racing" {
				entry.Status = "dnf"
			}
			if p != nil {
				w.returnCamelRacer(p, entry)
			}
			continue
		}
		if r.casual && r.startAt == 0 {
			p.jumpQueued, p.sprinting = false, false
			continue
		}
		if w.time >= r.startAt+CamelRaceTimeout {
			if entry.Status == "racing" {
				entry.Status = "dnf"
			}
			w.returnCamelRacer(p, entry)
			continue
		}
		if w.time+1e-8 < r.startAt || entry.Status != "racing" {
			p.jumpQueued, p.sprinting = false, false
			continue
		}
		if at := w.moveCamelRacer(p, entry, min(dt, max(0, w.time-r.startAt))); at > 0 {
			finished = append(finished, finisher{p, entry, at})
		}
	}
	sort.Slice(finished, func(i, j int) bool {
		if finished[i].at != finished[j].at {
			return finished[i].at < finished[j].at
		}
		return finished[i].entry.ID < finished[j].entry.ID
	})
	for _, f := range finished {
		w.finishCamelRacer(f.p, f.entry, f.at)
	}
	allDone := true
	for _, entry := range r.entries {
		if entry.Status == "racing" {
			allDone = false
			break
		}
	}
	if allDone && r.returnAt == 0 {
		r.returnAt = w.time + 20
	}
	if r.returnAt > 0 && w.time >= r.returnAt {
		w.returnCamelRaceEntrants(r)
	}
}

func (w *World) camelRaceSnapshot(id string) CamelRaceView {
	next := nextCamelRaceStart(w.time)
	view := CamelRaceView{Mode: "scheduled", CasualLobbies: w.casualRaceLobbies(), NextStartAt: next, NextGatherAt: next - CamelRaceGather, RegisteredStartAt: w.raceRegistrations[id], Phase: "registration", Laps: CamelRaceLaps, Entrants: []CamelRaceEntryView{}, Track: camelRaceTrack}
	view.ReminderStartAt, view.ReminderSeconds = camelRaceReminder(w.time)
	r := w.camelRace
	if p := w.players[id]; p != nil && isCamelRaceIsland(p.island) {
		r = w.camelRaceForPlayer(p)
		view.InArena = true
		if p.raceCamel != nil {
			c := *p.raceCamel
			view.OwnCamel = &c
		}
	}
	if r == nil {
		return view
	}
	view.StartAt, view.GatherAt, view.EndsAt, view.EntrantCount = r.startAt, r.startAt-CamelRaceGather, r.startAt+CamelRaceTimeout, r.entrantCount
	view.Phase = camelRacePhase(r, w.time)
	if r.casual {
		view.Mode, view.LobbyID, view.HostID = "casual", casualRaceID(r), r.hostID
		view.GatherAt = 0
		if r.startAt == 0 {
			view.EndsAt = 0
		}
	}
	entries := make([]*camelRaceEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Rank != b.Rank && (a.Rank > 0 || b.Rank > 0) {
			return a.Rank > 0 && (b.Rank == 0 || a.Rank < b.Rank)
		}
		if a.Status != b.Status {
			return a.Status == "racing"
		}
		if a.progress != b.progress {
			return a.progress > b.progress
		}
		return a.ID < b.ID
	})
	for _, entry := range entries {
		view.Entrants = append(view.Entrants, entry.CamelRaceEntryView)
	}
	return view
}

func (w *World) decorateCamelRaceSnapshot(viewer *player, out *Snapshot) {
	if viewer == nil || !isCamelRaceIsland(viewer.island) {
		return
	}
	out.Me.Area, out.Me.Zone = "camel race", "Camel Cove ring"
	out.Me.CanBuildHome, out.Me.BuildMode, out.Me.BakeryBuildMode = false, false, false
	out.Nodes = []Node{}
	out.Island = nil
	for i := range out.Players {
		out.Players[i].Area = "camel race"
	}
}
