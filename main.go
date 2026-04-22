package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	cfg, showHelp, err := parseRuntimeConfig(os.Args[1:], os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	if showHelp {
		fmt.Print(runtimeConfigUsage(filepath.Base(os.Args[0])))
		return
	}

	if cfg.Background {
		pid, err := startInBackground(os.Args[1:])
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Pico Xiangqi Arena started in background with PID %d", pid)
		log.Printf("Background logs: %s", backgroundLogPath)
		return
	}

	store := NewFileSnapshotStore(cfg.SnapshotPath)
	app := NewApp(store)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Pico Xiangqi Arena listening on %s", displayListenAddr(cfg.ListenAddr))
	log.Printf("Snapshot path: %s", cfg.SnapshotPath)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func startInBackground(args []string) (int, error) {
	if err := os.MkdirAll(filepath.Dir(backgroundLogPath), 0o755); err != nil {
		return 0, err
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	defer devNull.Close()

	logFile, err := os.OpenFile(backgroundLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()

	cmd := exec.Command(os.Args[0], stripBackgroundArgs(args)...)
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	applyBackgroundProcessAttrs(cmd)

	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	if err := cmd.Process.Release(); err != nil {
		return 0, err
	}
	return pid, nil
}
