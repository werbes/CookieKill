package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func rememberedService(t *testing.T, cfg Config) (*Service, *http.ServeMux) {
	t.Helper()
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.Register(mux)
	return s, mux
}

func verifyRemembered(mux *http.ServeMux, email, code string, remember bool, cookie *http.Cookie) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]any{"email": email, "code": code, "keepSignedIn": remember})
	return request(mux, http.MethodPost, "/api/verify", string(body), cookie)
}

func requireAuthenticated(t *testing.T, mux *http.ServeMux, cookie *http.Cookie, want bool) {
	t.Helper()
	w := request(mux, http.MethodGet, "/api/session", "", cookie)
	var response struct {
		Authenticated bool `json:"authenticated"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || w.Code != http.StatusOK || response.Authenticated != want {
		t.Fatalf("session = %d %s, want authenticated %v (decode: %v)", w.Code, w.Body.String(), want, err)
	}
}

func TestRememberedSessionSurvivesRestartAndLogout(t *testing.T) {
	cfg := Config{DevMode: true, SecureCookies: true, SessionTTL: time.Hour, SessionFile: filepath.Join(t.TempDir(), "sessions.json")}
	s, mux := rememberedService(t, cfg)
	now := time.Now().UTC().Truncate(time.Second)
	s.now = func() time.Time { return now }
	email := "remember@example.com"
	w := verifyRemembered(mux, email, loginCode(t, mux, email), true, nil)
	if w.Code != http.StatusOK || len(w.Result().Cookies()) != 1 {
		t.Fatalf("verify: %d %s", w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.MaxAge != int(rememberedSessionTTL.Seconds()) || !cookie.Expires.Equal(now.Add(rememberedSessionTTL)) {
		t.Fatalf("incorrect remembered cookie: %+v", cookie)
	}
	contents, err := os.ReadFile(cfg.SessionFile)
	if err != nil || strings.Contains(string(contents), cookie.Value) || strings.Contains(string(contents), email) {
		t.Fatalf("session storage must exist without bearer token or email (read error: %v)", err)
	}
	// Choosing not to remember another browser must not persist that session.
	ordinaryEmail := "ordinary@example.com"
	ordinary := verifyRemembered(mux, ordinaryEmail, loginCode(t, mux, ordinaryEmail), false, nil)
	if ordinary.Code != http.StatusOK {
		t.Fatalf("ordinary verify: %s", ordinary.Body.String())
	}
	ordinaryCookie := ordinary.Result().Cookies()[0]
	if ordinaryCookie.MaxAge != 0 || !ordinaryCookie.Expires.IsZero() {
		t.Fatalf("ordinary session must not set a persistent cookie: %+v", ordinaryCookie)
	}
	requireAuthenticated(t, mux, ordinaryCookie, true)
	_, restarted := rememberedService(t, cfg)
	requireAuthenticated(t, restarted, cookie, true)
	requireAuthenticated(t, restarted, ordinaryCookie, false)
	w = request(restarted, http.MethodPost, "/api/logout", "{}", cookie)
	if w.Code != http.StatusOK || len(w.Result().Cookies()) != 1 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("logout: %d %s", w.Code, w.Body.String())
	}
	requireAuthenticated(t, restarted, cookie, false)
	_, afterLogout := rememberedService(t, cfg)
	requireAuthenticated(t, afterLogout, cookie, false)
}

func TestRememberedSessionStillRequiresEmailVerificationAndExpires(t *testing.T) {
	s, mux, now := testService(t)
	email := "remember@example.com"
	code := loginCode(t, mux, email)
	w := verifyRemembered(mux, email, "wrong", true, nil)
	if w.Code != http.StatusUnauthorized || len(w.Result().Cookies()) != 0 || len(s.sessions) != 0 {
		t.Fatal("remember option bypassed email verification")
	}
	w = verifyRemembered(mux, email, code, true, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("verify: %s", w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	if w := verifyRemembered(mux, email, code, true, nil); w.Code != http.StatusUnauthorized {
		t.Fatal("remember option allowed code replay")
	}
	*now = now.Add(rememberedSessionTTL - time.Second)
	requireAuthenticated(t, mux, cookie, true)
	*now = now.Add(time.Second)
	requireAuthenticated(t, mux, cookie, false)
}

func TestRememberedSessionRotationAndOptOut(t *testing.T) {
	for _, remember := range []bool{true, false} {
		name := "remember again"
		if !remember {
			name = "opt out"
		}
		t.Run(name, func(t *testing.T) {
			cfg := Config{DevMode: true, SessionFile: filepath.Join(t.TempDir(), "sessions.json")}
			s, mux := rememberedService(t, cfg)
			now := time.Now()
			s.now = func() time.Time { return now }
			email := "remember@example.com"
			w := verifyRemembered(mux, email, loginCode(t, mux, email), true, nil)
			if w.Code != http.StatusOK {
				t.Fatalf("initial verify: %s", w.Body.String())
			}
			previous := w.Result().Cookies()[0]
			now = now.Add(time.Minute)
			w = verifyRemembered(mux, email, loginCode(t, mux, email), remember, previous)
			if w.Code != http.StatusOK {
				t.Fatalf("rotation: %s", w.Body.String())
			}
			current := w.Result().Cookies()[0]
			if current.Value == previous.Value || len(s.sessions) != 1 {
				t.Fatal("rotation must replace the old token")
			}
			requireAuthenticated(t, mux, previous, false)
			requireAuthenticated(t, mux, current, true)
			_, restarted := rememberedService(t, cfg)
			requireAuthenticated(t, restarted, previous, false)
			requireAuthenticated(t, restarted, current, remember)
		})
	}
}

func TestRememberedStorageFailureAllowsSafeRetry(t *testing.T) {
	cfg := Config{DevMode: true, SessionFile: filepath.Join(t.TempDir(), "sessions.json")}
	s, mux := rememberedService(t, cfg)
	email := "remember@example.com"
	w := verifyRemembered(mux, email, loginCode(t, mux, email), true, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("initial verify: %s", w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	// Renaming a save over a directory fails on all supported platforms.
	s.cfg.SessionFile = t.TempDir()
	w = request(mux, http.MethodPost, "/api/logout", "{}", cookie)
	if w.Code != http.StatusServiceUnavailable || len(w.Result().Cookies()) != 0 {
		t.Fatal("failed durable revocation must not report success or clear the cookie")
	}
	requireAuthenticated(t, mux, cookie, true)
	otherEmail := "another@example.com"
	code := loginCode(t, mux, otherEmail)
	w = verifyRemembered(mux, otherEmail, code, true, cookie)
	if w.Code != http.StatusServiceUnavailable || len(w.Result().Cookies()) != 0 || len(s.sessions) != 1 {
		t.Fatal("failed durable login must not issue a cookie or replace the old session")
	}
	requireAuthenticated(t, mux, cookie, true)
	s.cfg.SessionFile = cfg.SessionFile
	w = verifyRemembered(mux, otherEmail, code, true, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("retry should retain the unused code: %d %s", w.Code, w.Body.String())
	}
	current := w.Result().Cookies()[0]
	requireAuthenticated(t, mux, cookie, false)
	requireAuthenticated(t, mux, current, true)
	w = request(mux, http.MethodPost, "/api/logout", "{}", current)
	if w.Code != http.StatusOK {
		t.Fatalf("logout retry: %s", w.Body.String())
	}
	_, restarted := rememberedService(t, cfg)
	requireAuthenticated(t, restarted, cookie, false)
	requireAuthenticated(t, restarted, current, false)
}
