package main

import (
	"context"
	"crypto/tls"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"cookiekill/internal/auth"
	"cookiekill/internal/server"
	"cookiekill/internal/store"
	"golang.org/x/crypto/acme/autocert"
)

//go:embed web
var embedded embed.FS

func main() {
	if err := run(); err != nil {
		slog.Error("CookieKill stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	return runPlatform()
}

func runConsole() error {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "-h", "--h", "-help", "--help":
			printUsage()
			return nil
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runApplication(ctx, false, func() {})
}

func runApplication(ctx context.Context, service bool, ready func()) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cfg, err := loadConfig(executable, os.Args[1:], service)
	if err != nil {
		if !service && errors.Is(err, flag.ErrHelp) {
			printUsage()
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.LogFile), 0700); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	logFile, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logFile.Close()
	var output io.Writer = logFile
	if !service {
		output = io.MultiWriter(os.Stderr, logFile)
	}
	logger := slog.New(slog.NewTextHandler(output, nil))
	logger.Info("CookieKill starting", "service", service, "log", cfg.LogFile)
	err = runServer(ctx, cfg, ready, logger)
	if err != nil {
		logger.Error("CookieKill stopped", "error", err)
	} else {
		logger.Info("CookieKill stopped")
	}
	return err
}

func printUsage() {
	fmt.Fprintln(os.Stdout, `CookieKill reads config.json beside its executable.
Windows services start without arguments; edit config.json and restart the service.

Console development options:
  -dev          Enable local-only HTTP and development login codes.
  -addr address Development listen address (default 127.0.0.1:8080).
  -data path    Override the progress directory.

Without config.json, explicit -dev uses data and logs in the working directory.`)
}

// runServer reports readiness only after configuration, saved profiles and all
// listeners have initialized. The same cancellation path serves Ctrl+C and SCM.
func runServer(ctx context.Context, cfg appConfig, ready func(), logger *slog.Logger) error {
	a, err := auth.New(auth.Config{
		DevMode: cfg.Dev, SecureCookies: !cfg.Dev, SMTPHost: cfg.SMTP.Host, SMTPPort: cfg.SMTP.Port,
		SMTPUser: cfg.SMTP.User, SMTPPass: cfg.SMTP.Password, SMTPFrom: cfg.SMTP.From,
		SessionTTL: 7 * 24 * time.Hour,
	})
	if err != nil {
		return err
	}
	db, err := store.New(cfg.DataDir)
	if err != nil {
		return err
	}
	profiles, err := db.Load()
	if err != nil {
		return err
	}
	assets, err := fs.Sub(embedded, "web")
	if err != nil {
		return err
	}
	hub := server.New(a, db, profiles, logger)
	app := &http.Server{Addr: cfg.Addr, Handler: hub.Handler(assets, !cfg.Dev), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 40 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 16 << 10, ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError)}
	var challenge *http.Server
	if !cfg.Dev {
		certDir := filepath.Join(cfg.DataDir, "certificates")
		if err := os.MkdirAll(certDir, 0700); err != nil {
			return err
		}
		manager := &autocert.Manager{Prompt: autocert.AcceptTOS, Email: cfg.Email, HostPolicy: autocert.HostWhitelist(cfg.Domain), Cache: autocert.DirCache(certDir)}
		app.Addr = ":443"
		app.TLSConfig = manager.TLSConfig()
		app.TLSConfig.MinVersion = tls.VersionTLS12
		redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://"+cfg.Domain+r.URL.RequestURI(), http.StatusPermanentRedirect)
		})
		challenge = &http.Server{Addr: ":80", Handler: manager.HTTPHandler(redirect), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10, ErrorLog: app.ErrorLog}
	}
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", app.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", app.Addr, err)
	}
	defer listener.Close()
	var challengeListener net.Listener
	if challenge != nil {
		challengeListener, err = listenConfig.Listen(ctx, "tcp", challenge.Addr)
		if err != nil {
			return fmt.Errorf("listen on %s: %w", challenge.Addr, err)
		}
		defer challengeListener.Close()
	}
	if ctx.Err() != nil {
		return nil
	}
	loopCtx, cancelLoop := context.WithCancel(ctx)
	defer cancelLoop()
	loopDone := make(chan struct{})
	go func() { defer close(loopDone); hub.Run(loopCtx) }()
	errCh := make(chan error, 2)
	if cfg.Dev {
		go func() { errCh <- app.Serve(listener) }()
		logger.Info("CookieKill ready", "url", "http://"+listener.Addr().String(), "mode", "local development", "save", cfg.DataDir)
	} else {
		go func() { errCh <- challenge.Serve(challengeListener) }()
		go func() { errCh <- app.ServeTLS(listener, "", "") }()
		logger.Info("CookieKill ready", "url", "https://"+cfg.Domain, "mode", "production", "save", cfg.DataDir)
	}
	ready()
	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-errCh:
	}
	cancelLoop()
	<-loopDone
	closeErr := hub.Close()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	shutdownErr := app.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		app.Close()
	}
	if challenge != nil {
		if err := challenge.Shutdown(shutdownCtx); err != nil {
			challenge.Close()
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(serveErr, closeErr, shutdownErr)
}

func validateMode(dev bool, addr, domain, email string) error {
	if dev {
		if domain != "" {
			return errors.New("dev cannot be combined with a public domain")
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("development address: %w", err)
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return errors.New("dev requires a literal loopback address, such as 127.0.0.1:8080")
		}
		return nil
	}
	if domain == "" {
		return errors.New("use -dev for local play, or configure domain and smtp in config.json for production")
	}
	if net.ParseIP(domain) != nil || !strings.Contains(domain, ".") || strings.ContainsAny(domain, " /:@\\") || domain != strings.ToLower(domain) {
		return errors.New("domain must be a lowercase public hostname, for example play.example.com")
	}
	if email != "" && !strings.Contains(email, "@") {
		return errors.New("email must be a Let's Encrypt contact address or empty; SMTP login uses smtp.user and smtp.password")
	}
	return nil
}
