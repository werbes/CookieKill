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
	"time"

	"cookiekill/internal/auth"
	"cookiekill/internal/game"
	"cookiekill/internal/store"
	"github.com/coder/websocket"
)

const maxPlayers = 64

type peer struct {
	conn          *websocket.Conn
	out           chan outbound
	cancel        context.CancelFunc
	baselineReady chan struct{} // Writer closes after the first successful snapshot.
}

// Queue complete, independent snapshots. Only the connection writer computes
// deltas, so dropping a queued snapshot cannot invalidate the client's baseline.
type outbound struct {
	snapshot *game.Snapshot
	message  []byte
}

type Server struct {
	auth         *auth.Service
	store        *store.Store
	logger       *slog.Logger
	commands     chan ownerCommand
	done         chan struct{}
	closeStarted chan struct{}
	saveSlots    chan struct{}
	saves        chan saveJob
	finalSaved   chan error
	saveDone     chan struct{}
	closeErr     error // Published by closing done; read only after done closes.
}

func New(a *auth.Service, db *store.Store, profiles map[string]game.Profile, logger *slog.Logger) *Server {
	return newServer(a, db, profiles, logger, db.Save, serverTiming{50 * time.Millisecond, 100 * time.Millisecond, 5 * time.Second})
}

func (s *Server) Handler(assets fs.FS, secure bool) http.Handler {
	mux := http.NewServeMux()
	s.auth.Register(mux)
	mux.HandleFunc("GET /ws", s.connect)
	mux.HandleFunc("GET /api/world", func(w http.ResponseWriter, r *http.Request) {
		layout, err := s.layout(r.Context())
		if err != nil {
			http.Error(w, "World is shutting down", http.StatusServiceUnavailable)
			return
		}
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
	if !s.capacity(r.Context(), p.ID) {
		http.Error(w, "World full; try again shortly", http.StatusServiceUnavailable)
		return
	}
	// Handshakes are network work; neither admission check blocks the world.
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionNoContextTakeover})
	if err != nil {
		return
	}
	// net/http clears its deadlines on hijack. These loops own the connection's
	// lifetime and set their own deadlines independently of the HTTP request.
	ctx, cancel := context.WithCancel(context.Background())
	client := &peer{conn: c, out: make(chan outbound, 2), cancel: cancel, baselineReady: make(chan struct{})}
	// Capacity may have changed while the network handshake was in progress.
	if !s.attach(ctx, p.ID, p.Name, client) {
		cancel()
		c.CloseNow()
		return
	}
	c.SetReadLimit(2048)
	defer func() {
		cancel()
		c.CloseNow()
		s.submit(context.Background(), detachCommand{id: p.ID, peer: client})
	}()
	go func() {
		defer cancel()
		// Old tabs keep receiving full snapshots until reloaded. Each new
		// connection starts with a full baseline, including the world layout.
		encoder := stateEncoder{delta: r.URL.Query().Get("updates") == "delta-v1"}
		encoder.animalPaths = encoder.delta && r.URL.Query().Get("animals") == "paths-v1"
		baselineWritten := false
		for {
			select {
			case <-ctx.Done():
				return
			case next := <-client.out:
				b := next.message
				if next.snapshot != nil {
					var err error
					b, err = encoder.encode(*next.snapshot)
					if err != nil {
						s.logger.Error("encode world", "error", err)
						return
					}
				}
				writeCtx, done := context.WithTimeout(ctx, 5*time.Second)
				err := c.Write(writeCtx, websocket.MessageText, b)
				done()
				if err != nil {
					return
				}
				if next.snapshot != nil {
					encoder.commit(*next.snapshot)
					if !baselineWritten {
						baselineWritten = true
						close(client.baselineReady)
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
		command := playerCommand{id: p.ID, peer: client, reply: make(chan error, 1)}
		switch envelope.Type {
		case "input":
			var input game.Input
			if err = json.Unmarshal(b, &input); err == nil {
				if finite(input.X, input.Z, input.Yaw, input.Pitch) {
					command.input = &input
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
				command.action = &action
			}
		case "ping":
		default:
			err = errors.New("Unknown message type")
		}
		if err == nil {
			// Await each command before reading the next packet: bounded pressure
			// preserves press/release edges and action ordering for every player.
			err = s.apply(ctx, command)
		}
		if errors.Is(err, errReplaced) {
			c.Close(websocket.StatusCode(4001), "This account was opened in another tab")
			return
		}
		if errors.Is(err, errClosed) || ctx.Err() != nil {
			return
		}
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
	case p.out <- outbound{message: b}:
	default:
	}
}

// Only complete independent snapshots enter the bounded connection queue.
func queueSnapshot(p *peer, snapshot game.Snapshot) {
	next := outbound{snapshot: &snapshot}
	select {
	case p.out <- next:
	default:
		select {
		case <-p.out:
		default:
		}
		select {
		case p.out <- next:
		default:
		}
	}
}
