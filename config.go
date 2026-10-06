package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const configFilename = "config.json"

type appConfig struct {
	Dev     bool       `json:"dev"`
	Addr    string     `json:"addr"`
	Domain  string     `json:"domain"`
	Email   string     `json:"email"`
	DataDir string     `json:"dataDir"`
	LogFile string     `json:"logFile"`
	SMTP    smtpConfig `json:"smtp"`
}

type smtpConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	From     string `json:"from"`
}

// loadConfig always finds service configuration beside the executable, rather
// than in the service manager's working directory. Environment variables never
// override configuration. The only file-free mode is explicit console -dev.
func loadConfig(executablePath string, args []string, service bool) (appConfig, error) {
	cfg := appConfig{
		Addr:    "127.0.0.1:8080",
		DataDir: "data",
		LogFile: filepath.Join("logs", "cookiekill.log"),
		SMTP:    smtpConfig{Port: 587},
	}
	if service && len(args) != 0 {
		return cfg, errors.New("Windows service does not accept arguments; configure config.json beside the executable")
	}
	executablePath, err := filepath.Abs(executablePath)
	if err != nil {
		return cfg, fmt.Errorf("resolve executable path: %w", err)
	}
	baseDir := filepath.Dir(executablePath)
	configPath := filepath.Join(baseDir, configFilename)
	file, err := os.Open(configPath)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return cfg, fmt.Errorf("open %s: %w", configPath, err)
	}
	if !missing {
		defer file.Close()
		reader := bufio.NewReader(file)
		// Windows PowerShell 5.1 and some editors write a UTF-8 BOM. It is
		// an encoding marker, not part of the JSON document.
		if prefix, _ := reader.Peek(3); bytes.Equal(prefix, []byte{0xef, 0xbb, 0xbf}) {
			_, _ = reader.Discard(3)
		}
		decoder := json.NewDecoder(reader)
		decoder.DisallowUnknownFields()
		value := &cfg
		if err := decoder.Decode(&value); err != nil {
			return cfg, fmt.Errorf("decode %s: %w", configPath, err)
		}
		if value == nil {
			return cfg, fmt.Errorf("%s must contain a JSON object", configPath)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return cfg, fmt.Errorf("%s must contain exactly one JSON object", configPath)
		}
	}
	if !service {
		flags := flag.NewFlagSet("CookieKill", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		flags.BoolVar(&cfg.Dev, "dev", cfg.Dev, "enable local-only development email codes")
		flags.StringVar(&cfg.Addr, "addr", cfg.Addr, "development listen address (loopback only)")
		flags.StringVar(&cfg.DataDir, "data", cfg.DataDir, "persistent progress and certificate directory")
		if err := flags.Parse(args); err != nil {
			return cfg, err
		}
		if flags.NArg() != 0 {
			return cfg, errors.New("unexpected arguments; configure config.json beside the executable")
		}
	}
	if missing {
		if service || !cfg.Dev {
			return cfg, fmt.Errorf("required configuration %s is missing; create it beside the executable (console development can use -dev)", configPath)
		}
		// go run builds into a temporary directory. Keep its development data in
		// the caller's directory when no sibling configuration exists.
		baseDir, err = os.Getwd()
		if err != nil {
			return cfg, fmt.Errorf("resolve development working directory: %w", err)
		}
	}
	if err := validateMode(cfg.Dev, cfg.Addr, cfg.Domain, cfg.Email); err != nil {
		return cfg, err
	}
	if cfg.SMTP.Port < 1 || cfg.SMTP.Port > 65535 {
		return cfg, errors.New("config.json smtp.port must be between 1 and 65535")
	}
	if strings.TrimSpace(cfg.DataDir) == "" || strings.TrimSpace(cfg.LogFile) == "" {
		return cfg, errors.New("config.json dataDir and logFile must not be empty")
	}
	if !filepath.IsAbs(cfg.DataDir) {
		cfg.DataDir = filepath.Join(baseDir, cfg.DataDir)
	}
	if !filepath.IsAbs(cfg.LogFile) {
		cfg.LogFile = filepath.Join(baseDir, cfg.LogFile)
	}
	return cfg, nil
}
