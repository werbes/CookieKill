package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func receiveWithin[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for lifecycle event")
		var zero T
		return zero
	}
}

// Capturing the structured readiness URL lets the test use an OS-assigned port.
type readinessLogHandler struct {
	slog.Handler
	urls chan string
}

func (h readinessLogHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Message == "CookieKill ready" {
		record.Attrs(func(attr slog.Attr) bool {
			if attr.Key == "url" {
				h.urls <- attr.Value.String()
			}
			return true
		})
	}
	return h.Handler.Handle(ctx, record)
}

func TestRunServerCancellationSavesAndReleasesListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dataDir := t.TempDir()
	cfg := appConfig{Dev: true, Addr: "127.0.0.1:0", DataDir: dataDir}
	ready := make(chan struct{}, 1)
	done := make(chan error, 1)
	urls := make(chan string, 1)
	logger := slog.New(readinessLogHandler{Handler: slog.NewTextHandler(io.Discard, nil), urls: urls})
	go func() { done <- runServer(ctx, cfg, func() { ready <- struct{}{} }, logger) }()
	receiveWithin(t, ready)
	url := receiveWithin(t, urls)
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(url + "/")
	if err != nil {
		t.Fatalf("ready server is not serving HTTP: %v", err)
	}
	response.Body.Close()
	client.CloseIdleConnections()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("ready server returned HTTP %d", response.StatusCode)
	}
	cancel()
	if err := receiveWithin(t, done); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(dataDir, "players.json"))
	if err != nil {
		t.Fatalf("shutdown did not persist players: %v", err)
	}
	var save struct {
		Version  int                        `json:"version"`
		Profiles map[string]json.RawMessage `json:"profiles"`
	}
	if err := json.Unmarshal(contents, &save); err != nil || save.Version != 2 || save.Profiles == nil {
		t.Fatalf("invalid shutdown save: %s (decode error: %v)", contents, err)
	}
	listener, err := net.Listen("tcp", strings.TrimPrefix(url, "http://"))
	if err != nil {
		t.Fatalf("shutdown did not release its listener: %v", err)
	}
	listener.Close()
}

func TestRunServerOccupiedAddressNeverReportsReady(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cfg := appConfig{Dev: true, Addr: listener.Addr().String(), DataDir: t.TempDir()}
	ready := make(chan struct{}, 1)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		done <- runServer(ctx, cfg, func() { ready <- struct{}{} }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	if err := receiveWithin(t, done); err == nil || !strings.Contains(err.Error(), "listen on ") {
		t.Fatalf("expected listener startup error, got %v", err)
	}
	select {
	case <-ready:
		t.Fatal("server reported ready despite failing to bind its listener")
	default:
	}
}

func TestRunServerPreservesMalformedSave(t *testing.T) {
	dataDir := t.TempDir()
	savePath := filepath.Join(dataDir, "players.json")
	contents := []byte(`{"version":2,"profiles":`)
	if err := os.WriteFile(savePath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{}, 1)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		done <- runServer(ctx, appConfig{Dev: true, Addr: "127.0.0.1:0", DataDir: dataDir}, func() { ready <- struct{}{} }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	if err := receiveWithin(t, done); err == nil || !strings.Contains(err.Error(), "invalid save file") {
		t.Fatalf("expected malformed-save startup error, got %v", err)
	}
	select {
	case <-ready:
		t.Fatal("server reported ready despite a malformed save")
	default:
	}
	after, err := os.ReadFile(savePath)
	if err != nil || string(after) != string(contents) {
		t.Fatalf("malformed save was not preserved: %q (read error: %v)", after, err)
	}
}
