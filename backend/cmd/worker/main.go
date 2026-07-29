package main

import (
	"log/slog"
	"os"
)

// Worker binary — Phase 6+ (gmail poller) and Phase 7 (follow-ups) live here.
// Kept minimal for Phase 0 so `go build ./...` succeeds.
func main() {
	slog.New(slog.NewTextHandler(os.Stdout, nil)).Info("worker skeleton — no jobs registered yet")
}
