// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/config"
	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
	"github.com/MagicalWig34653/db-manager/internal/serverstate"
	"github.com/MagicalWig34653/db-manager/internal/webui"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		address := os.Getenv("DBM_ADDRESS")
		if address == "" {
			address = "127.0.0.1:8080"
		}
		if address == ":8080" || address == "0.0.0.0:8080" {
			address = "127.0.0.1:8080"
		}
		client := http.Client{Timeout: 2 * time.Second}
		response, err := client.Get("http://" + address + "/healthz")
		if err != nil {
			os.Exit(1)
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "state-migrate" {
		os.Exit(stateMigrate(os.Args[2:]))
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	engine, err := database.New(cfg.DataDir, cfg.AllowedHosts, database.WithCAFile(cfg.CAFile))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var apiOptions []httpapi.Option
	stateConfig, err := serverstate.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if stateConfig.Enabled() {
		store, err := startState(stateConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer store.Close()
		apiOptions = append(apiOptions, httpapi.WithStateStore(store))
	}
	server := &http.Server{Addr: cfg.Address, Handler: webui.Wrap(httpapi.New(engine, cfg.Token, apiOptions...)), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 70 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case err = <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "HTTP-Server konnte nicht gestartet werden.")
			os.Exit(1)
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if server.Shutdown(shutdown) != nil {
			_ = server.Close()
		}
	}
}

// startState connects the optional server state store. If a migration account is
// configured the schema is migrated first, under the cross-replica lock. A
// misconfigured or unreachable store prevents start (fail closed); afterwards a
// monitor tracks reachability and the API refuses requests while it is down.
func startState(cfg serverstate.Config) (*serverstate.Store, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if cfg.MigrationUser != "" {
		if err := reportMigration(ctx, cfg); err != nil {
			return nil, err
		}
	}
	store, err := serverstate.Open(cfg)
	if err != nil {
		return nil, err
	}
	if err := store.WaitReady(ctx, 30*time.Second); err != nil {
		_ = store.Close()
		return nil, err
	}
	store.Monitor(0)
	fmt.Fprintln(os.Stderr, "Zustandsspeicher: verbunden, Schema passt.")
	return store, nil
}

func reportMigration(ctx context.Context, cfg serverstate.Config) error {
	deadline, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var report serverstate.Report
	var err error
	for {
		report, err = serverstate.Migrate(deadline, cfg, serverstate.MigrateOptions{})
		if err != serverstate.ErrUnavailable {
			break
		}
		select {
		case <-deadline.Done():
			return serverstate.ErrUnavailable
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		return err
	}
	for _, version := range report.Applied {
		fmt.Fprintf(os.Stderr, "Zustandsspeicher: Migration %d angewendet.\n", version)
	}
	if len(report.Applied) == 0 {
		fmt.Fprintf(os.Stderr, "Zustandsspeicher: Schema aktuell (Version %d), keine Migration nötig.\n", report.Current)
	}
	return nil
}

// stateMigrate implements `db-manager state-migrate [--plan]` as an explicit
// one-shot step, for operators who do not want DDL credentials in the server.
func stateMigrate(args []string) int {
	plan := len(args) == 1 && args[0] == "--plan"
	if len(args) > 1 || (len(args) == 1 && !plan) {
		fmt.Fprintln(os.Stderr, "Aufruf: db-manager state-migrate [--plan]")
		return 2
	}
	cfg, err := serverstate.LoadConfig()
	if err == nil && !cfg.Enabled() {
		err = fmt.Errorf("Zustandsspeicher ist nicht konfiguriert (%s fehlt).", serverstate.EnvHost)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if plan {
		report, err := serverstate.Plan(ctx, cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "Aktuelle Version %d, ausstehende Migrationen: %v\n", report.Current, report.Applied)
		return 0
	}
	if err := reportMigration(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
