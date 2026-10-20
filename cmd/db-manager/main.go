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
	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

// pendingEngine keeps the infrastructure buildable before driver integration.
type pendingEngine struct{}

func (pendingEngine) Test(context.Context, core.Connection) error {
	return &core.Error{Code: "unsupported_driver", Message: "Datenbanktreiber ist noch nicht verfügbar."}
}
func (pendingEngine) Query(context.Context, core.QueryRequest) (core.QueryResult, error) {
	return core.QueryResult{}, &core.Error{Code: "unsupported_driver", Message: "Datenbanktreiber ist noch nicht verfügbar."}
}
func (pendingEngine) Catalog(context.Context, core.Connection) ([]core.Table, error) {
	return nil, &core.Error{Code: "unsupported_driver", Message: "Datenbanktreiber ist noch nicht verfügbar."}
}
func (pendingEngine) Drivers() []string { return []string{} }

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
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	server := &http.Server{Addr: cfg.Address, Handler: httpapi.New(pendingEngine{}, cfg.Token), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 70 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
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
