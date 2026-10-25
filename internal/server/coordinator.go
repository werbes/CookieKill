package server

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"cookiekill/internal/auth"
	"cookiekill/internal/game"
	"cookiekill/internal/store"
	"github.com/coder/websocket"
)

var (
	errClosed   = errors.New("World is shutting down")
	errReplaced = errors.New("This account was opened in another tab")
)

const (
	commandCapacity = 256
	reconnectGrace  = 30 * time.Second
)

type ownerCommand interface{ worldCommand() }
type commandTag struct{}

func (commandTag) worldCommand() {}

type capacityCommand struct {
	commandTag
	id    string
	reply chan bool
}
type attachCommand struct {
	commandTag
	id, name string
	peer     *peer
	reply    chan bool
}
type detachCommand struct {
	commandTag
	id     string
	peer   *peer
	resume bool
}
type playerCommand struct {
	commandTag
	id     string
	peer   *peer
	input  *game.Input
	action *game.Action
	reply  chan error
}
type layoutCommand struct {
	commandTag
	reply chan game.Layout
}
type runCommand struct {
	commandTag
	start bool
	reply chan struct{}
}
type saveCommand struct {
	commandTag
	reply chan error
}
type closeCommand struct{ commandTag }

// Inspection is also a message, so tests and diagnostics never race the owner.
type inspectCommand struct {
	commandTag
	id    string
	reply chan ownerView
}
type ownerView struct {
	snapshot game.Snapshot
	players  int
	closing  bool
}

type serverTiming struct{ tick, snapshot, autosave time.Duration }
type saveJob struct {
	profiles map[string]game.Profile
	waiters  []chan error
	final    bool
}
type suspendedSession struct {
	resumeID string
	until    time.Time
}

// coordinator is never shared: only ownWorld may access its mutable fields.
type coordinator struct {
	server                           *Server
	world                            *game.World
	peers                            map[string]*peer
	suspended                        map[string]suspendedSession
	closing                          bool
	timing                           serverTiming
	ticker                           *time.Ticker
	ticks                            <-chan time.Time
	lastTick, nextSnapshot, nextSave time.Time
	tickRemainder                    time.Duration
}

func newServer(a *auth.Service, db *store.Store, profiles map[string]game.Profile, logger *slog.Logger, persist func(map[string]game.Profile) error, timing serverTiming) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{auth: a, store: db, logger: logger, commands: make(chan ownerCommand, commandCapacity), done: make(chan struct{}), closeStarted: make(chan struct{}, 1), saveSlots: make(chan struct{}, 8), saves: make(chan saveJob, 1), finalSaved: make(chan error, 1), saveDone: make(chan struct{})}
	c := &coordinator{server: s, world: game.New(profiles), peers: make(map[string]*peer), suspended: make(map[string]suspendedSession), timing: timing}
	go s.saveWorker(persist)
	go c.ownWorld()
	return s
}

func (s *Server) submit(ctx context.Context, command ownerCommand) error {
	select {
	case <-s.done:
		return errClosed
	default:
	}
	select {
	case s.commands <- command:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return errClosed
	}
}

func (s *Server) capacity(ctx context.Context, id string) bool {
	reply := make(chan bool, 1)
	if s.submit(ctx, capacityCommand{id: id, reply: reply}) != nil {
		return false
	}
	select {
	case result := <-reply:
		return result
	case <-ctx.Done():
		return false
	case <-s.done:
		return false
	}
}

func (s *Server) attach(ctx context.Context, id, name string, p *peer) bool {
	reply := make(chan bool, 1)
	if s.submit(ctx, attachCommand{id: id, name: name, peer: p, reply: reply}) != nil {
		return false
	}
	select {
	case result := <-reply:
		return result
	case <-ctx.Done():
		return false
	case <-s.done:
		return false
	}
}

func (s *Server) apply(ctx context.Context, command playerCommand) error {
	if err := s.submit(ctx, command); err != nil {
		return err
	}
	select {
	case err := <-command.reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return errClosed
	}
}

func (s *Server) layout(ctx context.Context) (game.Layout, error) {
	reply := make(chan game.Layout, 1)
	if err := s.submit(ctx, layoutCommand{reply: reply}); err != nil {
		return game.Layout{}, err
	}
	select {
	case layout := <-reply:
		return layout, nil
	case <-ctx.Done():
		return game.Layout{}, ctx.Err()
	case <-s.done:
		return game.Layout{}, errClosed
	}
}

func (s *Server) inspect(ctx context.Context, id string) (ownerView, error) {
	reply := make(chan ownerView, 1)
	if err := s.submit(ctx, inspectCommand{id: id, reply: reply}); err != nil {
		return ownerView{}, err
	}
	select {
	case view := <-reply:
		return view, nil
	case <-ctx.Done():
		return ownerView{}, ctx.Err()
	case <-s.done:
		return ownerView{}, errClosed
	}
}

// Run controls simulation ticking; the owner remains available for API queries,
// final Save, and Close both before Run starts and after its context ends.
func (s *Server) Run(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	reply := make(chan struct{}, 1)
	if s.submit(ctx, runCommand{start: true, reply: reply}) != nil {
		return
	}
	select {
	case <-reply:
	case <-s.done:
		return
	}
	select {
	case <-ctx.Done():
	case <-s.done:
		return
	}
	reply = make(chan struct{}, 1)
	if s.submit(context.Background(), runCommand{reply: reply}) != nil {
		return
	}
	select {
	case <-reply:
	case <-s.done:
	}
}

// Save waits for an ordered persistence barrier. At most eight explicit saves
// can be pending; waiting callers never consume space in the world mailbox.
func (s *Server) Save() error {
	select {
	case s.saveSlots <- struct{}{}:
	case <-s.done:
		return errClosed
	}
	defer func() { <-s.saveSlots }()
	reply := make(chan error, 1)
	if err := s.submit(context.Background(), saveCommand{reply: reply}); err != nil {
		return err
	}
	select {
	case err := <-reply:
		return err
	case <-s.done:
		select {
		case err := <-reply:
			return err
		default:
			return errClosed
		}
	}
}

// Close is an idempotent final barrier. No older asynchronous save can follow
// it, and both the owner and the sole disk worker have stopped when it returns.
func (s *Server) Close() error {
	select {
	case s.closeStarted <- struct{}{}:
		s.submit(context.Background(), closeCommand{})
	default:
	}
	<-s.done
	return s.closeErr
}

func (c *coordinator) ownWorld() {
	defer close(c.server.done)
	defer c.stopTicker()
	for {
		// Check the clock before taking another command. A busy mailbox cannot
		// win repeated random selects and starve simulation ticks.
		select {
		case now := <-c.ticks:
			c.tick(now)
		default:
		}
		select {
		case now := <-c.ticks:
			c.tick(now)
		case command := <-c.server.commands:
			c.handle(command)
		case err := <-c.server.finalSaved:
			c.server.closeErr = err
			<-c.server.saveDone
			return
		}
	}
}

func (c *coordinator) stopTicker() {
	if c.ticker != nil {
		c.ticker.Stop()
		c.ticker = nil
		c.ticks = nil
	}
}

func (c *coordinator) tick(now time.Time) {
	c.expireSuspended(now)
	// Ticker timestamps jitter by nanoseconds. Accumulate elapsed time into
	// exact simulation steps rather than invoking Tick(.050000001), which
	// would perform an entire second world update for the trailing nanosecond.
	const clockTolerance = time.Microsecond
	elapsed := now.Sub(c.lastTick) + c.tickRemainder
	steps := max(0, int((elapsed+clockTolerance)/c.timing.tick))
	c.tickRemainder = elapsed - time.Duration(steps)*c.timing.tick
	for i := 0; i < min(steps, max(1, int(time.Second/c.timing.tick))); i++ {
		c.world.Tick(c.timing.tick.Seconds())
	}
	c.lastTick = now
	clockNow := now.Add(clockTolerance)
	if !clockNow.Before(c.nextSnapshot) {
		for id, p := range c.peers {
			c.enqueueSnapshot(id, p)
		}
		c.nextSnapshot = advanceDeadline(c.nextSnapshot, clockNow, c.timing.snapshot)
	}
	if !clockNow.Before(c.nextSave) {
		c.queueSave(nil, false)
		c.nextSave = advanceDeadline(c.nextSave, clockNow, c.timing.autosave)
	}
}

func (c *coordinator) expireSuspended(now time.Time) {
	for id, session := range c.suspended {
		if now.Before(session.until) {
			continue
		}
		delete(c.suspended, id)
		if c.peers[id] == nil {
			c.world.Leave(id)
		}
	}
}

func advanceDeadline(previous, now time.Time, interval time.Duration) time.Time {
	return previous.Add((now.Sub(previous)/interval + 1) * interval)
}

func (c *coordinator) enqueueSnapshot(id string, p *peer) {
	// A dropped first frame still needs its layout until a writer confirms a
	// successfully delivered baseline. Steady-state frames avoid layout copies.
	select {
	case <-p.baselineReady:
		queueSnapshot(p, c.world.SnapshotUpdate(id))
	default:
		queueSnapshot(p, c.world.Snapshot(id))
	}
}

func (c *coordinator) handle(raw ownerCommand) {
	switch command := raw.(type) {
	case capacityCommand:
		command.reply <- !c.closing && (len(c.peers) < maxPlayers || c.peers[command.id] != nil)
	case attachCommand:
		if c.closing || len(c.peers) >= maxPlayers && c.peers[command.id] == nil {
			command.reply <- false
			return
		}
		now := time.Now()
		resumes := false
		if session, suspended := c.suspended[command.id]; suspended {
			delete(c.suspended, command.id)
			resumes = command.peer.resumeID != "" && session.resumeID == command.peer.resumeID && now.Before(session.until)
			if !resumes {
				c.world.Leave(command.id)
			}
		}
		if old := c.peers[command.id]; old != nil {
			// A reconnect can win the race with the previous socket's detach.
			// A matching resume identifier keeps the runtime; another client
			// still receives a fresh session and replaces the old socket.
			if command.peer.resumeID == "" || old.resumeID != command.peer.resumeID {
				c.world.Leave(command.id)
			}
			go func() {
				if old.conn != nil {
					old.conn.Close(websocket.StatusCode(4001), "This account was opened in another tab")
				}
				if old.cancel != nil {
					old.cancel()
				}
			}()
		}
		c.peers[command.id] = command.peer
		c.world.Join(command.id, command.name)
		// Every physical connection starts with a complete keyframe. This
		// resets the delta stream without resetting the retained player session.
		c.enqueueSnapshot(command.id, command.peer)
		command.reply <- true
	case detachCommand:
		if c.peers[command.id] == command.peer && !c.closing {
			delete(c.peers, command.id)
			if command.resume && command.peer.resumeID != "" {
				c.world.Disconnect(command.id)
				c.suspended[command.id] = suspendedSession{resumeID: command.peer.resumeID, until: time.Now().Add(reconnectGrace)}
			} else {
				delete(c.suspended, command.id)
				// Explicit departures still save and release runtime immediately.
				c.world.Leave(command.id)
			}
		}
	case playerCommand:
		var err error
		switch {
		case c.closing:
			err = errClosed
		case c.peers[command.id] != command.peer:
			err = errReplaced
		case command.input != nil:
			c.world.Input(command.id, *command.input)
		case command.action != nil:
			err = c.world.Act(command.id, *command.action)
		}
		command.reply <- err
	case layoutCommand:
		command.reply <- c.world.StaticLayout()
	case inspectCommand:
		command.reply <- ownerView{snapshot: c.world.Snapshot(command.id), players: len(c.peers), closing: c.closing}
	case runCommand:
		if !command.start {
			c.stopTicker()
		} else if !c.closing && c.ticker == nil {
			c.lastTick = time.Now()
			c.tickRemainder = 0
			c.nextSnapshot = c.lastTick.Add(c.timing.snapshot)
			c.nextSave = c.lastTick.Add(c.timing.autosave)
			c.ticker = time.NewTicker(c.timing.tick)
			c.ticks = c.ticker.C
		}
		command.reply <- struct{}{}
	case saveCommand:
		if c.closing {
			command.reply <- errClosed
		} else {
			c.queueSave(command.reply, false)
		}
	case closeCommand:
		if c.closing {
			return
		}
		c.closing = true
		c.stopTicker()
		for _, p := range c.peers {
			if p.cancel != nil {
				p.cancel()
			}
			if p.conn != nil {
				go p.conn.CloseNow()
			}
		}
		c.queueSave(nil, true)
	}
}

// One pending immutable snapshot replaces older pending work, carrying every
// explicit caller's barrier forward. Only the owner sends into this channel;
// the worker may consume concurrently, so replacement never blocks the world.
func (c *coordinator) queueSave(waiter chan error, final bool) {
	job := saveJob{profiles: c.world.Profiles(), final: final}
	if waiter != nil {
		job.waiters = []chan error{waiter}
	}
	select {
	case c.server.saves <- job:
		return
	default:
	}
	select {
	case previous := <-c.server.saves:
		job.waiters = append(previous.waiters, job.waiters...)
	default:
	}
	c.server.saves <- job
}

func (s *Server) saveWorker(persist func(map[string]game.Profile) error) {
	defer close(s.saveDone)
	for job := range s.saves {
		err := persist(job.profiles)
		if err != nil {
			s.logger.Error("save progress", "error", err)
		}
		for _, reply := range job.waiters {
			reply <- err
		}
		if job.final {
			s.finalSaved <- err
			return
		}
	}
}
