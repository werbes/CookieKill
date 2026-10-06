package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, configFilename), []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "CookieKill.exe")
}

func TestServiceConfigurationIsRelativeToExecutable(t *testing.T) {
	exe := writeTestConfig(t, `{"dev":true}`)
	t.Chdir(t.TempDir())
	// Legacy variables must not make service startup depend on its account's
	// environment or silently override the JSON file.
	for _, key := range []string{"CK_DOMAIN", "CK_ACME_EMAIL", "CK_DATA_DIR", "CK_SMTP_HOST", "CK_SMTP_PORT", "CK_SMTP_USER", "CK_SMTP_PASS", "CK_SMTP_FROM"} {
		t.Setenv(key, "ignored")
	}
	cfg, err := loadConfig(exe, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:8080" || cfg.SMTP.Port != 587 || cfg.Domain != "" || cfg.SMTP.Host != "" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.DataDir != filepath.Join(filepath.Dir(exe), "data") {
		t.Fatalf("dataDir = %q", cfg.DataDir)
	}
	if cfg.LogFile != filepath.Join(filepath.Dir(exe), "logs", "cookiekill.log") {
		t.Fatalf("logFile = %q", cfg.LogFile)
	}
}

func TestProductionJSONConfiguration(t *testing.T) {
	exe := writeTestConfig(t, `{
		"dev":false, "addr":"127.0.0.1:8090", "domain":"play.example.com", "email":"admin@example.com",
		"dataDir":"state", "logFile":"output/service.log",
		"smtp":{"host":"smtp.example.com", "port":465, "user":"mailer", "password":"secret", "from":"login@example.com"}
	}`)
	cfg, err := loadConfig(exe, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dev || cfg.Addr != "127.0.0.1:8090" || cfg.Domain != "play.example.com" || cfg.Email != "admin@example.com" {
		t.Fatal("production settings were not loaded")
	}
	if cfg.SMTP != (smtpConfig{Host: "smtp.example.com", Port: 465, User: "mailer", Password: "secret", From: "login@example.com"}) {
		t.Fatal("SMTP settings were not loaded")
	}
	if cfg.DataDir != filepath.Join(filepath.Dir(exe), "state") || cfg.LogFile != filepath.Join(filepath.Dir(exe), "output", "service.log") {
		t.Fatal("relative paths were not resolved beside the executable")
	}
}

func TestConfigAcceptsUTF8BOM(t *testing.T) {
	exe := writeTestConfig(t, "\xef\xbb\xbf"+`{"dev":true,"smtp":{"password":"café"}}`)
	cfg, err := loadConfig(exe, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Dev || cfg.SMTP.Password != "café" {
		t.Fatal("UTF-8 configuration was not decoded correctly")
	}
}

func TestConfigPreservesAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "state")
	log := filepath.Join(dir, "service.log")
	contents, err := json.Marshal(map[string]any{"dev": true, "dataDir": data, "logFile": log})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(writeTestConfig(t, string(contents)), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != data || cfg.LogFile != log {
		t.Fatal("absolute paths were changed")
	}
}

func TestProductionConfigWithoutContactOrSMTPLogin(t *testing.T) {
	for _, contact := range []string{"", `"email":"",`} {
		exe := writeTestConfig(t, `{"domain":"play.example.com",`+contact+`"smtp":{"host":"smtp.example.com","from":"game@example.com","user":"","password":""}}`)
		cfg, err := loadConfig(exe, nil, true)
		if err != nil {
			t.Fatalf("production configuration without contact/login was rejected: %v", err)
		}
		if cfg.Email != "" || cfg.SMTP.User != "" || cfg.SMTP.Password != "" {
			t.Fatal("unexpected contact email or SMTP credentials")
		}
	}
}

func TestConfigRejectsInvalidJSONAndValues(t *testing.T) {
	for name, contents := range map[string]string{
		"empty":              ``,
		"malformed":          `{"dev":true`,
		"null":               `null`,
		"array":              `[]`,
		"unknown field":      `{"dev":true,"data":"state"}`,
		"unknown SMTP field": `{"dev":true,"smtp":{"pass":"secret"}}`,
		"trailing object":    `{"dev":true} {}`,
		"trailing garbage":   `{"dev":true} rubbish`,
		"wrong value type":   `{"dev":"true"}`,
		"zero port":          `{"dev":true,"smtp":{"port":0}}`,
		"negative port":      `{"dev":true,"smtp":{"port":-1}}`,
		"large port":         `{"dev":true,"smtp":{"port":65536}}`,
		"empty data path":    `{"dev":true,"dataDir":""}`,
		"empty log path":     `{"dev":true,"logFile":" "}`,
		"unsafe dev address": `{"dev":true,"addr":"0.0.0.0:8080"}`,
		"missing domain":     `{"dev":false}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadConfig(writeTestConfig(t, contents), nil, true); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}

func TestServiceRequiresJSONAndRejectsArguments(t *testing.T) {
	missingExe := filepath.Join(t.TempDir(), "CookieKill.exe")
	if _, err := loadConfig(missingExe, nil, true); err == nil || !strings.Contains(err.Error(), "config.json") {
		t.Fatal("service accepted missing configuration")
	}
	exe := writeTestConfig(t, `{"dev":true}`)
	if _, err := loadConfig(exe, []string{"-dev"}, true); err == nil {
		t.Fatal("service accepted command-line overrides")
	}
}

func TestConsoleDevelopmentWithoutConfig(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	exe := filepath.Join(t.TempDir(), "CookieKill.exe")
	for _, args := range [][]string{nil, {"-addr", "127.0.0.1:8090"}, {"-dev=false"}} {
		if _, err := loadConfig(exe, args, false); err == nil {
			t.Fatalf("missing configuration accepted without explicit development: %v", args)
		}
	}
	cfg, err := loadConfig(exe, []string{"-dev", "-addr", "127.0.0.1:8090", "-data", "local-state"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Dev || cfg.Addr != "127.0.0.1:8090" || cfg.DataDir != filepath.Join(cwd, "local-state") || cfg.LogFile != filepath.Join(cwd, "logs", "cookiekill.log") {
		t.Fatalf("unexpected console development configuration: %+v", cfg)
	}
}

func TestConsoleUsesSiblingJSON(t *testing.T) {
	exe := writeTestConfig(t, `{"dev":true,"dataDir":"configured-state"}`)
	t.Chdir(t.TempDir())
	cfg, err := loadConfig(exe, []string{"-addr", "127.0.0.1:8091"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:8091" || cfg.DataDir != filepath.Join(filepath.Dir(exe), "configured-state") {
		t.Fatal("console did not use sibling configuration with overrides")
	}
	for _, args := range [][]string{{"-domain", "play.example.com"}, {"extra"}} {
		if _, err := loadConfig(exe, args, false); err == nil {
			t.Fatalf("unsupported arguments accepted: %v", args)
		}
	}
}
