// Package auth provides passwordless email login and opaque, expiring sessions.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	cookieName       = "cookiekill_session"
	challengeTTL     = 10 * time.Minute
	maxCodeAttempts  = 5
	maxPendingCodes  = 10000
	maxSessions      = 20000
	maxRateLimitKeys = 40000
)

// Config requires SMTP in production. Port 465 uses implicit TLS; all other
// ports require STARTTLS. Development codes are only returned to loopback peers.
type Config struct {
	DevMode       bool
	SecureCookies bool
	SMTPHost      string
	SMTPPort      int
	SMTPUser      string
	SMTPPass      string
	SMTPFrom      string
	SessionTTL    time.Duration
}

// Player deliberately contains no email address.
type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type challenge struct {
	digest   [32]byte
	salt     string
	expires  time.Time
	attempts int
}

type session struct {
	player  Player
	expires time.Time
}

type rateLimit struct {
	count   int
	expires time.Time
}

type Service struct {
	mu         sync.Mutex
	cfg        Config
	now        func() time.Time
	send       func(email, code string) error
	challenges map[string]challenge
	sessions   map[[32]byte]session
	limits     map[string]rateLimit
}

func New(cfg Config) (*Service, error) {
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 7 * 24 * time.Hour
	}
	if cfg.SessionTTL < time.Minute || cfg.SessionTTL > 30*24*time.Hour {
		return nil, errors.New("session lifetime must be between one minute and 30 days")
	}
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 587
	}
	if !cfg.DevMode {
		if !cfg.SecureCookies {
			return nil, errors.New("production login requires secure cookies and HTTPS")
		}
		if cfg.SMTPHost == "" || strings.ContainsAny(cfg.SMTPHost, "\r\n /\\") || cfg.SMTPPort < 1 || cfg.SMTPPort > 65535 {
			return nil, errors.New("production login requires a valid SMTP host and port")
		}
		from, err := mail.ParseAddress(cfg.SMTPFrom)
		if err != nil || strings.ContainsAny(cfg.SMTPFrom, "\r\n") {
			return nil, errors.New("production login requires a valid SMTP from address")
		}
		cfg.SMTPFrom = from.String()
		if (cfg.SMTPUser == "") != (cfg.SMTPPass == "") {
			return nil, errors.New("SMTP username and password must be configured together")
		}
	}
	s := &Service{
		cfg: cfg, now: time.Now,
		challenges: make(map[string]challenge),
		sessions:   make(map[[32]byte]session),
		limits:     make(map[string]rateLimit),
	}
	s.send = func(email, code string) error { return sendCode(cfg, email, code) }
	return s, nil
}

func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/session", s.session)
	mux.HandleFunc("POST /api/login", s.guard(s.login))
	mux.HandleFunc("POST /api/verify", s.guard(s.verify))
	mux.HandleFunc("POST /api/logout", s.guard(s.logout))
}

// Authenticate validates the opaque cookie against its server-side session.
func (s *Service) Authenticate(r *http.Request) (Player, bool) {
	digest, ok := sessionDigest(r)
	if !ok {
		return Player{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[digest]
	if !ok {
		return Player{}, false
	}
	if !s.now().Before(sess.expires) {
		delete(s.sessions, digest)
		return Player{}, false
	}
	return sess.player, true
}

func (s *Service) session(w http.ResponseWriter, r *http.Request) {
	p, ok := s.Authenticate(r)
	response := map[string]any{"authenticated": ok, "devMode": s.cfg.DevMode}
	if ok {
		response["player"] = p
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DevMode && !loopbackPeer(r) {
		writeError(w, http.StatusForbidden, "Development login is only available locally.")
		return
	}
	var request struct {
		Email string `json:"email"`
	}
	if !readJSON(w, r, &request) {
		return
	}
	email, ok := normalizeEmail(request.Email)
	if !ok {
		writeError(w, http.StatusBadRequest, "Enter a valid email address.")
		return
	}
	codeNumber, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Login is temporarily unavailable.")
		return
	}
	code := fmt.Sprintf("%06d", codeNumber.Int64())
	salt, err := randomToken()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Login is temporarily unavailable.")
		return
	}
	now := s.now()
	c := challenge{digest: codeDigest(email, code, salt), salt: salt, expires: now.Add(challengeTTL)}
	s.mu.Lock()
	s.pruneLocked(now)
	allowedIP := s.allowLocked("login-ip:"+peerIP(r), 10, 15*time.Minute, now)
	allowedEmail := s.allowLocked("login-email:"+email, 1, time.Minute, now)
	if !allowedIP || !allowedEmail || len(s.challenges) >= maxPendingCodes {
		s.mu.Unlock()
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "Please wait before requesting another code.")
		return
	}
	s.challenges[email] = c
	s.mu.Unlock()
	if !s.cfg.DevMode {
		if err := s.send(email, code); err != nil {
			s.mu.Lock()
			if current, exists := s.challenges[email]; exists && current.digest == c.digest {
				delete(s.challenges, email)
			}
			s.mu.Unlock()
			writeError(w, http.StatusServiceUnavailable, "Email delivery is temporarily unavailable. Please try again later.")
			return
		}
	}
	response := map[string]any{"message": "If the address can receive email, your sign-in code is on its way. It expires in 10 minutes."}
	if s.cfg.DevMode {
		response["devCode"] = code
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Service) verify(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DevMode && !loopbackPeer(r) {
		writeError(w, http.StatusForbidden, "Development login is only available locally.")
		return
	}
	var request struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !readJSON(w, r, &request) {
		return
	}
	email, ok := normalizeEmail(request.Email)
	if !ok {
		writeError(w, http.StatusUnauthorized, "That code is invalid or expired.")
		return
	}
	token, err := randomToken()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Login is temporarily unavailable.")
		return
	}
	now := s.now()
	s.mu.Lock()
	s.pruneLocked(now)
	allowedIP := s.allowLocked("verify-ip:"+peerIP(r), 40, 15*time.Minute, now)
	allowedEmail := s.allowLocked("verify-email:"+email, 20, 15*time.Minute, now)
	if !allowedIP || !allowedEmail {
		s.mu.Unlock()
		w.Header().Set("Retry-After", "900")
		writeError(w, http.StatusTooManyRequests, "Too many attempts. Please try again later.")
		return
	}
	c, exists := s.challenges[email]
	if !exists || !now.Before(c.expires) || c.attempts >= maxCodeAttempts {
		delete(s.challenges, email)
		s.mu.Unlock()
		writeError(w, http.StatusUnauthorized, "That code is invalid or expired.")
		return
	}
	digest := codeDigest(email, strings.TrimSpace(request.Code), c.salt)
	if subtle.ConstantTimeCompare(digest[:], c.digest[:]) != 1 {
		c.attempts++
		if c.attempts >= maxCodeAttempts {
			delete(s.challenges, email)
		} else {
			s.challenges[email] = c
		}
		s.mu.Unlock()
		writeError(w, http.StatusUnauthorized, "That code is invalid or expired.")
		return
	}
	if len(s.sessions) >= maxSessions {
		s.mu.Unlock()
		writeError(w, http.StatusServiceUnavailable, "Login is temporarily unavailable.")
		return
	}
	delete(s.challenges, email)
	// A fresh login rotates this browser's session instead of accumulating them.
	if previous, ok := sessionDigest(r); ok {
		delete(s.sessions, previous)
	}
	p := playerForEmail(email)
	expires := now.Add(s.cfg.SessionTTL)
	s.sessions[sha256.Sum256([]byte(token))] = session{player: p, expires: expires}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: int(s.cfg.SessionTTL.Seconds())})
	writeJSON(w, http.StatusOK, map[string]any{"player": p})
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	if digest, ok := sessionDigest(r); ok {
		s.mu.Lock()
		delete(s.sessions, digest)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteStrictMode, Expires: time.Unix(1, 0), MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" || r.Header.Get("Sec-Fetch-Site") == "same-site" {
			writeError(w, http.StatusForbidden, "A same-origin request is required.")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			scheme := "http"
			if r.TLS != nil || s.cfg.SecureCookies {
				scheme = "https"
			}
			if err != nil || u.Scheme != scheme || !strings.EqualFold(u.Host, r.Host) || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
				writeError(w, http.StatusForbidden, "A same-origin request is required.")
				return
			}
		}
		contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || contentType != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "Use application/json.")
			return
		}
		next(w, r)
	}
}

func (s *Service) allowLocked(key string, maximum int, period time.Duration, now time.Time) bool {
	limit, exists := s.limits[key]
	if !exists && len(s.limits) >= maxRateLimitKeys {
		return false
	}
	if !now.Before(limit.expires) {
		limit = rateLimit{expires: now.Add(period)}
	}
	if limit.count >= maximum {
		return false
	}
	limit.count++
	s.limits[key] = limit
	return true
}

func (s *Service) pruneLocked(now time.Time) {
	for key, c := range s.challenges {
		if !now.Before(c.expires) {
			delete(s.challenges, key)
		}
	}
	for key, sess := range s.sessions {
		if !now.Before(sess.expires) {
			delete(s.sessions, key)
		}
	}
	for key, limit := range s.limits {
		if !now.Before(limit.expires) {
			delete(s.limits, key)
		}
	}
}

func readJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "Invalid request.")
		return false
	}
	return true
}

func normalizeEmail(input string) (string, bool) {
	if strings.ContainsAny(input, "\r\n") {
		return "", false
	}
	email := strings.ToLower(strings.TrimSpace(input))
	if len(email) > 254 {
		return "", false
	}
	for _, character := range email {
		if character > 127 {
			return "", false
		}
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || address.Name != "" {
		return "", false
	}
	local, domain, ok := strings.Cut(email, "@")
	if !ok || len(local) == 0 || len(local) > 64 || !strings.Contains(domain, ".") {
		return "", false
	}
	return email, true
}

func playerForEmail(email string) Player {
	digest := sha256.Sum256([]byte(email))
	local, _, _ := strings.Cut(email, "@")
	var name strings.Builder
	for _, c := range local {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-' {
			name.WriteRune(c)
			if name.Len() == 18 {
				break
			}
		}
	}
	if name.Len() == 0 {
		name.WriteString("Baker")
	}
	return Player{ID: "u_" + hex.EncodeToString(digest[:16]), Name: name.String()}
}

func peerIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if parsed := net.ParseIP(host); parsed != nil {
		return parsed.String()
	}
	return "unknown"
}

func loopbackPeer(r *http.Request) bool {
	ip := net.ParseIP(peerIP(r))
	return ip != nil && ip.IsLoopback()
}

func codeDigest(email, code, salt string) [32]byte {
	return sha256.Sum256([]byte(email + "\x00" + code + "\x00" + salt))
}

func randomToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token[:]), nil
}

func sessionDigest(r *http.Request) ([32]byte, bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil || len(cookie.Value) != 43 {
		return [32]byte{}, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil || len(decoded) != 32 {
		return [32]byte{}, false
	}
	return sha256.Sum256([]byte(cookie.Value)), true
}

func writeJSON(w http.ResponseWriter, status int, response any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
