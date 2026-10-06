package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testService(t *testing.T) (*Service, *http.ServeMux, *time.Time) {
	t.Helper()
	s, err := New(Config{DevMode: true, SessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	mux := http.NewServeMux()
	s.Register(mux)
	return s, mux, &now
}

func request(mux *http.ServeMux, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://localhost:8080"+path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func loginCode(t *testing.T, mux *http.ServeMux, email string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email})
	w := request(mux, http.MethodPost, "/api/login", string(body), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("login returned %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Code string `json:"devCode"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Code) != 6 {
		t.Fatalf("expected six-digit development code, got %q", response.Code)
	}
	return response.Code
}

func verifyCode(mux *http.ServeMux, email, code string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"email": email, "code": code})
	return request(mux, http.MethodPost, "/api/verify", string(body), nil)
}

func TestLoginSessionReplayAndLogout(t *testing.T) {
	s, mux, _ := testService(t)
	code := loginCode(t, mux, "  Cookie.Baker+one@Example.com ")
	w := verifyCode(mux, "cookie.baker+one@example.com", code)
	if w.Code != http.StatusOK {
		t.Fatalf("verify returned %d: %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/" || cookies[0].MaxAge != 3600 {
		t.Fatalf("incorrect session cookie: %+v", cookies)
	}
	cookie := cookies[0]
	if len(cookie.Value) != 43 {
		t.Fatalf("opaque session token has unexpected length %d", len(cookie.Value))
	}
	if w := verifyCode(mux, "cookie.baker+one@example.com", code); w.Code != http.StatusUnauthorized {
		t.Fatalf("replayed code returned %d", w.Code)
	}
	w = request(mux, http.MethodGet, "/api/session", "", cookie)
	var sessionResponse struct {
		Authenticated bool   `json:"authenticated"`
		Player        Player `json:"player"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &sessionResponse); err != nil {
		t.Fatal(err)
	}
	if !sessionResponse.Authenticated || sessionResponse.Player.ID != playerForEmail("cookie.baker+one@example.com").ID || sessionResponse.Player.Name != "cookiebakerone" {
		t.Fatalf("incorrect session response: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "@") {
		t.Fatal("session exposed the email address")
	}
	w = request(mux, http.MethodPost, "/api/logout", "{}", cookie)
	if w.Code != http.StatusOK || len(s.sessions) != 0 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not revoke session and clear cookie")
	}
	w = request(mux, http.MethodGet, "/api/session", "", cookie)
	if !strings.Contains(w.Body.String(), `"authenticated":false`) {
		t.Fatal("logged-out session still authenticated")
	}
}

func TestCodeAndSessionExpire(t *testing.T) {
	_, mux, now := testService(t)
	code := loginCode(t, mux, "baker@example.com")
	*now = now.Add(challengeTTL)
	if w := verifyCode(mux, "baker@example.com", code); w.Code != http.StatusUnauthorized {
		t.Fatalf("expired code returned %d", w.Code)
	}
	code = loginCode(t, mux, "baker@example.com")
	w := verifyCode(mux, "baker@example.com", code)
	if w.Code != http.StatusOK {
		t.Fatalf("verify failed: %s", w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	*now = now.Add(time.Hour)
	w = request(mux, http.MethodGet, "/api/session", "", cookie)
	if !strings.Contains(w.Body.String(), `"authenticated":false`) {
		t.Fatal("expired session still authenticated")
	}
}

func TestFiveWrongAttemptsInvalidateCode(t *testing.T) {
	s, mux, _ := testService(t)
	code := loginCode(t, mux, "baker@example.com")
	for i := 0; i < maxCodeAttempts; i++ {
		if w := verifyCode(mux, "baker@example.com", "invalid"); w.Code != http.StatusUnauthorized {
			t.Fatalf("wrong code returned %d", w.Code)
		}
	}
	if _, exists := s.challenges["baker@example.com"]; exists {
		t.Fatal("challenge remained after five failed attempts")
	}
	if w := verifyCode(mux, "baker@example.com", code); w.Code != http.StatusUnauthorized {
		t.Fatalf("locked code returned %d", w.Code)
	}
}

func TestLoginThrottlesEmailAndSocketIP(t *testing.T) {
	_, mux, now := testService(t)
	loginCode(t, mux, "baker@example.com")
	w := request(mux, http.MethodPost, "/api/login", `{"email":"BAKER@example.com"}`, nil)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("repeated email returned %d", w.Code)
	}
	*now = now.Add(time.Minute)
	loginCode(t, mux, "baker@example.com")
	for i := 0; i < 7; i++ {
		loginCode(t, mux, fmt.Sprintf("baker%d@example.com", i))
	}
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/login", strings.NewReader(`{"email":"last@example.com"}`))
	r.RemoteAddr = "127.0.0.1:3333"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Forwarded-For", "192.0.2.99")
	r.Header.Set("X-Real-IP", "192.0.2.100")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("forwarding headers bypassed IP throttle: %d", w.Code)
	}
}

func TestVerifyThrottleIncludesMissingChallenges(t *testing.T) {
	_, mux, _ := testService(t)
	for i := 0; i < 40; i++ {
		w := verifyCode(mux, fmt.Sprintf("missing%d@example.com", i), "123456")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d returned %d", i, w.Code)
		}
	}
	if w := verifyCode(mux, "another@example.com", "123456"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("IP verify throttle returned %d", w.Code)
	}
}

func TestOriginContentTypeAndBodyGuards(t *testing.T) {
	tests := []struct {
		name, origin, fetchSite, contentType, body string
		status                                     int
	}{
		{"foreign origin", "https://attacker.example", "", "application/json", `{}`, http.StatusForbidden},
		{"sibling origin", "http://sibling.localhost:8080", "", "application/json", `{}`, http.StatusForbidden},
		{"null origin", "null", "", "application/json", `{}`, http.StatusForbidden},
		{"wrong scheme", "https://localhost:8080", "", "application/json", `{}`, http.StatusForbidden},
		{"cross-site fetch", "", "cross-site", "application/json", `{}`, http.StatusForbidden},
		{"same-site fetch", "", "same-site", "application/json", `{}`, http.StatusForbidden},
		{"form submission", "", "", "application/x-www-form-urlencoded", `{}`, http.StatusUnsupportedMediaType},
		{"malformed JSON", "", "", "application/json", `{`, http.StatusBadRequest},
		{"trailing JSON", "", "", "application/json", `{"email":"a@example.com"}{}`, http.StatusBadRequest},
		{"unexpected key", "", "", "application/json", `{"email":"a@example.com","admin":true}`, http.StatusBadRequest},
		{"large body", "", "", "application/json", `{"email":"` + strings.Repeat("a", 2200) + `@example.com"}`, http.StatusBadRequest},
		{"same origin", "http://localhost:8080", "same-origin", "application/json; charset=utf-8", `{"email":"a@example.com"}`, http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, mux, _ := testService(t)
			r := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/login", strings.NewReader(test.body))
			r.RemoteAddr = "127.0.0.1:1234"
			r.Header.Set("Origin", test.origin)
			r.Header.Set("Sec-Fetch-Site", test.fetchSite)
			r.Header.Set("Content-Type", test.contentType)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != test.status {
				t.Fatalf("got %d, want %d: %s", w.Code, test.status, w.Body.String())
			}
		})
	}
}

func TestDevelopmentCodesRequireLoopbackSocket(t *testing.T) {
	_, mux, _ := testService(t)
	for _, path := range []string{"/api/login", "/api/verify"} {
		r := httptest.NewRequest(http.MethodPost, "http://localhost:8080"+path, strings.NewReader(`{"email":"baker@example.com"}`))
		r.RemoteAddr = "192.0.2.1:1234"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Forwarded-For", "127.0.0.1")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "devCode") {
			t.Fatalf("nonlocal %s returned %d: %s", path, w.Code, w.Body.String())
		}
	}
}

func TestProductionDeliveryAndSecureCookie(t *testing.T) {
	s, err := New(Config{SecureCookies: true, SMTPHost: "smtp.example.com", SMTPFrom: "CookieKill <login@example.com>"})
	if err != nil {
		t.Fatal(err)
	}
	var sentCode string
	s.send = func(email, code string) error {
		if email != "baker@example.com" {
			t.Fatalf("unexpected recipient %q", email)
		}
		sentCode = code
		return nil
	}
	mux := http.NewServeMux()
	s.Register(mux)
	w := request(mux, http.MethodPost, "/api/login", `{"email":"baker@example.com"}`, nil)
	if w.Code != http.StatusOK || sentCode == "" || strings.Contains(w.Body.String(), "devCode") || strings.Contains(w.Body.String(), sentCode) {
		t.Fatalf("unsafe production response: %d %s", w.Code, w.Body.String())
	}
	w = verifyCode(mux, "baker@example.com", sentCode)
	if w.Code != http.StatusOK {
		t.Fatalf("verify returned %d: %s", w.Code, w.Body.String())
	}
	if cookie := w.Result().Cookies()[0]; !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("unsafe production cookie: %+v", cookie)
	}
}

func TestFailedDeliveryRevokesChallengeWithoutLeakingSMTPError(t *testing.T) {
	s, err := New(Config{SecureCookies: true, SMTPHost: "smtp.example.com", SMTPFrom: "login@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	s.send = func(email, code string) error { return errors.New("secret SMTP detail") }
	mux := http.NewServeMux()
	s.Register(mux)
	w := request(mux, http.MethodPost, "/api/login", `{"email":"baker@example.com"}`, nil)
	if w.Code != http.StatusServiceUnavailable || strings.Contains(w.Body.String(), "secret") || len(s.challenges) != 0 {
		t.Fatalf("unsafe delivery failure: %d %s", w.Code, w.Body.String())
	}
}

func TestProductionConfigurationRequired(t *testing.T) {
	for _, cfg := range []Config{
		{},
		{SecureCookies: true},
		{SecureCookies: true, SMTPHost: "smtp.example.com", SMTPFrom: "bad"},
		{SecureCookies: true, SMTPHost: "smtp.example.com", SMTPFrom: "login@example.com", SMTPUser: "user"},
		{SecureCookies: true, SMTPHost: "smtp.example.com", SMTPFrom: "login@example.com", SMTPPort: 70000},
		{DevMode: true, SessionTTL: time.Second},
	} {
		if _, err := New(cfg); err == nil {
			t.Fatalf("unsafe configuration accepted: %+v", cfg)
		}
	}
}

func TestProductionSMTPAuthenticationIsOptional(t *testing.T) {
	for _, test := range []struct {
		name, user, password string
		wantError            bool
	}{
		{name: "relay without login"},
		{name: "authenticated relay", user: "user", password: "password"},
		{name: "username without password", user: "user", wantError: true},
		{name: "password without username", password: "password", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(Config{
				SecureCookies: true,
				SMTPHost:      "smtp.example.com",
				SMTPFrom:      "login@example.com",
				SMTPUser:      test.user,
				SMTPPass:      test.password,
			})
			if (err != nil) != test.wantError {
				t.Fatalf("New() error = %v, want error = %v", err, test.wantError)
			}
		})
	}
}

func TestEmailNormalizationRejectsHeaderInjection(t *testing.T) {
	for _, input := range []string{"", "not-an-email", "Baker <baker@example.com>", "a@example.com\r\nBcc: b@example.com", "a@example.com\n", strings.Repeat("a", 65) + "@example.com"} {
		if _, ok := normalizeEmail(input); ok {
			t.Fatalf("accepted invalid address %q", input)
		}
	}
	if normalized, ok := normalizeEmail(" Baker@EXAMPLE.COM "); !ok || normalized != "baker@example.com" {
		t.Fatal("email normalization failed")
	}
}
