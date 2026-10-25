package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"cookiekill/internal/auth"
	"cookiekill/internal/game"
	"cookiekill/internal/store"
)

func coordinatorServer(t *testing.T, persist func(map[string]game.Profile) error, timing serverTiming) *Server {
	t.Helper()
	a, err := auth.New(auth.Config{DevMode: true, SessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if persist == nil {
		persist = db.Save
	}
	s := newServer(a, db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), persist, timing)
	t.Cleanup(func() { s.Close() })
	return s
}

func attachTestPeer(t *testing.T, s *Server, id string) *peer {
	return attachTestPeerWithResume(t, s, id, "")
}

func attachTestPeerWithResume(t *testing.T, s *Server, id, resumeID string) *peer {
	t.Helper()
	p := &peer{out: make(chan outbound, 2), cancel: func() {}, baselineReady: make(chan struct{}), resumeID: resumeID}
	if !s.attach(context.Background(), id, id, p) {
		t.Fatalf("cannot attach %s", id)
	}
	return p
}

func applyTestCommand(t *testing.T, s *Server, id string, p *peer, input *game.Input, action *game.Action) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.apply(ctx, playerCommand{id: id, peer: p, input: input, action: action, reply: make(chan error, 1)}); err != nil {
		t.Fatal(err)
	}
}

func inspectTestOwner(t *testing.T, s *Server, id string) ownerView {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	view, err := s.inspect(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func waitCoordinator(t *testing.T, label string, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !predicate() {
		if time.Now().After(deadline) {
			t.Fatal(label)
		}
		time.Sleep(time.Millisecond)
	}
}

func startTestRun(t *testing.T, s *Server) (context.CancelFunc, <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	return cancel, done
}

func TestOwnerSupportsWorldAPIBeforeRunAndCloseBeforeRun(t *testing.T) {
	s := coordinatorServer(t, nil, serverTiming{10 * time.Millisecond, 20 * time.Millisecond, time.Hour})
	handler := s.Handler(fstest.MapFS{"index.html": {Data: []byte("game")}}, false)
	request := httptest.NewRequest("GET", "http://localhost/api/world", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("pre-Run API failed: %d", response.Code)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { s.Run(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run after Close did not return")
	}
	select {
	case <-s.saveDone:
	default:
		t.Fatal("Close left the save worker running")
	}
	if err := s.Save(); !errors.Is(err, errClosed) {
		t.Fatalf("save after close: %v", err)
	}
}

func TestSlowAutosaveDoesNotBlockTicksInputOrRunCancellation(t *testing.T) {
	started := make(chan map[string]game.Profile, 4)
	release := make(chan struct{})
	var unblock sync.Once
	t.Cleanup(func() { unblock.Do(func() { close(release) }) })
	s := coordinatorServer(t, func(profiles map[string]game.Profile) error { started <- profiles; <-release; return nil }, serverTiming{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond})
	// Unblock before the server cleanup if any assertion fails.
	t.Cleanup(func() { unblock.Do(func() { close(release) }) })
	p := attachTestPeer(t, s, "alice")
	cancel, runDone := startTestRun(t, s)
	var first map[string]game.Profile
	select {
	case first = <-started:
	case <-time.After(time.Second):
		t.Fatal("autosave never started")
	}
	before := inspectTestOwner(t, s, "alice").snapshot
	applyTestCommand(t, s, "alice", p, &game.Input{X: 1, Jump: true}, nil)
	applyTestCommand(t, s, "alice", p, &game.Input{X: 1, Jump: false}, nil)
	applyTestCommand(t, s, "alice", p, nil, &game.Action{Action: "move_item", From: "hotbar", FromSlot: 0, To: "bag", ToSlot: 0})
	waitCoordinator(t, "blocked disk stopped simulation or lost a Space press/release", func() bool {
		next := inspectTestOwner(t, s, "alice").snapshot
		return next.Time > before.Time+.06 && next.Me.X > before.Me.X+.1 && !next.Me.Grounded && next.Me.BagSlots[0].Count == 10
	})
	if first["alice"].BagSlots[0].Count != 0 || first["alice"].Hotbar[0].Count != 10 {
		t.Fatal("worker snapshot mutated while simulation continued")
	}
	if len(s.saves) > 1 {
		t.Fatal("autosave backlog is unbounded")
	}
	cancel()
	select {
	case <-runDone:
	case <-time.After(time.Second):
		t.Fatal("canceling Run waited for disk I/O")
	}
	unblock.Do(func() { close(release) })
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveAndCloseBarriersCoalesceInOrderAndKeepNewestProgress(t *testing.T) {
	started := make(chan map[string]game.Profile, 4)
	release := make(chan struct{})
	var unblock sync.Once
	s := coordinatorServer(t, func(profiles map[string]game.Profile) error { started <- profiles; <-release; return nil }, serverTiming{10 * time.Millisecond, 20 * time.Millisecond, time.Hour})
	t.Cleanup(func() { unblock.Do(func() { close(release) }) })
	p := attachTestPeer(t, s, "alice")
	firstDone := make(chan error, 1)
	go func() { firstDone <- s.Save() }()
	var first map[string]game.Profile
	select {
	case first = <-started:
	case <-time.After(time.Second):
		t.Fatal("first Save did not reach worker")
	}
	secondDone := make(chan error, 1)
	go func() { secondDone <- s.Save() }()
	waitCoordinator(t, "second Save was not queued", func() bool { return len(s.saves) == 1 })
	applyTestCommand(t, s, "alice", p, nil, &game.Action{Action: "move_item", From: "hotbar", FromSlot: 0, To: "bag", ToSlot: 0})
	closeDone := make(chan error, 1)
	go func() { closeDone <- s.Close() }()
	waitCoordinator(t, "Close did not seal command admission", func() bool { return inspectTestOwner(t, s, "alice").closing })
	select {
	case <-secondDone:
		t.Fatal("Save barrier returned before queued data reached disk")
	default:
	}
	select {
	case <-closeDone:
		t.Fatal("Close returned before final save")
	default:
	}
	if first["alice"].BagSlots[0].Count != 0 {
		t.Fatal("in-flight save aliases current progress")
	}
	unblock.Do(func() { close(release) })
	for _, done := range []chan error{firstDone, secondDone, closeDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("persistence barrier did not complete")
		}
	}
	select {
	case latest := <-started:
		if latest["alice"].BagSlots[0].Count != 10 || latest["alice"].Hotbar[0].Count != 0 {
			t.Fatal("final save lost latest progress")
		}
	default:
		t.Fatal("final snapshot was never persisted")
	}
	select {
	case <-started:
		t.Fatal("coalesced older save ran after final persistence")
	default:
	}
	select {
	case <-s.saveDone:
	default:
		t.Fatal("save worker still running after Close")
	}
}

func TestCloseReturnsPersistenceFailureToAllCallers(t *testing.T) {
	want := errors.New("simulated disk failure")
	s := coordinatorServer(t, func(map[string]game.Profile) error { return want }, serverTiming{10 * time.Millisecond, 20 * time.Millisecond, time.Hour})
	results := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() { results <- s.Close() }()
	}
	for i := 0; i < 4; i++ {
		select {
		case got := <-results:
			if !errors.Is(got, want) {
				t.Fatalf("Close error = %v", got)
			}
		case <-time.After(time.Second):
			t.Fatal("concurrent Close hung")
		}
	}
}

func TestCapacityAndReplacementAreCheckedByOwnerAfterHandshake(t *testing.T) {
	s := coordinatorServer(t, nil, serverTiming{10 * time.Millisecond, 20 * time.Millisecond, time.Hour})
	var first *peer
	for i := 0; i < maxPlayers-1; i++ {
		p := attachTestPeer(t, s, fmt.Sprintf("player-%d", i))
		if i == 0 {
			first = p
		}
	}
	if !s.capacity(context.Background(), "late-handshake") {
		t.Fatal("pre-handshake capacity unexpectedly full")
	}
	attachTestPeer(t, s, "last-slot")
	extra := &peer{out: make(chan outbound, 2), cancel: func() {}}
	if s.attach(context.Background(), "late-handshake", "Late", extra) {
		t.Fatal("handshake race bypassed capacity")
	}
	if !s.capacity(context.Background(), "player-0") {
		t.Fatal("full world rejected replacement for an existing player")
	}
	replacement := attachTestPeer(t, s, "player-0")
	oldAction := playerCommand{id: "player-0", peer: first, action: &game.Action{Action: "move_item", From: "hotbar", FromSlot: 0, To: "bag", ToSlot: 0}, reply: make(chan error, 1)}
	if err := s.apply(context.Background(), oldAction); !errors.Is(err, errReplaced) {
		t.Fatalf("old peer action accepted: %v", err)
	}
	s.submit(context.Background(), detachCommand{id: "player-0", peer: first})
	view := inspectTestOwner(t, s, "player-0")
	if view.players != maxPlayers || view.snapshot.Me.BagSlots[0].Count != 0 {
		t.Fatal("old connection disconnected or modified its replacement")
	}
	applyTestCommand(t, s, "player-0", replacement, nil, &game.Action{Action: "move_item", From: "hotbar", FromSlot: 0, To: "bag", ToSlot: 0})
	if inspectTestOwner(t, s, "player-0").snapshot.Me.BagSlots[0].Count != 10 {
		t.Fatal("replacement cannot act")
	}
}

func TestDetachUnloadsPlayerWithoutSimulationTicksAndKeepsProgress(t *testing.T) {
	for _, stopped := range []bool{false, true} {
		name := "before Run"
		if stopped {
			name = "after Run stops"
		}
		t.Run(name, func(t *testing.T) {
			s := coordinatorServer(t, nil, serverTiming{10 * time.Millisecond, 20 * time.Millisecond, time.Hour})
			p := attachTestPeer(t, s, "alice")
			if stopped {
				cancel, done := startTestRun(t, s)
				waitCoordinator(t, "simulation did not start", func() bool {
					return inspectTestOwner(t, s, "alice").snapshot.Time > 0
				})
				cancel()
				<-done
			}
			applyTestCommand(t, s, "alice", p, &game.Input{Yaw: 2.1, X: 1, Jump: true}, nil)
			applyTestCommand(t, s, "alice", p, nil, &game.Action{Action: "move_item", From: "hotbar", FromSlot: 0, To: "bag", ToSlot: 0})
			if err := s.submit(context.Background(), detachCommand{id: "alice", peer: p}); err != nil {
				t.Fatal(err)
			}
			view := inspectTestOwner(t, s, "alice")
			if view.players != 0 || view.snapshot.Me.ID != "" || len(view.snapshot.Players) != 0 {
				t.Fatal("disconnect retained a live player without simulation ticks")
			}
			if err := s.Save(); err != nil {
				t.Fatal(err)
			}
			saved, err := s.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if saved["alice"].BagSlots[0].Count != 10 || saved["alice"].Hotbar[0].Count != 0 {
				t.Fatal("unloading the player lost its latest account progress")
			}
			reconnected := attachTestPeer(t, s, "alice")
			fresh := (<-reconnected.out).snapshot
			if fresh == nil || fresh.Layout == nil || fresh.Me.Yaw != 0 || fresh.Me.BagSlots[0].Count != 10 {
				t.Fatal("reconnect did not create a fresh runtime from saved progress")
			}
		})
	}
}

func TestReplacementRebuildsPlayerAndIgnoresLateDetach(t *testing.T) {
	s := coordinatorServer(t, nil, serverTiming{10 * time.Millisecond, 20 * time.Millisecond, time.Hour})
	old := attachTestPeer(t, s, "alice")
	applyTestCommand(t, s, "alice", old, &game.Input{Yaw: 2.1, X: 1, Jump: true}, nil)
	applyTestCommand(t, s, "alice", old, nil, &game.Action{Action: "move_item", From: "hotbar", FromSlot: 0, To: "bag", ToSlot: 0})
	replacement := attachTestPeer(t, s, "alice")
	fresh := (<-replacement.out).snapshot
	if fresh == nil || fresh.Layout == nil || fresh.Me.Yaw != 0 || fresh.Me.BagSlots[0].Count != 10 {
		t.Fatal("socket replacement reused transient state or lost account progress")
	}
	if err := s.submit(context.Background(), detachCommand{id: "alice", peer: old}); err != nil {
		t.Fatal(err)
	}
	view := inspectTestOwner(t, s, "alice")
	if view.players != 1 || view.snapshot.Me.ID != "alice" {
		t.Fatal("late detach removed the replacement session")
	}
	applyTestCommand(t, s, "alice", replacement, &game.Input{Yaw: 1.2}, nil)
	if got := inspectTestOwner(t, s, "alice").snapshot.Me.Yaw; got != 1.2 {
		t.Fatalf("replacement cannot receive input: yaw=%v", got)
	}
}

func TestMatchingResumeIdentifierPreservesPlayerBeforeLateDetach(t *testing.T) {
	s := coordinatorServer(t, nil, serverTiming{10 * time.Millisecond, 20 * time.Millisecond, time.Hour})
	old := attachTestPeerWithResume(t, s, "alice", "same-tab")
	applyTestCommand(t, s, "alice", old, &game.Input{Yaw: 2.1, X: 1, Jump: true}, nil)
	applyTestCommand(t, s, "alice", old, nil, &game.Action{Action: "move_item", From: "hotbar", FromSlot: 0, To: "bag", ToSlot: 0})

	replacement := attachTestPeerWithResume(t, s, "alice", "same-tab")
	keyframe := (<-replacement.out).snapshot
	if keyframe == nil || keyframe.Layout == nil || keyframe.Me.Yaw != 2.1 || keyframe.Me.BagSlots[0].Count != 10 {
		t.Fatal("matching reconnect did not preserve transient state or account progress")
	}
	if err := s.submit(context.Background(), detachCommand{id: "alice", peer: old, resume: true}); err != nil {
		t.Fatal(err)
	}
	view := inspectTestOwner(t, s, "alice")
	if view.players != 1 || view.snapshot.Me.ID != "alice" || view.snapshot.Me.Yaw != 2.1 {
		t.Fatal("late detach interrupted the resumed session")
	}
}

func TestBusyInputMailboxCannotStarveSimulationTicks(t *testing.T) {
	s := coordinatorServer(t, nil, serverTiming{10 * time.Millisecond, 100 * time.Millisecond, time.Hour})
	peers := make([]*peer, 8)
	for i := range peers {
		peers[i] = attachTestPeer(t, s, fmt.Sprintf("flood-%d", i))
		close(peers[i].baselineReady)
	}
	startTestRun(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	var producers sync.WaitGroup
	for i, p := range peers {
		producers.Go(func() {
			for ctx.Err() == nil {
				s.apply(ctx, playerCommand{id: fmt.Sprintf("flood-%d", i), peer: p, input: &game.Input{Yaw: .2}, reply: make(chan error, 1)})
			}
		})
	}
	producers.Wait()
	if got := inspectTestOwner(t, s, "flood-0").snapshot.Time; got < .15 {
		t.Fatalf("flood starved ticks: world advanced only %v seconds", got)
	}
	if cap(s.commands) != commandCapacity {
		t.Fatal("world command mailbox is not bounded")
	}
}

func TestTickerJitterKeepsExactStepsAndTenHertzSnapshots(t *testing.T) {
	w := game.New(nil)
	w.Join("alice", "Alice")
	p := &peer{out: make(chan outbound, 2), baselineReady: make(chan struct{})}
	close(p.baselineReady)
	start := time.Unix(1000, 0)
	c := &coordinator{world: w, peers: map[string]*peer{"alice": p}, timing: serverTiming{50 * time.Millisecond, 100 * time.Millisecond, time.Hour}, lastTick: start, nextSnapshot: start.Add(100 * time.Millisecond), nextSave: start.Add(time.Hour)}
	frames := 0
	for tick := 1; tick <= 40; tick++ {
		jitter := time.Duration(100-tick%3*100) * time.Nanosecond
		c.tick(start.Add(time.Duration(tick)*50*time.Millisecond + jitter))
		select {
		case <-p.out:
			frames++
			if tick%2 != 0 {
				t.Fatalf("snapshot drifted to tick %d", tick)
			}
		default:
			if tick%2 == 0 {
				t.Fatalf("snapshot skipped at tick %d", tick)
			}
		}
	}
	if frames != 20 || math.Abs(w.SnapshotUpdate("alice").Time-2) > 1e-9 {
		t.Fatalf("jitter changed cadence: frames=%d worldTime=%v", frames, w.SnapshotUpdate("alice").Time)
	}
	c.tick(start.Add(2350 * time.Millisecond))
	if math.Abs(w.SnapshotUpdate("alice").Time-2.35) > 1e-9 {
		t.Fatal("missed ticker intervals did not catch up in fixed steps")
	}
	if !c.nextSnapshot.Equal(start.Add(2400 * time.Millisecond)) {
		t.Fatalf("broadcast deadline accumulated jitter: %v", c.nextSnapshot)
	}
}

func TestSnapshotLayoutRemainsUntilFirstSuccessfulWrite(t *testing.T) {
	w := game.New(nil)
	w.Join("alice", "Alice")
	p := &peer{out: make(chan outbound, 2), baselineReady: make(chan struct{})}
	c := &coordinator{world: w}
	for i := 0; i < 5; i++ {
		c.enqueueSnapshot("alice", p)
	}
	for len(p.out) > 0 {
		if snapshot := (<-p.out).snapshot; snapshot == nil || snapshot.Layout == nil {
			t.Fatal("dropping an unwritten first frame lost the initial world layout")
		}
	}
	close(p.baselineReady)
	c.enqueueSnapshot("alice", p)
	if snapshot := (<-p.out).snapshot; snapshot == nil || snapshot.Layout != nil {
		t.Fatal("written baseline did not switch to layout-free updates")
	}
}
