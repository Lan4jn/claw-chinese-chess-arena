package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRuntimeConfigDefaults(t *testing.T) {
	cfg, showHelp, err := parseRuntimeConfig(nil, func(string) string { return "" })
	if err != nil {
		t.Fatalf("parseRuntimeConfig() error = %v", err)
	}
	if showHelp {
		t.Fatalf("expected showHelp=false")
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("expected default listen addr :8080, got %q", cfg.ListenAddr)
	}
	if cfg.SnapshotPath != filepath.Join("runtime", "arena-snapshot.json") {
		t.Fatalf("expected default snapshot path, got %q", cfg.SnapshotPath)
	}
	if cfg.Background {
		t.Fatalf("expected default background=false")
	}
}

func TestParseRuntimeConfigFromFlags(t *testing.T) {
	cfg, showHelp, err := parseRuntimeConfig([]string{
		"--host", "0.0.0.0",
		"--port", "9090",
		"--snapshot", "data/custom.json",
		"--background",
	}, func(string) string { return "" })
	if err != nil {
		t.Fatalf("parseRuntimeConfig() error = %v", err)
	}
	if showHelp {
		t.Fatalf("expected showHelp=false")
	}
	if cfg.ListenAddr != "0.0.0.0:9090" {
		t.Fatalf("expected listen addr 0.0.0.0:9090, got %q", cfg.ListenAddr)
	}
	if cfg.SnapshotPath != "data/custom.json" {
		t.Fatalf("expected custom snapshot path, got %q", cfg.SnapshotPath)
	}
	if !cfg.Background {
		t.Fatalf("expected background=true")
	}
}

func TestParseRuntimeConfigSupportsIPv6Host(t *testing.T) {
	cfg, showHelp, err := parseRuntimeConfig([]string{
		"--host", "::",
		"--port", "9090",
	}, func(string) string { return "" })
	if err != nil {
		t.Fatalf("parseRuntimeConfig() error = %v", err)
	}
	if showHelp {
		t.Fatalf("expected showHelp=false")
	}
	if cfg.ListenAddr != "[::]:9090" {
		t.Fatalf("expected IPv6 listen addr [::]:9090, got %q", cfg.ListenAddr)
	}
}

func TestParseRuntimeConfigSupportsBracketedIPv6Host(t *testing.T) {
	cfg, showHelp, err := parseRuntimeConfig([]string{
		"--host", "[::1]",
		"--port", "7070",
	}, func(string) string { return "" })
	if err != nil {
		t.Fatalf("parseRuntimeConfig() error = %v", err)
	}
	if showHelp {
		t.Fatalf("expected showHelp=false")
	}
	if cfg.ListenAddr != "[::1]:7070" {
		t.Fatalf("expected bracketed IPv6 host to normalize to [::1]:7070, got %q", cfg.ListenAddr)
	}
}

func TestParseRuntimeConfigListenFlagTakesPriority(t *testing.T) {
	cfg, showHelp, err := parseRuntimeConfig([]string{
		"--listen", "192.168.1.50:7777",
		"--host", "0.0.0.0",
		"--port", "9090",
	}, func(string) string { return "" })
	if err != nil {
		t.Fatalf("parseRuntimeConfig() error = %v", err)
	}
	if showHelp {
		t.Fatalf("expected showHelp=false")
	}
	if cfg.ListenAddr != "192.168.1.50:7777" {
		t.Fatalf("expected explicit listen addr to win, got %q", cfg.ListenAddr)
	}
}

func TestParseRuntimeConfigUsesEnvironmentFallbacks(t *testing.T) {
	env := map[string]string{
		"HOST":          "0.0.0.0",
		"PORT":          "8181",
		"SNAPSHOT_PATH": "runtime/custom-snapshot.json",
	}
	cfg, showHelp, err := parseRuntimeConfig(nil, func(key string) string {
		return env[key]
	})
	if err != nil {
		t.Fatalf("parseRuntimeConfig() error = %v", err)
	}
	if showHelp {
		t.Fatalf("expected showHelp=false")
	}
	if cfg.ListenAddr != "0.0.0.0:8181" {
		t.Fatalf("expected listen addr from env, got %q", cfg.ListenAddr)
	}
	if cfg.SnapshotPath != "runtime/custom-snapshot.json" {
		t.Fatalf("expected snapshot path from env, got %q", cfg.SnapshotPath)
	}
}

func TestParseRuntimeConfigUsesIPv6EnvironmentHost(t *testing.T) {
	env := map[string]string{
		"HOST": "::",
		"PORT": "8181",
	}
	cfg, showHelp, err := parseRuntimeConfig(nil, func(key string) string {
		return env[key]
	})
	if err != nil {
		t.Fatalf("parseRuntimeConfig() error = %v", err)
	}
	if showHelp {
		t.Fatalf("expected showHelp=false")
	}
	if cfg.ListenAddr != "[::]:8181" {
		t.Fatalf("expected IPv6 listen addr from env, got %q", cfg.ListenAddr)
	}
}

func TestParseRuntimeConfigHelp(t *testing.T) {
	cfg, showHelp, err := parseRuntimeConfig([]string{"--help"}, func(string) string { return "" })
	if err != nil {
		t.Fatalf("parseRuntimeConfig() unexpected error = %v", err)
	}
	if !showHelp {
		t.Fatalf("expected showHelp=true")
	}
	if cfg.ListenAddr != "" {
		t.Fatalf("expected empty config when only showing help, got listen addr %q", cfg.ListenAddr)
	}

	usage := runtimeConfigUsage("pico-xiangqi-arena")
	for _, expected := range []string{"--host", "--port", "--listen", "--background", "--help", "[::]:8080"} {
		if !strings.Contains(usage, expected) {
			t.Fatalf("expected usage text to contain %q, got %q", expected, usage)
		}
	}
}

func TestStripBackgroundArgs(t *testing.T) {
	args := stripBackgroundArgs([]string{
		"--host", "0.0.0.0",
		"--background",
		"--port", "8080",
		"--daemon",
		"--snapshot", "runtime/x.json",
	})

	got := strings.Join(args, " ")
	if strings.Contains(got, "--background") {
		t.Fatalf("expected --background to be removed, got %q", got)
	}
	if strings.Contains(got, "--daemon") {
		t.Fatalf("expected --daemon to be removed, got %q", got)
	}
	if !strings.Contains(got, "--host 0.0.0.0") || !strings.Contains(got, "--port 8080") {
		t.Fatalf("expected non-background flags to be preserved, got %q", got)
	}
}
