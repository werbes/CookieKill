package main

import "testing"

func TestDevelopmentMustStayOnLoopback(t *testing.T) {
	for _, addr := range []string{":8080", "0.0.0.0:8080", "192.168.1.4:8080", "localhost:8080"} {
		if validateMode(true, addr, "", "") == nil {
			t.Errorf("accepted unsafe development address %q", addr)
		}
	}
	for _, addr := range []string{"127.0.0.1:8080", "[::1]:8080"} {
		if err := validateMode(true, addr, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if validateMode(true, "127.0.0.1:8080", "play.example.com", "") == nil {
		t.Fatal("development enabled with public domain")
	}
}

func TestProductionConfiguration(t *testing.T) {
	for _, domain := range []string{"", "127.0.0.1", "https://example.com", "example.com:443", "localhost", "bad host.com"} {
		if validateMode(false, "", domain, "admin@example.com") == nil {
			t.Errorf("accepted invalid domain %q", domain)
		}
	}
	if err := validateMode(false, "", "play.example.com", ""); err != nil {
		t.Fatalf("optional ACME contact email was required: %v", err)
	}
	if validateMode(false, "", "play.example.com", "not-an-email") == nil {
		t.Fatal("accepted invalid optional ACME email")
	}
	if err := validateMode(false, "", "play.example.com", "admin@example.com"); err != nil {
		t.Fatal(err)
	}
}
