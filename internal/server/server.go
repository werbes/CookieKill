// Package server binds authenticated browser connections to one authoritative world.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cookiekill/internal/auth"
	"cookiekill/internal/game"
	"cookiekill/internal/store"
	"github.com/coder/websocket"
)

const maxPlayers = 64

type peer struct {
	conn       *websocket.Conn
	out        chan []byte
	cancel     context.CancelFunc
	layoutSent atomic.Bool
}

type Server struct {
	mu       sync.Mutex
	world    *game.World
	auth     *auth.Service
	store    *store.Store
	peers    map[string]*peer
	departed map[string]time.Time
	closing  bool
	logger   *slog.Logger
}

func New(a *auth.Service, db *store.Store, profiles map[string]game.Profile, logger *slog.Logger) *Server {
	return &Server{auth: a, store: db, world: game.New(profiles), peers: make(map[string]*peer), departed: make(map[string]time.Time), logger: logger}
}

func (s *Server) Handler(assets fs.FS, secure bool) http.Handler {
	mux := http.NewServeMux()
	s.auth.Register(mux)
	mux.HandleFunc("GET /ws", s.connect)
	mux.HandleFunc("GET /api/world", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		layout := s.world.StaticLayout()
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"layout": layout})
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	files := http.FileServerFS(assets)
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(r.URL.Path, "/vendor/") {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A loopback bind alone does not stop a website using DNS rebinding to
		// reach local development codes under an attacker-controlled hostname.
		if !secure && !localHost(r.Host) {
			http.Error(w, "Local hostname required", http.StatusMisdirectedRequest)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		if secure {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) connect(w http.ResponseWriter, r *http.Request) {
	p, ok := s.auth.Authenticate(r)
	if !ok {
		http.Error(w, "Sign in to enter the world", http.StatusUnauthorized)
		return
	}
	// Browser websocket upgrades must be same origin, including the scheme.
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if err != nil || !strings.EqualFold(u.Host, r.Host) || u.Scheme != scheme || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			http.Error(w, "Origin not allowed", http.StatusForbidden)
			return
		}
	}
	s.mu.Lock()
	if s.closing || (len(s.peers) >= maxPlayers && s.peers[p.ID] == nil) {
		s.mu.Unlock()
		http.Error(w, "World full; try again shortly", http.StatusServiceUnavailable)
		return
	}
	s.mu.Unlock()
	// The network handshake must not hold the simulation mutex.
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	s.mu.Lock()
	// Capacity may have changed while the network handshake was in progress.
	if s.closing || (len(s.peers) >= maxPlayers && s.peers[p.ID] == nil) {
		s.mu.Unlock()
		c.CloseNow()
		return
	}
	// net/http clears its deadlines on hijack. These loops own the connection's
	// lifetime and set their own deadlines independently of the HTTP request.
	ctx, cancel := context.WithCancel(context.Background())
	client := &peer{conn: c, out: make(chan []byte, 2), cancel: cancel}
	if old := s.peers[p.ID]; old != nil {
		go func() {
			old.conn.Close(websocket.StatusCode(4001), "This account was opened in another tab")
			old.cancel()
		}()
	}
	s.peers[p.ID] = client
	delete(s.departed, p.ID)
	s.world.Join(p.ID, p.Name)
	s.enqueueSnapshot(p.ID, client)
	s.mu.Unlock()
	c.SetReadLimit(2048)
	defer func() {
		cancel()
		c.CloseNow()
		s.mu.Lock()
		if s.peers[p.ID] == client {
			delete(s.peers, p.ID)
			s.world.Input(p.ID, game.Input{})
			// Keep the player in the world briefly so disconnecting cannot evade a hit.
			s.departed[p.ID] = time.Now().Add(10 * time.Second)
		}
		s.mu.Unlock()
	}()
	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case b := <-client.out:
				writeCtx, done := context.WithTimeout(ctx, 5*time.Second)
				err := c.Write(writeCtx, websocket.MessageText, b)
				done()
				if err != nil {
					return
				}
				// Every snapshot queued before this first successful write carries
				// the map, so dropping a stale frame cannot lose initial scenery.
				if len(b) > 20 && !client.layoutSent.Load() {
					var frame struct {
						Layout *game.Layout `json:"layout"`
					}
					if json.Unmarshal(b, &frame) == nil && frame.Layout != nil {
						client.layoutSent.Store(true)
					}
				}
			}
		}
	}()
	tokens := 80.0
	actionTokens := 12.0
	last := time.Now()
	for {
		readCtx, done := context.WithTimeout(ctx, 90*time.Second)
		kind, b, err := c.Read(readCtx)
		done()
		if err != nil {
			return
		}
		now := time.Now()
		elapsed := now.Sub(last).Seconds()
		last = now
		tokens = math.Min(80, tokens+elapsed*40)
		actionTokens = math.Min(12, actionTokens+elapsed*8)
		if tokens < 1 {
			cancel()
			return
		}
		tokens--
		if kind != websocket.MessageText {
			s.sendError(client, "Use JSON messages")
			continue
		}
		var envelope struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(b, &envelope) != nil {
			s.sendError(client, "Invalid message")
			continue
		}
		if _, valid := s.auth.Authenticate(r); !valid {
			c.Close(websocket.StatusPolicyViolation, "Your session ended; sign in again")
			return
		}
		s.mu.Lock()
		if s.peers[p.ID] != client {
			s.mu.Unlock()
			c.Close(websocket.StatusCode(4001), "This account was opened in another tab")
			return
		}
		if s.closing {
			s.mu.Unlock()
			return
		}
		switch envelope.Type {
		case "input":
			var input game.Input
			if err = json.Unmarshal(b, &input); err == nil {
				if finite(input.X, input.Z, input.Yaw, input.Pitch) {
					s.world.Input(p.ID, input)
				} else {
					err = errors.New("Invalid movement")
				}
			}
		case "action":
			if actionTokens < 1 {
				err = errors.New("Slow down a little")
				break
			}
			actionTokens--
			var action game.Action
			if err = json.Unmarshal(b, &action); err == nil {
				err = s.world.Act(p.ID, action)
			}
		case "ping":
		default:
			err = errors.New("Unknown message type")
		}
		s.mu.Unlock()
		if err != nil {
			s.sendError(client, err.Error())
		}
	}
}

func localHost(host string) bool {
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func finite(values ...float64) bool {
	for _, n := range values {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return false
		}
	}
	return true
}

func (s *Server) sendError(p *peer, message string) {
	b, _ := json.Marshal(struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}{"error", message})
	select {
	case p.out <- b:
	default:
	}
}

// Called with mu held; queues are bounded and never stall the simulation.
func (s *Server) enqueueSnapshot(id string, p *peer) {
	snapshot := s.world.Snapshot(id)
	if p.layoutSent.Load() {
		snapshot.Layout = nil
	}
	value := struct {
		Type string `json:"type"`
		game.Snapshot
	}{"snapshot", snapshot}
	b, err := json.Marshal(value)
	if err != nil {
		s.logger.Error("encode world", "error", err)
		return
	}
	select {
	case p.out <- b:
	default:
		select {
		case <-p.out:
		default:
		}
		select {
		case p.out <- b:
		default:
		}
	}
}

func (s *Server) Run(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	saver := time.NewTicker(5 * time.Second)
	defer saver.Stop()
	ticks := 0
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.mu.Lock()
			s.world.Tick(.05)
			for id, deadline := range s.departed {
				if !now.Before(deadline) {
					s.world.Leave(id)
					delete(s.departed, id)
				}
			}
			ticks++
			if ticks%2 == 0 {
				for id, p := range s.peers {
					s.enqueueSnapshot(id, p)
				}
			}
			s.mu.Unlock()
		case <-saver.C:
			if err := s.Save(); err != nil {
				s.logger.Error("save progress", "error", err)
			}
		}
	}
}

func (s *Server) Save() error {
	s.mu.Lock()
	profiles := s.world.Profiles()
	s.mu.Unlock()
	return s.store.Save(profiles)
}

func (s *Server) Close() error {
	s.mu.Lock()
	s.closing = true
	for _, p := range s.peers {
		p.cancel()
		go p.conn.CloseNow()
	}
	s.mu.Unlock()
	return s.Save()
}
