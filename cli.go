package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
)

const backgroundLogPath = "runtime/arena.log"

type runtimeConfig struct {
	ListenAddr   string
	SnapshotPath string
	Background   bool
}

func parseRuntimeConfig(args []string, getenv func(string) string) (runtimeConfig, bool, error) {
	var cfg runtimeConfig

	fs := flag.NewFlagSet("pico-xiangqi-arena", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	defaultSnapshotPath := strings.TrimSpace(getenv("SNAPSHOT_PATH"))
	if defaultSnapshotPath == "" {
		defaultSnapshotPath = filepath.Join("runtime", "arena-snapshot.json")
	}

	var (
		host       string
		port       string
		listen     string
		snapshot   string
		background bool
		daemon     bool
		help       bool
		shortHelp  bool
	)

	fs.StringVar(&host, "host", strings.TrimSpace(getenv("HOST")), "IP or hostname to bind")
	fs.StringVar(&port, "port", strings.TrimSpace(getenv("PORT")), "Port to listen on")
	fs.StringVar(&listen, "listen", strings.TrimSpace(getenv("LISTEN")), "Full listen address, overrides --host/--port")
	fs.StringVar(&snapshot, "snapshot", defaultSnapshotPath, "Snapshot file path")
	fs.BoolVar(&background, "background", false, "Start in the background")
	fs.BoolVar(&daemon, "daemon", false, "Alias of --background")
	fs.BoolVar(&help, "help", false, "Show help and exit")
	fs.BoolVar(&shortHelp, "h", false, "Show help and exit")

	if err := fs.Parse(args); err != nil {
		return cfg, false, err
	}

	if help || shortHelp {
		return runtimeConfig{}, true, nil
	}

	cfg.ListenAddr = normalizeListenAddr(host, port, listen)
	cfg.SnapshotPath = strings.TrimSpace(snapshot)
	cfg.Background = background || daemon
	return cfg, false, nil
}

func normalizeListenAddr(host string, port string, listen string) string {
	listen = strings.TrimSpace(listen)
	if listen != "" {
		return listen
	}

	host = normalizeListenHost(host)
	rawPort := strings.TrimSpace(port)
	if rawPort == "" {
		rawPort = "8080"
	}

	if host == "" {
		if strings.Contains(rawPort, ":") {
			return rawPort
		}
		if strings.HasPrefix(rawPort, ":") {
			return rawPort
		}
		return ":" + rawPort
	}

	rawPort = strings.TrimPrefix(rawPort, ":")
	return net.JoinHostPort(host, rawPort)
}

func normalizeListenHost(host string) string {
	host = strings.TrimSpace(host)
	if len(host) >= 2 && strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return host[1 : len(host)-1]
	}
	return host
}

func runtimeConfigUsage(appName string) string {
	return fmt.Sprintf(`Usage:
  %s [options]

Options:
  --host <ip>         Bind to a specific IP or hostname
  --port <port>       Listen port, default 8080
  --listen <addr>     Full listen address, overrides --host/--port
  --snapshot <path>   Snapshot file path, default runtime/arena-snapshot.json
  --background        Start in background mode and write logs to %s
  --daemon            Alias of --background
  --help, -h          Show this help text

Examples:
  %s
  %s --port 9090
  %s --host 0.0.0.0 --port 8080
  %s --host :: --port 8080
  %s --listen [::]:8080 --background

Environment:
  PORT            Fallback listen port
  HOST            Fallback bind host
  LISTEN          Fallback full listen address
  SNAPSHOT_PATH   Fallback snapshot file path
`, appName, backgroundLogPath, appName, appName, appName, appName, appName)
}

func stripBackgroundArgs(args []string) []string {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--background", "-background", "--daemon", "-daemon":
			continue
		default:
			filtered = append(filtered, arg)
		}
	}
	return filtered
}

func displayListenAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, ":") {
		return "0.0.0.0" + addr
	}
	return addr
}
