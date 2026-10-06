package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestDevelopmentRejectsDNSRebindingHost(t *testing.T) {
	s, _ := testServer(t)
	handler := s.Handler(fstest.MapFS{"index.html": {Data: []byte("CookieKill")}}, false)
	r := httptest.NewRequest(http.MethodPost, "http://attacker.example/api/login", strings.NewReader(`{"email":"baker@example.com"}`))
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://attacker.example")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusMisdirectedRequest || strings.Contains(w.Body.String(), "devCode") {
		t.Fatalf("development code accessible under foreign hostname: %d %s", w.Code, w.Body.String())
	}
}

func TestLocalHostValidation(t *testing.T) {
	for _, host := range []string{"localhost", "localhost:8080", "LOCALHOST:8080", "127.0.0.1:8080", "[::1]:8080", "[::1]"} {
		if !localHost(host) {
			t.Errorf("local host %q rejected", host)
		}
	}
	for _, host := range []string{"attacker.example:8080", "localhost.attacker.example", "127.0.0.1.attacker.example", "0.0.0.0:8080", "192.168.1.5:8080"} {
		if localHost(host) {
			t.Errorf("foreign host %q accepted", host)
		}
	}
}
