// finder-probe: hit the CascadeFinder from the CLI so you can debug the
// pipeline without going through auth + the extension.
//
// Usage:
//
//	go run ./cmd/finder-probe -name "Jane Smith" -company "Stripe" -url "https://linkedin.com/in/janesmith"
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/shubham/oneapply/backend/internal/config"
	"github.com/shubham/oneapply/backend/internal/finder"
)

func main() {
	_ = godotenv.Load("../.env", ".env")

	name := flag.String("name", "", "recruiter full name")
	company := flag.String("company", "", "company name")
	url := flag.String("url", "", "linkedin url (optional)")
	verbose := flag.Bool("v", false, "verbose logging")
	flag.Parse()

	if *name == "" || *company == "" {
		fmt.Fprintln(os.Stderr, "usage: finder-probe -name '...' -company '...' [-url '...']")
		os.Exit(2)
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	cfg, err := config.Load()
	if err != nil {
		// Config is only used for Apollo key; keep going without it.
		slog.Warn("config load failed, proceeding without Apollo", "err", err)
		cfg = &config.Config{}
	}

	// No cache adapter — probe always runs the full cascade.
	cascade := finder.NewCascadeFinder(
		nil,
		finder.NewHeuristicDomainResolver(),
		finder.NewDefaultPatternGenerator(),
		finder.NewSMTPVerifier("oneapply.local", "postmaster@oneapply.local"),
		finder.NewApolloFinder(cfg.ApolloAPIKey),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	res, err := cascade.FindEmail(ctx, finder.FindEmailRequest{
		Name:        *name,
		Company:     *company,
		LinkedInURL: *url,
	})
	elapsed := time.Since(start)

	fmt.Fprintf(os.Stderr, "elapsed: %s\n", elapsed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if res == nil {
		fmt.Fprintln(os.Stderr, "result: NOT FOUND")
		os.Exit(1)
	}

	out, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(out))
}
